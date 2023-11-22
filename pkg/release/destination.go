package release

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Destination manages the destination for built embedded cluster releases.
type Destination struct {
	client        S3API
	presignClient S3PresignAPI
	bucket        string
}

// Exists checks if a release has already been built. Returns the URL for the release if
// it has already been built. This function returns an empty string if the release has not
// been built.
func (d *Destination) Exists(ctx context.Context, req *BuildRequest) (string, error) {
	key := fmt.Sprintf("%s.tgz", req.uniqName())
	if _, err := d.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &d.bucket,
		Key:    &key,
	}); err != nil {
		var notFound *types.NoSuchKey
		if errors.As(err, &notFound) {
			return "", nil
		} else if strings.Contains(err.Error(), "NotFound") {
			return "", nil
		}
		return "", err
	}
	return d.createPresignedURL(ctx, key)
}

// createPresignedURL creates a presigned URL for the given object key.
func (d *Destination) createPresignedURL(ctx context.Context, key string) (string, error) {
	psreq, err := d.presignClient.PresignGetObject(
		ctx,
		&s3.GetObjectInput{
			Bucket: &d.bucket,
			Key:    &key,
		},
		func(opts *s3.PresignOptions) {
			opts.Expires = time.Hour
		},
	)
	if err != nil {
		return "", fmt.Errorf("unable to presign get object: %w", err)
	}
	return psreq.URL, nil
}

// Upload uploads a release to the destination. Returns the URL for the release.
func (d *Destination) Upload(ctx context.Context, req *BuildRequest, src SizeReadCloser) (string, error) {
	key := fmt.Sprintf("%s.tgz", req.uniqName())
	size := src.Size()
	if _, err := d.client.PutObject(
		ctx,
		&s3.PutObjectInput{
			Bucket:        &d.bucket,
			Key:           &key,
			Body:          src,
			ContentLength: &size,
		},
	); err != nil {
		return "", fmt.Errorf("unable to upload release: %w", err)
	}
	return d.createPresignedURL(ctx, key)
}

// NewDestination returns a new release destination.
func NewDestination(ctx context.Context) (*Destination, error) {
	bucket := os.Getenv("EMBEDDED_CLUSTER_RELEASE_BUCKET_NAME")
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to load aws default config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true })
	return &Destination{
		client:        client,
		presignClient: s3.NewPresignClient(client),
		bucket:        bucket,
	}, nil
}
