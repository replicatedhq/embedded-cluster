package appupgrade

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
	appUpgrade := types.AppUpgrade{
		Status: types.Status{
			State: types.StatePending,
		},
		Logs: "",
	}
	return NewMemoryStore(WithAppUpgrade(appUpgrade))
}

func TestNewMemoryStore(t *testing.T) {
	store := newMemoryStore()

	assert.NotNil(t, store)
	appUpgrade, err := store.Get()
	require.NoError(t, err)
	assert.Equal(t, types.StatePending, appUpgrade.Status.State)
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
		Description: "Upgrading application",
		LastUpdated: time.Now(),
	}
	err = store.SetStatus(newStatus)
	require.NoError(t, err)

	// Test getting updated status
	status, err = store.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, types.StateRunning, status.State)
	assert.Equal(t, "Upgrading application", status.Description)
}

func TestMemoryStore_AddLogs(t *testing.T) {
	store := newMemoryStore()

	// Test adding logs (AddLogs automatically adds newline)
	err := store.AddLogs("First log entry")
	require.NoError(t, err)

	appUpgrade, err := store.Get()
	require.NoError(t, err)
	assert.Equal(t, "First log entry\n", appUpgrade.Logs)

	// Test adding more logs
	err = store.AddLogs("Second log entry")
	require.NoError(t, err)

	appUpgrade, err = store.Get()
	require.NoError(t, err)
	assert.Equal(t, "First log entry\nSecond log entry\n", appUpgrade.Logs)
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := newMemoryStore()
	var wg sync.WaitGroup
	numGoroutines := 10

	// Test concurrent status updates
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			status := types.Status{
				State:       types.StateRunning,
				Description: "Concurrent update",
				LastUpdated: time.Now(),
			}
			err := store.SetStatus(status)
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	// Test concurrent log appends
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			err := store.AddLogs("Log entry from goroutine")
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	// Verify final state
	appUpgrade, err := store.Get()
	require.NoError(t, err)
	assert.Equal(t, types.StateRunning, appUpgrade.Status.State)

	// Should have 10 log entries (each with automatic newline)
	logCount := strings.Count(appUpgrade.Logs, "Log entry from goroutine\n")
	assert.Equal(t, numGoroutines, logCount)
}

func TestMemoryStore_WithAppUpgrade(t *testing.T) {
	customUpgrade := types.AppUpgrade{
		Status: types.Status{
			State:       types.StateSucceeded,
			Description: "Custom upgrade completed",
			LastUpdated: time.Now(),
		},
		Logs: "Custom logs",
	}

	store := NewMemoryStore(WithAppUpgrade(customUpgrade))
	appUpgrade, err := store.Get()
	require.NoError(t, err)

	assert.Equal(t, types.StateSucceeded, appUpgrade.Status.State)
	assert.Equal(t, "Custom upgrade completed", appUpgrade.Status.Description)
	assert.Equal(t, "Custom logs", appUpgrade.Logs)
}

func TestMemoryStore_SetStatus_UpdatesTimestamp(t *testing.T) {
	store := newMemoryStore()

	// Get initial timestamp
	initialStatus, err := store.GetStatus()
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond) // Ensure time difference

	// Update status
	newStatus := types.Status{
		State:       types.StateRunning,
		Description: "Upgrading application",
		LastUpdated: time.Now(),
	}
	err = store.SetStatus(newStatus)
	require.NoError(t, err)

	// Verify timestamp was updated
	updatedStatus, err := store.GetStatus()
	require.NoError(t, err)
	assert.True(t, updatedStatus.LastUpdated.After(initialStatus.LastUpdated))
}
