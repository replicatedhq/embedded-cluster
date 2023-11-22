package release

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"testing"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockS3PresignClient struct {
	mock.Mock
}

func (m *MockS3PresignClient) PresignGetObject(ctx context.Context, input *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error) {
	args := m.Called(ctx, input, optFns)
	return args.Get(0).(*v4.PresignedHTTPRequest), args.Error(1)
}

type MockS3Client struct {
	mock.Mock
}

func (m *MockS3Client) GetObject(ctx context.Context, input *s3.GetObjectInput, params ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*s3.GetObjectOutput), args.Error(1)
}

func (m *MockS3Client) HeadObject(ctx context.Context, input *s3.HeadObjectInput, params ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*s3.HeadObjectOutput), args.Error(1)
}

func (m *MockS3Client) PutObject(ctx context.Context, input *s3.PutObjectInput, params ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(*s3.PutObjectOutput), args.Error(1)
}

func createReleaseTGZWithContent(content string) io.ReadCloser {
	buf := bytes.NewBuffer(nil)
	gzipWriter := gzip.NewWriter(buf)
	tarWriter := tar.NewWriter(gzipWriter)
	header := &tar.Header{
		Name: "embedded-cluster",
		Mode: 0755,
		Size: int64(len(content)),
	}
	tarWriter.WriteHeader(header)
	tarWriter.Write([]byte(content))
	tarWriter.Close()
	gzipWriter.Close()
	return io.NopCloser(buf)
}

func countOpenFileDescriptors() (int, error) {
	fds, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return 0, err
	}
	return len(fds), nil
}

func Test_binaryFor(t *testing.T) {
	openfd, err := countOpenFileDescriptors()
	require.NoError(t, err)
	dir, err := os.MkdirTemp("/tmp", "embedded-cluster-test-*")
	defer os.RemoveAll(dir)
	require.NoError(t, err)
	localReleases = dir
	s3mock := &MockS3Client{}
	s3mock.On("GetObject", mock.Anything, mock.Anything).Return((*s3.GetObjectOutput)(nil), fmt.Errorf("object does not exist"))
	src, err := newSource(context.Background())
	require.NoError(t, err)
	src.client = s3mock
	_, err = src.binaryFor(context.Background(), "development")
	require.Equal(t, err.Error(), "unable to get object: object does not exist")
	s3mock = &MockS3Client{}
	release := createReleaseTGZWithContent("binary content")
	s3mock.On("GetObject", mock.Anything, mock.Anything).Return(&s3.GetObjectOutput{Body: release}, nil)
	src.client = s3mock
	reader, err := src.binaryFor(context.Background(), "development")
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "binary content", string(data))
	err = reader.Close()
	require.NoError(t, err)
	// change the content of the cached release and attempt to get it
	// again. We should get the cached version.
	_, err = os.Stat(path.Join(dir, "development"))
	require.NoError(t, err)
	err = os.WriteFile(path.Join(dir, "development"), []byte("content has been changed"), 0755)
	require.NoError(t, err)
	reader, err = src.binaryFor(context.Background(), "development")
	require.NoError(t, err)
	data, err = io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "content has been changed", string(data))
	err = reader.Close()
	require.NoError(t, err)
	openfdAfter, err := countOpenFileDescriptors()
	require.NoError(t, err)
	require.Equal(t, openfd, openfdAfter)
}
