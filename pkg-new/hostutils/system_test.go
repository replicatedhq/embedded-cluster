package hostutils

import (
	_ "embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSysctlConfig(t *testing.T) {
	basedir, err := os.MkdirTemp("", "embedded-cluster-test-base-dir")
	assert.NoError(t, err)
	defer os.RemoveAll(basedir)

	orig := sysctlConfigPath
	defer func() {
		sysctlConfigPath = orig
	}()

	rc := runtimeconfig.New(nil)
	rc.SetDataDir(basedir)

	// happy path.
	dstdir, err := os.MkdirTemp("", "embedded-cluster-test")
	assert.NoError(t, err)
	defer os.RemoveAll(dstdir)

	sysctlConfigPath = filepath.Join(dstdir, "sysctl.conf")
	err = sysctlConfig()
	assert.NoError(t, err)

	// check that the file exists.
	_, err = os.Stat(sysctlConfigPath)
	assert.NoError(t, err)

	// now use a non-existing directory.
	sysctlConfigPath = filepath.Join(dstdir, "non-existing-dir", "sysctl.conf")
	// we do not expect an error here.
	assert.NoError(t, err)
}

func TestDynamicSysctlConfig(t *testing.T) {
	// Create a temporary config file.
	configPath := filepath.Join(t.TempDir(), "99-dynamic-embedded-cluster.conf")

	tests := []struct {
		name            string
		mockValues      map[string]int64
		expectedLines   []string
		unexpectedLines []string
	}{
		{
			name: "inotify max_user values below minimum thresholds are updated",
			mockValues: map[string]int64{
				"fs.inotify.max_user_instances": 128,  // Below min
				"fs.inotify.max_user_watches":   8192, // Below min
			},
			expectedLines: []string{
				"fs.inotify.max_user_instances = 1024",
				"fs.inotify.max_user_watches = 65536",
			},
		},
		{
			name: "only below minimum values of inotify max_user are updated",
			mockValues: map[string]int64{
				"fs.inotify.max_user_instances": 128,     // Below min
				"fs.inotify.max_user_watches":   1048576, // Above min
			},
			expectedLines: []string{
				"fs.inotify.max_user_instances = 1024",
			},
			unexpectedLines: []string{
				"fs.inotify.max_user_watches",
			},
		},
		{
			name: "inotify max_user values above minimum thresholds are not updated",
			mockValues: map[string]int64{
				"fs.inotify.max_user_instances": 2048,    // Above min
				"fs.inotify.max_user_watches":   1048576, // Above min
			},
			expectedLines: []string{}, // No updates needed
			unexpectedLines: []string{
				"fs.inotify.max_user_instances",
				"fs.inotify.max_user_watches",
			},
		},
		{
			name: "inotify max_user values equal to minimum thresholds are not updated",
			mockValues: map[string]int64{
				"fs.inotify.max_user_instances": 1024,  // Equal to min
				"fs.inotify.max_user_watches":   65536, // Equal to min
			},
			expectedLines: []string{}, // No updates needed
			unexpectedLines: []string{
				"fs.inotify.max_user_instances",
				"fs.inotify.max_user_watches",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock getter
			mockGetter := func(key string) (int64, error) {
				value, exists := tt.mockValues[key]
				if !exists {
					t.Fatalf("unexpected key requested: %s", key)
				}
				return value, nil
			}

			err := generateDynamicSysctlConfig(mockGetter, configPath)
			require.NoError(t, err)

			// Read generated file
			content, err := os.ReadFile(configPath)
			require.NoError(t, err)

			// Check for expected lines
			for _, expectedLine := range tt.expectedLines {
				assert.Contains(t, string(content), expectedLine)
			}

			// Check for unexpected lines
			for _, unexpectedLine := range tt.unexpectedLines {
				assert.NotContains(t, string(content), unexpectedLine)
			}
		})
	}
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
