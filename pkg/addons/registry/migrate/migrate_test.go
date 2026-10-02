package migrate

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeUploader reads the first n bytes of the body (all of it if n < 0) and records them.
type fakeUploader struct {
	n      int
	bodies [][]byte
}

func (u *fakeUploader) UploadObject(ctx context.Context, input *transfermanager.UploadObjectInput, opts ...func(*transfermanager.Options)) (*transfermanager.UploadObjectOutput, error) {
	r := input.Body
	if u.n >= 0 {
		r = io.LimitReader(r, int64(u.n))
	}
	b, err := io.ReadAll(r)
	u.bodies = append(u.bodies, b)
	return &transfermanager.UploadObjectOutput{}, err
}

func Test_uploadObject(t *testing.T) {
	content := []byte("0123456789abcdefghijklmnopqrstuvwxyz")

	tests := []struct {
		name          string
		firstAttemptN int
	}{
		{name: "retry after the first attempt read the whole file", firstAttemptN: -1},
		{name: "retry after the first attempt read part of the file", firstAttemptN: 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "data")
			require.NoError(t, os.WriteFile(path, content, 0644))
			f, err := os.Open(path)
			require.NoError(t, err)
			defer f.Close()

			// first attempt consumes the body like a failed multipart upload would
			require.NoError(t, uploadObject(context.Background(), &fakeUploader{n: tt.firstAttemptN}, f, "key"))

			retry := &fakeUploader{n: -1}
			require.NoError(t, uploadObject(context.Background(), retry, f, "key"))
			assert.Equal(t, [][]byte{content}, retry.bodies)
		})
	}
}
