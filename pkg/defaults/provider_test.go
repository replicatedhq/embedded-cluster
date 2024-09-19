package defaults

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "embedded-cluster")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)
	def := NewProvider(tmpdir)
	assert.DirExists(t, def.EmbeddedClusterLogsSubDir(), "logs dir should exist")
	assert.DirExists(t, def.EmbeddedClusterBinsSubDir(), "embedded-cluster binary dir should exist")
}

func TestEnsureAllDirectoriesAreInsideBase(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "embedded-cluster")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpdir)
	def := NewProvider(tmpdir)
	for _, fn := range []func() string{
		def.EmbeddedClusterBinsSubDir,
		def.EmbeddedClusterLogsSubDir,
	} {
		assert.Contains(t, fn(), tmpdir, "directory should be inside base")
		count := strings.Count(fn(), tmpdir)
		assert.Equal(t, 1, count, "base directory should not repeat")
	}
}
