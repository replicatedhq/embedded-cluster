package installation

import (
	"sync"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryStore(t *testing.T) {
	rc := runtimeconfig.New(nil)
	store := NewMemoryStore(rc, types.NewStatus())

	assert.NotNil(t, store)
	assert.NotNil(t, store.rc)
	assert.Equal(t, rc, store.rc)
}

func TestMemoryStore_GetConfig(t *testing.T) {
	rc := runtimeconfig.New(&ecv1beta1.RuntimeConfigSpec{
		DataDir: "/some/dir",
		AdminConsole: ecv1beta1.AdminConsoleSpec{
			Port: 8080,
		},
	})
	store := NewMemoryStore(rc, types.NewStatus())

	config, err := store.GetConfig()

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, &types.InstallationConfig{
		AdminConsolePort:        8080,
		DataDirectory:           "/some/dir",
		LocalArtifactMirrorPort: 50000,           // default
		GlobalCIDR:              "10.244.0.0/16", // default
	}, config)
}

func TestMemoryStore_SetConfig(t *testing.T) {
	rc := runtimeconfig.New(&ecv1beta1.RuntimeConfigSpec{
		DataDir: "/a/different/dir",
		AdminConsole: ecv1beta1.AdminConsoleSpec{
			Port: 1000,
		},
	})

	store := NewMemoryStore(rc, types.NewStatus())
	expectedConfig := types.InstallationConfig{
		DataDirectory:           "/some/dir",
		AdminConsolePort:        8080,
		LocalArtifactMirrorPort: 50000,           // default
		GlobalCIDR:              "10.244.0.0/16", // default
	}
	expectedRc := runtimeconfig.New(&ecv1beta1.RuntimeConfigSpec{
		DataDir: "/some/dir",
		AdminConsole: ecv1beta1.AdminConsoleSpec{
			Port: 8080,
		},
		LocalArtifactMirror: ecv1beta1.LocalArtifactMirrorSpec{
			Port: 50000,
		},
		Network: ecv1beta1.NetworkSpec{
			GlobalCIDR: "10.244.0.0/16",
		},
	})

	err := store.SetConfig(expectedConfig)

	require.NoError(t, err)

	// Verify the config was stored
	actualConfig, err := store.GetConfig()
	require.NoError(t, err)
	assert.Equal(t, &expectedConfig, actualConfig)

	// Verify the runtime config was updated
	assert.Equal(t, expectedRc, store.rc)
}

func TestMemoryStore_GetStatus(t *testing.T) {
	rc := runtimeconfig.New(nil)
	status := &types.Status{
		State:       "failed",
		Description: "Failure",
	}
	store := NewMemoryStore(rc, status)

	status, err := store.GetStatus()

	require.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, &types.Status{
		State:       "failed",
		Description: "Failure",
	}, status)
}

func TestMemoryStore_SetStatus(t *testing.T) {
	rc := runtimeconfig.New(nil)
	status := &types.Status{
		State:       "failed",
		Description: "Failure",
	}
	store := NewMemoryStore(rc, status)
	expectedStatus := types.Status{
		State:       "running",
		Description: "Running",
	}

	err := store.SetStatus(expectedStatus)

	require.NoError(t, err)

	// Verify the status was stored
	actualStatus, err := store.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, &expectedStatus, actualStatus)
}

// Useful to test concurrent access with -race flag
func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	rc := runtimeconfig.New(nil)
	status := types.NewStatus()
	store := NewMemoryStore(rc, status)
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
				config := types.InstallationConfig{
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
