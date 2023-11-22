package release

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"debug/elf"
	"encoding/base64"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeSRC struct{}

func (f fakeSRC) binaryFor(ctx context.Context, version string) (io.ReadCloser, error) {
	path, err := exec.LookPath("ls")
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func TestBuild(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping build test on non-linux platform")
	}
	binName := uuid.NewString()
	request := &BuildRequest{
		BinaryName:             binName,
		EmbeddedClusterVersion: "v1.0.0",
		KOTSRelease:            base64.StdEncoding.EncodeToString([]byte("release content")),
		KOTSReleaseVersion:     "v2.0.0",
	}
	builder, err := NewBuilder(context.Background())
	require.NoError(t, err)
	builder.src = fakeSRC{}
	res, err := builder.Build(context.Background(), request)
	require.NoError(t, err)
	defer res.Close()
	gzipReader, err := gzip.NewReader(res)
	require.NoError(t, err)
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		path := filepath.Join("/tmp", header.Name)
		switch header.Typeflag {
		case tar.TypeReg:
			outFile, err := os.Create(path)
			require.NoError(t, err)
			_, err = io.Copy(outFile, tarReader)
			require.NoError(t, err)
			outFile.Close()
		}
	}
	binPath := path.Join("/tmp", binName)
	elfp, err := elf.Open(binPath)
	require.NoError(t, err)
	defer elfp.Close()
	section := elfp.Section("sec_bundle")
	require.NotNil(t, section)
	data, err := section.Data()
	require.NoError(t, err)
	require.Equal(t, "release content", string(data))
	os.RemoveAll(binPath)
}
