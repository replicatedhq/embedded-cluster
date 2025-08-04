package install

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
	appInstall := types.AppInstall{
		Status: types.Status{
			State: types.StatePending,
		},
		Logs: "",
	}
	return NewMemoryStore(WithAppInstall(appInstall))
}

func TestNewMemoryStore(t *testing.T) {
	store := newMemoryStore()

	assert.NotNil(t, store)
	appInstall, err := store.Get()
	require.NoError(t, err)
	assert.Equal(t, types.StatePending, appInstall.Status.State)
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
		Description: "Installing application",
		LastUpdated: time.Now(),
	}
	err = store.SetStatus(newStatus)
	require.NoError(t, err)

	// Test getting updated status
	status, err = store.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, types.StateRunning, status.State)
	assert.Equal(t, "Installing application", status.Description)
}

func TestMemoryStore_SetStatusDesc(t *testing.T) {
	store := newMemoryStore()

	// Set initial status first
	err := store.SetStatus(types.Status{
		State:       types.StateRunning,
		Description: "Initial description",
		LastUpdated: time.Now(),
	})
	require.NoError(t, err)

	// Test setting status description
	err = store.SetStatusDesc("New description")
	require.NoError(t, err)

	// Verify the description was updated
	status, err := store.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, "New description", status.Description)
	assert.Equal(t, types.StateRunning, status.State) // State should remain unchanged
}

func TestMemoryStore_SetStatusDesc_WithoutStatus(t *testing.T) {
	store := &memoryStore{
		appInstall: types.AppInstall{},
	}

	// Test setting status description when status state is empty
	err := store.SetStatusDesc("Should fail")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state not set")
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

	// Test getting app install
	appInstall, err := store.Get()
	require.NoError(t, err)
	assert.Empty(t, appInstall.Logs)
	assert.Equal(t, types.StatePending, appInstall.Status.State)

	// Add logs and update status
	err = store.AddLogs("Test log")
	require.NoError(t, err)
	err = store.SetStatus(types.Status{
		State:       types.StateRunning,
		Description: "Installing",
		LastUpdated: time.Now(),
	})
	require.NoError(t, err)

	// Test getting updated app install
	appInstall, err = store.Get()
	require.NoError(t, err)
	assert.Equal(t, "Test log\n", appInstall.Logs)
	assert.Equal(t, types.StateRunning, appInstall.Status.State)
	assert.Equal(t, "Installing", appInstall.Status.Description)
}

// Test concurrent access to ensure thread safety
func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := newMemoryStore()
	var wg sync.WaitGroup

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
					LastUpdated: time.Now(),
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

	// Concurrent app install reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
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

func TestMemoryStore_DeepCopy(t *testing.T) {
	store := newMemoryStore()

	// Set up initial state
	err := store.SetStatus(types.Status{
		State:       types.StateRunning,
		Description: "Original description",
		LastUpdated: time.Now(),
	})
	require.NoError(t, err)
	err = store.AddLogs("Original log")
	require.NoError(t, err)

	// Get the app install
	appInstall1, err := store.Get()
	require.NoError(t, err)

	// Get it again
	appInstall2, err := store.Get()
	require.NoError(t, err)

	// Modify one copy
	appInstall1.Status.Description = "Modified description"
	appInstall1.Logs = "Modified logs"

	// Verify the other copy wasn't affected (deep copy working)
	assert.Equal(t, "Original description", appInstall2.Status.Description)
	assert.Equal(t, "Original log\n", appInstall2.Logs)

	// Verify the store wasn't affected
	appInstall3, err := store.Get()
	require.NoError(t, err)
	assert.Equal(t, "Original description", appInstall3.Status.Description)
	assert.Equal(t, "Original log\n", appInstall3.Logs)
}