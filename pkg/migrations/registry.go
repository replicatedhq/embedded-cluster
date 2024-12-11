package migrations

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"
	k8sconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	registryBucket        = "registry"
	registryNamespace     = "registry"
	registryLabelSelector = "app=docker-registry"
	seaweedFSS3URL        = "http://seaweedfs-s3.seaweedfs:8333/"
)

// RegistryData runs a migration that copies data on disk in the registry PVC to the seaweedfs s3 store.
func RegistryData(ctx context.Context) error {
	logrus.Debug("Migrating registry data")

	s3Client, err := getS3Client(ctx)
	if err != nil {
		return errors.Wrap(err, "get s3 client")
	}

	if err := ensureRegistryBucket(ctx, s3Client); err != nil {
		return errors.Wrap(err, "ensure registry bucket")
	}

	pipeReader, pipeWriter := io.Pipe()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		defer pipeWriter.Close()
		return readRegistryData(ctx, pipeWriter)
	})

	g.Go(func() error {
		return writeRegistryData(ctx, pipeReader, s3Client)
	})

	if err := g.Wait(); err != nil {
		return err
	}

	logrus.Debug("Registry data migrated successfully")
	return nil
}

func getS3Client(ctx context.Context) (*s3.Client, error) {
	// TODO NOW: pass creds as params
	creds := credentials.NewStaticCredentialsProvider(
		os.Getenv("s3AccessKey"),
		os.Getenv("s3SecretKey"),
		"",
	)

	conf, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithCredentialsProvider(creds),
		awsconfig.WithRegion("us-east-1"),
	)
	if err != nil {
		return nil, errors.Wrap(err, "load aws config")
	}

	s3Client := s3.NewFromConfig(conf, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(seaweedFSS3URL)
	})

	return s3Client, nil
}

func ensureRegistryBucket(ctx context.Context, s3Client *s3.Client) error {
	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: &registryBucket,
	})
	if err != nil {
		if !strings.Contains(err.Error(), "BucketAlreadyExists") {
			return errors.Wrap(err, "create bucket")
		}
	}
	return nil
}

func readRegistryData(ctx context.Context, writer io.Writer) error {
	cfg, err := k8sconfig.GetConfig()
	if err != nil {
		return errors.Wrap(err, "get kubernetes config")
	}

	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "create kubernetes clientset")
	}

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return errors.Wrap(err, "add corev1 scheme")
	}

	pods, err := clientSet.CoreV1().Pods(registryNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: registryLabelSelector,
	})
	if err != nil {
		return errors.Wrap(err, "list registry pods")
	}
	if len(pods.Items) == 0 {
		return errors.New("no registry pods found")
	}
	podName := pods.Items[0].Name

	req := clientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(registryNamespace).
		SubResource("exec")

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   []string{"tar", "-c", "-C", "/var/lib/registry", "."},
		Container: "docker-registry",
		Stdout:    true,
		Stderr:    true,
	}, parameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return errors.Wrap(err, "create exec")
	}

	var stderr bytes.Buffer
	if err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: writer,
		Stderr: &stderr,
	}); err != nil {
		return fmt.Errorf("stream exec: %v: %s", err, stderr.String())
	}

	return nil
}

func writeRegistryData(ctx context.Context, reader io.Reader, s3Client *s3.Client) error {
	tr := tar.NewReader(reader)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "read tar header")
		}

		if header.FileInfo().IsDir() {
			continue
		}

		relPath, err := filepath.Rel("./", header.Name)
		if err != nil {
			return errors.Wrap(err, "get relative path")
		}

		_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: &registryBucket,
			Key:    aws.String(relPath),
			Body:   tr,
		})
		if err != nil {
			return errors.Wrap(err, "upload to s3")
		}

		logrus.Debugf("Uploaded %s", relPath)
	}

	return nil
}
