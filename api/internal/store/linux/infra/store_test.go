package infra

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMemoryStore() Store {
	infra := types.LinuxInfra{
		Status: types.Status{
			State: types.StatePending,
		},
		Components: []types.LinuxInfraComponent{},
		Logs:       "",
	}
	return NewMemoryStore(WithInfra(infra))
}

func TestNewMemoryStore(t *testing.T) {
	store := newMemoryStore()

	assert.NotNil(t, store)
	infra, err := store.Get()
	require.NoError(t, err)
	assert.Equal(t, types.StatePending, infra.Status.State)
}

func TestMemoryStore_GetAndSetStatus(t *testing.T) {
	store := newMemoryStore()

	// Test initial status
	status, err := store.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, types.StatePending, status.State)

	// Test setting status
	newStatus := types.Status{
		State:       types.StateRunning,
		Description: "Installing components",
	}
	err = store.SetStatus(newStatus)
	require.NoError(t, err)

	// Test getting updated status
	status, err = store.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, types.StateRunning, status.State)
	assert.Equal(t, "Installing components", status.Description)
}

func TestMemoryStore_SetStatusDesc(t *testing.T) {
	store := newMemoryStore()

	// Test setting status description
	err := store.SetStatusDesc("New description")
	require.NoError(t, err)

	// Verify the description was updated
	status, err := store.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, "New description", status.Description)
	assert.Equal(t, types.StatePending, status.State) // State should remain unchanged
}

func TestMemoryStore_RegisterComponent(t *testing.T) {
	store := newMemoryStore()

	// Test registering a component
	err := store.RegisterComponent("k0s")
	require.NoError(t, err)

	// Verify component was added
	infra, err := store.Get()
	require.NoError(t, err)
	assert.Len(t, infra.Components, 1)
	assert.Equal(t, "k0s", infra.Components[0].Name)
	assert.Equal(t, types.StatePending, infra.Components[0].Status.State)

	// Test registering another component
	err = store.RegisterComponent("addons")
	require.NoError(t, err)

	infra, err = store.Get()
	require.NoError(t, err)
	assert.Len(t, infra.Components, 2)
}

func TestMemoryStore_SetComponentStatus(t *testing.T) {
	store := newMemoryStore()

	// Register a component first
	err := store.RegisterComponent("k0s")
	require.NoError(t, err)

	// Test setting component status
	now := time.Now()
	componentStatus := types.Status{
		State:       types.StateRunning,
		Description: "Installing k0s",
		LastUpdated: now,
	}
	err = store.SetComponentStatus("k0s", componentStatus)
	require.NoError(t, err)

	// Verify the component status was updated
	infra, err := store.Get()
	require.NoError(t, err)
	assert.Len(t, infra.Components, 1)
	assert.Equal(t, types.StateRunning, infra.Components[0].Status.State)
	assert.Equal(t, "Installing k0s", infra.Components[0].Status.Description)
	assert.Equal(t, now, infra.Components[0].Status.LastUpdated)

	// Test setting status for non-existent component
	err = store.SetComponentStatus("nonexistent", componentStatus)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "component nonexistent not found")
}

func TestMemoryStore_AddLogs(t *testing.T) {
	store := newMemoryStore()

	// Test adding logs
	err := store.AddLogs("First log entry")
	require.NoError(t, err)

	logs, err := store.GetLogs()
	require.NoError(t, err)
	assert.Equal(t, "First log entry\n", logs)

	// Test adding more logs
	err = store.AddLogs("Second log entry")
	require.NoError(t, err)

	logs, err = store.GetLogs()
	require.NoError(t, err)
	assert.Equal(t, "First log entry\nSecond log entry\n", logs)
}

func TestMemoryStore_LogTruncation(t *testing.T) {
	store := newMemoryStore()

	// Create a large log entry that exceeds maxLogSize
	largeLog := strings.Repeat("a", maxLogSize+1000)
	err := store.AddLogs(largeLog)
	require.NoError(t, err)

	logs, err := store.GetLogs()
	require.NoError(t, err)

	// Should be truncated and contain the truncation message
	assert.True(t, len(logs) <= maxLogSize+50) // Allow some buffer for truncation message
	assert.Contains(t, logs, "... (truncated)")
}

func TestMemoryStore_GetLogs(t *testing.T) {
	store := newMemoryStore()

	// Test getting logs when empty
	logs, err := store.GetLogs()
	require.NoError(t, err)
	assert.Empty(t, logs)

	// Add some logs and test retrieval
	err = store.AddLogs("Test log 1")
	require.NoError(t, err)
	err = store.AddLogs("Test log 2")
	require.NoError(t, err)

	logs, err = store.GetLogs()
	require.NoError(t, err)
	assert.Equal(t, "Test log 1\nTest log 2\n", logs)
}

func TestMemoryStore_Get(t *testing.T) {
	store := newMemoryStore()

	// Test getting infra
	infra, err := store.Get()
	require.NoError(t, err)
	assert.Empty(t, infra.Components)
	assert.Empty(t, infra.Logs)

	// Register a component and add logs
	err = store.RegisterComponent("k0s")
	require.NoError(t, err)
	err = store.AddLogs("Test log")
	require.NoError(t, err)

	// Test getting updated infra
	infra, err = store.Get()
	require.NoError(t, err)
	assert.Len(t, infra.Components, 1)
	assert.Equal(t, "Test log\n", infra.Logs)
}

// Test concurrent access to ensure thread safety
func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := newMemoryStore()
	var wg sync.WaitGroup

	// Register a component first
	err := store.RegisterComponent("k0s")
	require.NoError(t, err)

	numGoroutines := 10
	numOperations := 50

	// Concurrent status operations
	wg.Add(numGoroutines * 2)
	for i := 0; i < numGoroutines; i++ {
		// Concurrent status writes
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				status := types.Status{
					State:       types.StateRunning,
					Description: "Concurrent test",
				}
				err := store.SetStatus(status)
				assert.NoError(t, err)
			}
		}(i)

		// Concurrent status reads
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, err := store.GetStatus()
				assert.NoError(t, err)
			}
		}(i)
	}

	// Concurrent log operations
	wg.Add(numGoroutines * 2)
	for i := 0; i < numGoroutines; i++ {
		// Concurrent log writes
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				err := store.AddLogs("Concurrent log")
				assert.NoError(t, err)
			}
		}(i)

		// Concurrent log reads
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, err := store.GetLogs()
				assert.NoError(t, err)
			}
		}(i)
	}

	// Concurrent component operations
	wg.Add(numGoroutines * 2)
	for i := 0; i < numGoroutines; i++ {
		// Concurrent component status writes
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				status := types.Status{
					State:       types.StateRunning,
					Description: "Concurrent component test",
				}
				err := store.SetComponentStatus("k0s", status)
				assert.NoError(t, err)
			}
		}(i)

		// Concurrent infra reads
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, err := store.Get()
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()
}

func TestMemoryStore_StatusDescWithoutStatus(t *testing.T) {
	store := &memoryStore{
		infra: types.LinuxInfra{},
	}

	// Test setting status description when status is nil
	err := store.SetStatusDesc("Should fail")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state not set")
}
