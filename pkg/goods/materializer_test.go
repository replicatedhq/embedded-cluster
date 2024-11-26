package goods

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaterializer_SysctlConfig(t *testing.T) {
	m := NewMaterializer()

	// happy path.
	dstdir, err := os.MkdirTemp("", "embedded-cluster-test")
	assert.NoError(t, err)
	defer os.RemoveAll(dstdir)

	dstpath := filepath.Join(dstdir, "sysctl.conf")
	err = m.SysctlConfig(dstpath)
	assert.NoError(t, err)

	expected, err := os.ReadFile(dstpath)
	assert.NoError(t, err)

	content, err := staticfs.ReadFile("static/99-embedded-cluster.conf")
	assert.NoError(t, err)
	assert.Equal(t, string(expected), string(content))

	// write to a non-existent directory.
	dstpath = filepath.Join(dstdir, "dir-does-not-exist", "sysctl.conf")
	err = m.SysctlConfig(dstpath)
	assert.Contains(t, err.Error(), "no such file or directory")
}
