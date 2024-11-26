package configutils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
)

func TestConfigureSysctl(t *testing.T) {
	basedir, err := os.MkdirTemp("", "embedded-cluster-test-base-dir")
	assert.NoError(t, err)
	defer os.RemoveAll(basedir)

	orig := sysctlConfigPath
	defer func() {
		sysctlConfigPath = orig
	}()

	runtimeconfig.SetDataDir(basedir)

	// happy path.
	dstdir, err := os.MkdirTemp("", "embedded-cluster-test")
	assert.NoError(t, err)
	defer os.RemoveAll(dstdir)

	sysctlConfigPath = filepath.Join(dstdir, "sysctl.conf")
	err = ConfigureSysctl()
	assert.NoError(t, err)

	// check that the file exists.
	_, err = os.Stat(sysctlConfigPath)
	assert.NoError(t, err)

	// now use a non-existing directory.
	sysctlConfigPath = filepath.Join(dstdir, "non-existing-dir", "sysctl.conf")
	// we do not expect an error here.
	assert.NoError(t, err)
}
