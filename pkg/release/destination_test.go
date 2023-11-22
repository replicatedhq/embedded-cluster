package release

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockSizeReadCloser struct {
	*strings.Reader
}

func (m *MockSizeReadCloser) Size() int64 {
	return int64(m.Reader.Len())
}

func (m *MockSizeReadCloser) Close() error {
	return nil
}

func Test_createPresignedObject(t *testing.T) {
	bucket := "bucket"
	key := "a-release.tgz"
	// test success
	mockPresignClient := &MockS3PresignClient{}
	destination := &Destination{presignClient: mockPresignClient, bucket: bucket}
	mockPresignClient.On(
		"PresignGetObject", mock.Anything, mock.Anything, mock.Anything,
	).Return(
		&v4.PresignedHTTPRequest{URL: "http://example.com/a-presigned-url"},
		nil,
	)
	url, err := destination.createPresignedURL(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, "http://example.com/a-presigned-url", url)
	// test error
	mockPresignClient = &MockS3PresignClient{}
	destination = &Destination{presignClient: mockPresignClient, bucket: bucket}
	mockPresignClient.On(
		"PresignGetObject",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(
		(*v4.PresignedHTTPRequest)(nil),
		fmt.Errorf("presign error"),
	)
	_, err = destination.createPresignedURL(context.Background(), key)
	require.Error(t, err)
	require.Contains(t, err.Error(), "presign error")
}

func TestExists(t *testing.T) {
	ctx := context.Background()
	req := &BuildRequest{
		BinaryName:             "embedded-cluster",
		EmbeddedClusterVersion: "v1.0.0",
		LicenseID:              "license-id",
		KOTSRelease:            "abc",
		KOTSReleaseVersion:     "v1.0.0",
	}
	bucket := "test-bucket"

	// s3 object exists and presigned url is generated
	mockClient := &MockS3Client{}
	mockPresignClient := &MockS3PresignClient{}
	destination := &Destination{
		client:        mockClient,
		presignClient: mockPresignClient,
		bucket:        bucket,
	}
	mockClient.On(
		"HeadObject",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(
		&s3.HeadObjectOutput{},
		nil,
	)
	mockPresignClient.On(
		"PresignGetObject",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(
		&v4.PresignedHTTPRequest{
			URL: "http://example.com/presigned-url",
		},
		nil,
	)
	url, err := destination.Exists(ctx, req)
	require.NoError(t, err)
	require.Equal(t, "http://example.com/presigned-url", url)

	// s3 object does not exist
	mockClient = &MockS3Client{}
	mockPresignClient = &MockS3PresignClient{}
	destination = &Destination{
		client:        mockClient,
		presignClient: mockPresignClient,
		bucket:        bucket,
	}
	mockClient.On(
		"HeadObject",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(
		(*s3.HeadObjectOutput)(nil),
		&types.NoSuchKey{},
	)
	_, err = destination.Exists(ctx, req)
	require.NoError(t, err)

	// unexpected error
	mockClient = &MockS3Client{}
	mockPresignClient = &MockS3PresignClient{}
	destination = &Destination{
		client:        mockClient,
		presignClient: mockPresignClient,
		bucket:        bucket,
	}
	mockError := errors.New("unexpected error")
	mockClient.On(
		"HeadObject",
		mock.Anything,
		mock.Anything,
	).Return(
		(*s3.HeadObjectOutput)(nil),
		mockError,
	)
	_, err = destination.Exists(ctx, req)
	require.Error(t, err)
}

func TestUpload(t *testing.T) {
	ctx := context.Background()
	req := &BuildRequest{
		BinaryName:             "embedded-cluster",
		EmbeddedClusterVersion: "v1.0.0",
		LicenseID:              "license-id",
		KOTSRelease:            "abc",
		KOTSReleaseVersion:     "v1.0.0",
	}
	bucket := "test-bucket"
	mockSrc := &MockSizeReadCloser{strings.NewReader("test")}

	// successful upload and presigned URL generation
	mockClient := &MockS3Client{}
	mockPresignClient := &MockS3PresignClient{}
	destination := &Destination{
		client:        mockClient,
		presignClient: mockPresignClient,
		bucket:        bucket,
	}
	mockClient.On(
		"PutObject", mock.Anything, mock.Anything,
	).Return(
		&s3.PutObjectOutput{}, nil,
	)
	mockPresignClient.On(
		"PresignGetObject",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(
		&v4.PresignedHTTPRequest{URL: "http://example.com/presigned-url"},
		nil,
	)
	url, err := destination.Upload(ctx, req, mockSrc)
	require.NoError(t, err)
	require.Equal(t, "http://example.com/presigned-url", url)

	// error during upload
	mockClient = &MockS3Client{}
	mockPresignClient = &MockS3PresignClient{}
	destination = &Destination{
		client:        mockClient,
		presignClient: mockPresignClient,
		bucket:        bucket,
	}
	mockClient.On(
		"PutObject", mock.Anything, mock.Anything,
	).Return(
		(*s3.PutObjectOutput)(nil),
		fmt.Errorf("upload error"),
	)
	_, err = destination.Upload(ctx, req, mockSrc)
	require.Error(t, err)
	require.Contains(t, err.Error(), "upload error")
}
