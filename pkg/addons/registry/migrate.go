package registry

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pkg/errors"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

var (
	s3Bucket        = "registry"
	s3RootDirectory = "registry"
	labelSelector   = "app=docker-registry"
)

// Migrate runs a migration that copies data on disk in the registry PVC to the seaweedfs s3 store.
func (r *Registry) Migrate(ctx context.Context, kcli client.Client, progressWriter *spinner.MessageWriter) error {
	s3Client, err := getS3Client(ctx, kcli, r.ServiceCIDR)
	if err != nil {
		return errors.Wrap(err, "get s3 client")
	}

	logrus.Debug("Ensuring registry bucket")
	if err := ensureRegistryBucket(ctx, s3Client); err != nil {
		return errors.Wrap(err, "ensure registry bucket")
	}
	logrus.Debug("Registry bucket ensured")

	pipeReader, pipeWriter := io.Pipe()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		defer pipeWriter.Close()
		return readRegistryData(ctx, pipeWriter)
	})

	g.Go(func() error {
		return writeRegistryData(ctx, pipeReader, s3manager.NewUploader(s3Client), progressWriter)
	})

	logrus.Debug("Copying registry data")
	if err := g.Wait(); err != nil {
		return err
	}
	logrus.Debug("Registry data copied")

	return nil
}

func getS3Client(ctx context.Context, kcli client.Client, serviceCIDR string) (*s3.Client, error) {
	accessKey, secretKey, err := seaweedfs.GetS3RWCreds(ctx, kcli)
	if err != nil {
		return nil, errors.Wrap(err, "get seaweedfs s3 rw creds")
	}

	creds := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
	conf, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithCredentialsProvider(creds),
		awsconfig.WithRegion("us-east-1"),
	)
	if err != nil {
		return nil, errors.Wrap(err, "load aws config")
	}

	s3URL, err := seaweedfs.GetS3URL(serviceCIDR)
	if err != nil {
		return nil, errors.Wrap(err, "get seaweedfs s3 endpoint")
	}

	s3Client := s3.NewFromConfig(conf, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = aws.String(s3URL)
	})

	return s3Client, nil
}

func ensureRegistryBucket(ctx context.Context, s3Client *s3.Client) error {
	_, err := s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: &s3Bucket,
	})
	if err != nil {
		if !strings.Contains(err.Error(), "BucketAlreadyExists") {
			return errors.Wrap(err, "create bucket")
		}
	}
	return nil
}

func readRegistryData(ctx context.Context, writer io.Writer) error {
	return execInPod(ctx, []string{"tar", "-c", "-C", "/var/lib/registry", "."}, writer)
}

func writeRegistryData(ctx context.Context, reader io.Reader, s3Uploader *s3manager.Uploader, progressWriter *spinner.MessageWriter) error {
	total, err := countRegistryFiles(ctx)
	if err != nil {
		return errors.Wrap(err, "count registry files")
	}

	progress := 0
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

		_, err = s3Uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket: &s3Bucket,
			Key:    aws.String(filepath.Join(s3RootDirectory, relPath)),
			Body:   tr,
		})
		if err != nil {
			return errors.Wrap(err, "upload to s3")
		}

		progress++
		progressWriter.Infof("Migrating data for high availability (%d%%)", (progress*100)/total)
	}

	return nil
}

func countRegistryFiles(ctx context.Context) (int, error) {
	var stdout bytes.Buffer
	if err := execInPod(ctx, []string{"sh", "-c", "find /var/lib/registry -type f | wc -l"}, &stdout); err != nil {
		return 0, errors.Wrap(err, "exec in pod")
	}
	return strconv.Atoi(strings.TrimSpace(stdout.String()))
}

func execInPod(ctx context.Context, command []string, stdout io.Writer) error {
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

	pods, err := clientSet.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
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
		Namespace(namespace).
		SubResource("exec")

	parameterCodec := runtime.NewParameterCodec(scheme)
	req.VersionedParams(&corev1.PodExecOptions{
		Command:   command,
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
		Stdout: stdout,
		Stderr: &stderr,
	}); err != nil {
		return fmt.Errorf("stream exec: %v: %s", err, stderr.String())
	}

	return nil
}
