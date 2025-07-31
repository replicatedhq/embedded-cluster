package preflight

import (
	"sync"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryStore(t *testing.T) {
	hostPreflight := types.HostPreflights{}
	store := NewMemoryStore(WithHostPreflight(hostPreflight))

	assert.NotNil(t, store)
	assert.Equal(t, hostPreflight, store.hostPreflight)
}

func TestMemoryStore_GetTitles(t *testing.T) {
	hostPreflight := types.HostPreflights{
		Titles: []string{"Memory Check", "Disk Space Check", "Network Check"},
	}
	store := NewMemoryStore(WithHostPreflight(hostPreflight))

	titles, err := store.GetTitles()

	require.NoError(t, err)
	assert.NotNil(t, titles)
	assert.Equal(t, []string{"Memory Check", "Disk Space Check", "Network Check"}, titles)
}

func TestMemoryStore_GetTitles_Empty(t *testing.T) {
	hostPreflight := types.HostPreflights{
		Titles: []string{},
	}
	store := NewMemoryStore(WithHostPreflight(hostPreflight))

	titles, err := store.GetTitles()

	require.NoError(t, err)
	assert.NotNil(t, titles)
	assert.Empty(t, titles)
}

func TestMemoryStore_SetTitles(t *testing.T) {
	hostPreflight := types.HostPreflights{
		Titles: []string{"Old Title"},
	}
	store := NewMemoryStore(WithHostPreflight(hostPreflight))
	expectedTitles := []string{"CPU Check", "RAM Check", "Storage Check"}

	err := store.SetTitles(expectedTitles)

	require.NoError(t, err)

	// Verify the titles were stored
	actualTitles, err := store.GetTitles()
	require.NoError(t, err)
	assert.Equal(t, expectedTitles, actualTitles)
}

func TestMemoryStore_GetOutput(t *testing.T) {
	output := &types.PreflightsOutput{}
	hostPreflight := types.HostPreflights{
		Output: output,
	}
	store := NewMemoryStore(WithHostPreflight(hostPreflight))

	result, err := store.GetOutput()

	require.NoError(t, err)
	assert.Equal(t, output, result)
}

func TestMemoryStore_GetOutput_Nil(t *testing.T) {
	hostPreflight := types.HostPreflights{
		Output: nil,
	}
	store := NewMemoryStore(WithHostPreflight(hostPreflight))

	result, err := store.GetOutput()

	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestMemoryStore_SetOutput(t *testing.T) {
	hostPreflight := types.HostPreflights{
		Output: nil,
	}
	store := NewMemoryStore(WithHostPreflight(hostPreflight))
	expectedOutput := &types.PreflightsOutput{}

	err := store.SetOutput(expectedOutput)

	require.NoError(t, err)

	// Verify the output was stored
	actualOutput, err := store.GetOutput()
	require.NoError(t, err)
	assert.Equal(t, expectedOutput, actualOutput)
}

func TestMemoryStore_SetOutput_Nil(t *testing.T) {
	hostPreflight := types.HostPreflights{
		Output: &types.PreflightsOutput{},
	}
	store := NewMemoryStore(WithHostPreflight(hostPreflight))

	err := store.SetOutput(nil)

	require.NoError(t, err)

	// Verify the output was set to nil
	actualOutput, err := store.GetOutput()
	require.NoError(t, err)
	assert.Nil(t, actualOutput)
}

func TestMemoryStore_GetStatus(t *testing.T) {
	status := types.Status{
		State:       types.StateRunning,
		Description: "Running host preflights",
		LastUpdated: time.Now(),
	}
	hostPreflight := types.HostPreflights{
		Status: status,
	}
	store := NewMemoryStore(WithHostPreflight(hostPreflight))

	result, err := store.GetStatus()

	require.NoError(t, err)
	assert.Equal(t, status, result)
}

func TestMemoryStore_SetStatus(t *testing.T) {
	hostPreflight := types.HostPreflights{
		Status: types.Status{
			State:       types.StateFailed,
			Description: "Failed",
		},
	}
	store := NewMemoryStore(WithHostPreflight(hostPreflight))
	expectedStatus := types.Status{
		State:       types.StateSucceeded,
		Description: "Host preflights passed",
		LastUpdated: time.Now(),
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
	hostPreflight := types.HostPreflights{}
	store := NewMemoryStore(WithHostPreflight(hostPreflight))
	var wg sync.WaitGroup

	// Test concurrent reads and writes
	numGoroutines := 10
	numOperations := 50

	// Concurrent titles operations
	wg.Add(numGoroutines * 2)
	for i := 0; i < numGoroutines; i++ {
		// Concurrent writes
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				titles := []string{"Memory Check", "Disk Check", "Network Check"}
				err := store.SetTitles(titles)
				assert.NoError(t, err)
			}
		}(i)

		// Concurrent reads
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, err := store.GetTitles()
				assert.NoError(t, err)
			}
		}(i)
	}

	// Concurrent output operations
	wg.Add(numGoroutines * 2)
	for i := 0; i < numGoroutines; i++ {
		// Concurrent writes
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				output := &types.PreflightsOutput{}
				err := store.SetOutput(output)
				assert.NoError(t, err)
			}
		}(i)

		// Concurrent reads
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, err := store.GetOutput()
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
					State:       types.StateRunning,
					Description: "Running",
					LastUpdated: time.Now(),
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
