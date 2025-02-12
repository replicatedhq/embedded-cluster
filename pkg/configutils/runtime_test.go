package configutils

import (
	_ "embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
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

func Test_ensureKernelModulesLoaded(t *testing.T) {
	// Create and set mock helper
	mock := &helpers.MockHelpers{
		Commands: make([]string, 0),
	}

	helpers.Set(mock)
	t.Cleanup(func() {
		helpers.Set(&helpers.Helpers{})
	})

	// Run the function being tested
	err := ensureKernelModulesLoaded()
	if err != nil {
		t.Errorf("ensureKernelModulesLoaded() returned error: %v", err)
	}

	// Expected modprobe commands
	expectedCommands := []string{
		"modprobe overlay",
		"modprobe ip_tables",
		"modprobe br_netfilter",
		"modprobe nf_conntrack",
	}

	// Verify the correct commands were run
	if len(mock.Commands) != len(expectedCommands) {
		t.Errorf("Expected %d commands, got %d", len(expectedCommands), len(mock.Commands))
	}

	for i, cmd := range expectedCommands {
		if i >= len(mock.Commands) {
			t.Errorf("Missing expected command: %s", cmd)
			continue
		}
		if mock.Commands[i] != cmd {
			t.Errorf("Expected command %q, got %q", cmd, mock.Commands[i])
		}
	}
}
