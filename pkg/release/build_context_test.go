package release

import (
	"context"
	"encoding/base64"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_newBuildContext(t *testing.T) {
	ctx := context.Background()
	req := &BuildRequest{
		BinaryName:         "testBinary",
		KOTSRelease:        base64.StdEncoding.EncodeToString([]byte("testRelease")),
		KOTSReleaseVersion: "1.0.0",
	}
	bctx, err := newBuildContext(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, bctx)
	require.Equal(t, req, bctx.request)
	require.NotEmpty(t, bctx.baseDir)
	objpath := bctx.kotsReleaseObjectPath()
	require.Contains(t, objpath, bctx.baseDir)
	relpath := bctx.kotsReleasePath()
	require.Contains(t, relpath, bctx.baseDir)
	binpath := bctx.binaryPath()
	require.Contains(t, binpath, bctx.baseDir)
	tgzpath := bctx.tgzPath()
	require.Contains(t, tgzpath, bctx.baseDir)
	err = bctx.Close()
	require.NoError(t, err)
	require.NoDirExists(t, bctx.baseDir)
}

func Test_withSourceBinary(t *testing.T) {
	ctx := context.Background()
	req := &BuildRequest{
		BinaryName:         "testBinary",
		KOTSRelease:        base64.StdEncoding.EncodeToString([]byte("testRelease")),
		KOTSReleaseVersion: "1.0.0",
	}
	bctx, err := newBuildContext(ctx, req)
	require.NoError(t, err)
	binary := strings.NewReader("binaryContent")
	err = bctx.withSourceBinary(binary)
	require.NoError(t, err)
	data, err := os.ReadFile(bctx.binaryPath())
	require.NoError(t, err)
	require.Equal(t, "binaryContent", string(data))
	err = bctx.Close()
	require.NoError(t, err)
	require.NoDirExists(t, bctx.baseDir)
}

func TestRead(t *testing.T) {
	ctx := context.Background()
	req := &BuildRequest{
		BinaryName:         "testBinary",
		KOTSRelease:        base64.StdEncoding.EncodeToString([]byte("testRelease")),
		KOTSReleaseVersion: "1.0.0",
	}
	bctx, err := newBuildContext(ctx, req)
	require.NoError(t, err)
	err = os.WriteFile(bctx.tgzPath(), []byte("tgzContent"), 0644)
	require.NoError(t, err)
	res, err := io.ReadAll(bctx)
	require.NoError(t, err)
	require.Equal(t, "tgzContent", string(res))
	err = bctx.Close()
	require.NoError(t, err)
	require.NoDirExists(t, bctx.baseDir)
	_, err = io.ReadAll(bctx)
	require.Contains(t, err.Error(), "file already closed")
}
