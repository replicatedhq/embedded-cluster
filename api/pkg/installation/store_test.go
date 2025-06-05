package installation

import (
	"sync"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore()

	assert.NotNil(t, store)
	assert.NotNil(t, store.config)
	assert.NotNil(t, store.status)
}

func TestMemoryStore_ReadConfig(t *testing.T) {
	store := NewMemoryStore()
	store.config = &types.InstallationConfig{
		AdminConsolePort: 8080,
		DataDirectory:    "/some/dir",
	}

	config, err := store.ReadConfig()

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, &types.InstallationConfig{
		AdminConsolePort: 8080,
		DataDirectory:    "/some/dir",
	}, config)
}

func TestMemoryStore_WriteConfig(t *testing.T) {
	store := NewMemoryStore()
	expectedConfig := types.InstallationConfig{
		AdminConsolePort: 8080,
		DataDirectory:    "/some/dir",
	}

	err := store.WriteConfig(expectedConfig)

	require.NoError(t, err)

	// Verify the config was stored
	actualConfig, err := store.ReadConfig()
	require.NoError(t, err)
	assert.Equal(t, &expectedConfig, actualConfig)
}

func TestMemoryStore_ReadStatus(t *testing.T) {
	store := NewMemoryStore()
	store.status = &types.InstallationStatus{
		State:       types.InstallationStateFailed,
		Description: "Failure",
	}

	status, err := store.ReadStatus()

	require.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, &types.InstallationStatus{
		State:       types.InstallationStateFailed,
		Description: "Failure",
	}, status)
}

func TestMemoryStore_WriteStatus(t *testing.T) {
	store := NewMemoryStore()
	expectedStatus := types.InstallationStatus{
		State:       types.InstallationStateFailed,
		Description: "Failure",
	}

	err := store.WriteStatus(expectedStatus)

	require.NoError(t, err)

	// Verify the status was stored
	actualStatus, err := store.ReadStatus()
	require.NoError(t, err)
	assert.Equal(t, &expectedStatus, actualStatus)
}

// Useful to test concurrent access with -race flag
func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := NewMemoryStore()
	var wg sync.WaitGroup

	// Test concurrent reads and writes
	numGoroutines := 10
	numOperations := 100

	// Concurrent config operations
	wg.Add(numGoroutines * 2)
	for i := 0; i < numGoroutines; i++ {
		// Concurrent writes
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				config := types.InstallationConfig{
					LocalArtifactMirrorPort: 8080,
					DataDirectory:           "/some/other/dir",
				}
				err := store.WriteConfig(config)
				assert.NoError(t, err)
			}
		}(i)

		// Concurrent reads
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, err := store.ReadConfig()
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
				status := types.InstallationStatus{
					State:       types.InstallationStatePending,
					Description: "Pending",
				}
				err := store.WriteStatus(status)
				assert.NoError(t, err)
			}
		}(i)

		// Concurrent reads
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, err := store.ReadStatus()
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()
}
