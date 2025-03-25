package migrate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	s3Bucket        = "registry"
	s3RootDirectory = "registry"
	labelSelector   = "app=docker-registry"
)

// RegistryData runs a migration that copies data on disk in the registry-data PVC to the seaweedfs
// s3 store. If it fails, it will scale the registry deployment back to 1. If it succeeds, it will
// create a secret used to indicate success to the operator.
func RegistryData(ctx context.Context, cli client.Client) error {
	// if the migration fails, we need to scale the registry back to 1
	success := false

	slog.Info("Scaling registry to 0 replicas")

	err := registryScale(ctx, cli, 0)
	if err != nil {
		return fmt.Errorf("scale registry to 0 replicas: %w", err)
	}

	defer func() {
		r := recover()

		if !success {
			slog.Info("Scaling registry back to 1 replica after migration failure")

			// this should use the background context as we want it to run even if the context expired
			err := registryScale(context.Background(), cli, 1)
			if err != nil {
				slog.Error("Failed to scale registry back to 1 replica", "error", err)
			}
		}

		if r != nil {
			panic(r)
		}
	}()

	slog.Info("Connecting to s3")

	s3Client, err := getS3Client(ctx)
	if err != nil {
		return errors.Wrap(err, "get s3 client")
	}

	slog.Info("Ensuring registry bucket")

	err = ensureRegistryBucket(ctx, s3Client)
	if err != nil {
		return errors.Wrap(err, "ensure registry bucket")
	}

	slog.Info("Running registry data migration")

	s3Uploader := s3manager.NewUploader(s3Client)

	total, err := countRegistryFiles()
	if err != nil {
		return errors.Wrap(err, "count registry files")
	}

	count := 0
	err = filepath.Walk("/registry", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("walk: %w", err)
		}

		if info.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}
		defer f.Close()

		relPath, err := filepath.Rel("/", path)
		if err != nil {
			return fmt.Errorf("get relative path: %w", err)
		}

		// retry uploading the object 5 times with exponential backoff
		var lasterr error
		err = wait.ExponentialBackoffWithContext(ctx, wait.Backoff{
			Duration: 1 * time.Second,
			Factor:   2,
			Steps:    5,
		}, func(ctx context.Context) (bool, error) {
			slog.Info("Uploading object", "path", relPath, "size", info.Size())

			_, err = s3Uploader.Upload(ctx, &s3.PutObjectInput{
				Bucket: ptr.To(s3Bucket),
				Key:    &relPath,
				Body:   f,
			})
			if err != nil {
				slog.Error("Failed to upload object", "path", relPath, "error", err)
			} else {
				slog.Info("Uploaded object", "path", relPath)
			}
			lasterr = err
			return err == nil, nil
		})
		if err != nil {
			if lasterr == nil {
				lasterr = err
			}
			return fmt.Errorf("upload object: %w", lasterr)
		}

		count++
		// NOTE: this is used by the cli to report progress
		// DO NOT CHANGE THIS
		slog.Info(
			"Uploaded object",
			append(
				[]any{"path", relPath, "size", info.Size()},
				getProgressArgs(count, total)...,
			)...,
		)

		return nil
	})
	if err != nil {
		return fmt.Errorf("walk registry data: %w", err)
	}

	success = true

	slog.Info("Registry data migration complete")

	return nil
}

// registryScale scales the registry deployment to the given replica count.
// '0' and '1' are the only acceptable values.
func registryScale(ctx context.Context, cli client.Client, scale int32) error {
	if scale != 0 && scale != 1 {
		return fmt.Errorf("invalid scale: %d", scale)
	}

	currentRegistry := &appsv1.Deployment{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: runtimeconfig.RegistryNamespace, Name: "registry"}, currentRegistry)
	if err != nil {
		return fmt.Errorf("get registry deployment: %w", err)
	}

	currentRegistry.Spec.Replicas = &scale

	err = cli.Update(ctx, currentRegistry)
	if err != nil {
		return fmt.Errorf("update registry deployment: %w", err)
	}

	return nil
}

func getS3Client(ctx context.Context) (*s3.Client, error) {
	creds := credentials.NewStaticCredentialsProvider(os.Getenv("s3AccessKey"), os.Getenv("s3SecretKey"), "")
	conf, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(creds),
		config.WithRegion("us-east-1"),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	s3Client := s3.NewFromConfig(conf, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String("http://seaweedfs-s3.seaweedfs:8333/")
	})

	return s3Client, nil
}

func ensureRegistryBucket(ctx context.Context, s3Client *s3.Client) error {
	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: ptr.To(s3Bucket),
	})
	if err != nil {
		var bne *s3types.BucketAlreadyExists
		if !errors.As(err, &bne) {
			return errors.Wrap(err, "create bucket")
		}
	}
	return nil
}

func countRegistryFiles() (int, error) {
	var count int
	err := filepath.Walk("/registry", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		count++
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("walk /registry directory: %w", err)
	}
	return count, nil
}
