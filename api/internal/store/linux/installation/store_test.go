package installation

import (
	"sync"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryStore(t *testing.T) {
	inst := types.LinuxInstallation{}
	store := NewMemoryStore(WithInstallation(inst))

	assert.NotNil(t, store)
	assert.Equal(t, inst, store.installation)
}

func TestMemoryStore_GetConfig(t *testing.T) {
	inst := types.LinuxInstallation{
		Config: types.LinuxInstallationConfig{
			AdminConsolePort: 8080,
			DataDirectory:    "/some/dir",
		},
	}
	store := NewMemoryStore(WithInstallation(inst))

	config, err := store.GetConfig()

	require.NoError(t, err)
	assert.Equal(t, types.LinuxInstallationConfig{
		AdminConsolePort: 8080,
		DataDirectory:    "/some/dir",
	}, config)
}

func TestMemoryStore_SetConfig(t *testing.T) {
	inst := types.LinuxInstallation{
		Config: types.LinuxInstallationConfig{
			AdminConsolePort: 1000,
			DataDirectory:    "/a/different/dir",
		},
	}
	store := NewMemoryStore(WithInstallation(inst))
	expectedConfig := types.LinuxInstallationConfig{
		AdminConsolePort: 8080,
		DataDirectory:    "/some/dir",
	}

	err := store.SetConfig(expectedConfig)

	require.NoError(t, err)

	// Verify the config was stored
	actualConfig, err := store.GetConfig()
	require.NoError(t, err)
	assert.Equal(t, expectedConfig, actualConfig)
}

func TestMemoryStore_GetStatus(t *testing.T) {
	inst := types.LinuxInstallation{
		Status: types.Status{
			State:       "failed",
			Description: "Failure",
		},
	}
	store := NewMemoryStore(WithInstallation(inst))

	status, err := store.GetStatus()

	require.NoError(t, err)
	assert.Equal(t, types.Status{
		State:       "failed",
		Description: "Failure",
	}, status)
}
func TestMemoryStore_SetStatus(t *testing.T) {
	inst := types.LinuxInstallation{
		Status: types.Status{
			State:       "failed",
			Description: "Failure",
		},
	}
	store := NewMemoryStore(WithInstallation(inst))
	expectedStatus := types.Status{
		State:       "running",
		Description: "Running",
	}

	err := store.SetStatus(expectedStatus)

	require.NoError(t, err)

	// Verify the status was stored
	actualStatus, err := store.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, expectedStatus, actualStatus)
}

// Useful to test concurrent access with -race flag
func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	inst := types.LinuxInstallation{}
	store := NewMemoryStore(WithInstallation(inst))
	var wg sync.WaitGroup

	// Test concurrent reads and writes
	numGoroutines := 10
	numOperations := 50

	// Concurrent config operations
	wg.Add(numGoroutines * 2)
	for i := 0; i < numGoroutines; i++ {
		// Concurrent writes
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				config := types.LinuxInstallationConfig{
					LocalArtifactMirrorPort: 8080,
					DataDirectory:           "/some/other/dir",
				}
				err := store.SetConfig(config)
				assert.NoError(t, err)
			}
		}(i)

		// Concurrent reads
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, err := store.GetConfig()
				assert.NoError(t, err)
			}
		}(i)
	}

	// Concurrent status operations
	wg.Add(numGoroutines * 2)
	for i := 0; i < numGoroutines; i++ {
		// Concurrent writes
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				status := types.Status{
					State:       "pending",
					Description: "Pending",
				}
				err := store.SetStatus(status)
				assert.NoError(t, err)
			}
		}(i)

		// Concurrent reads
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, err := store.GetStatus()
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()
}
