package release

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// localReleases is a location on disk where we keep already downloaded
// releases.
var localReleases = "/releases"

// S3PresignAPI is an interface for presigning S3 requests. This interface is useful for
// testing and is implemented by the s3.PresignClient.
type S3PresignAPI interface {
	PresignGetObject(context.Context, *s3.GetObjectInput, ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// S3API is an interface for the S3 client. It is used for test purposes.
type S3API interface {
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// Source represents the source of the release.
type Source struct {
	client S3API
	bucket string
}

// BinaryFor returns a ReadCloser to the binary for the given version.
func (s *Source) binaryFor(ctx context.Context, version string) (io.ReadCloser, error) {
	fpath := path.Join(localReleases, version)
	if fp, err := os.Open(fpath); err == nil {
		return fp, nil
	}
	key := fmt.Sprintf("%s.tgz", version)
	response, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return nil, fmt.Errorf("unable to get object: %w", err)
	}
	return newReleaseAsset(response, version)
}

// newSource returns a new release source.
func newSource(ctx context.Context) (*Source, error) {
	bucket := os.Getenv("EMBEDDED_CLUSTER_RELEASE_BUCKET_NAME")
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load aws default config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true })
	return &Source{client: client, bucket: bucket}, nil
}
