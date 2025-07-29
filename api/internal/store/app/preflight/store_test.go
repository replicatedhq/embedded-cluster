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
	appPreflight := types.AppPreflights{}
	store := NewMemoryStore(WithAppPreflight(appPreflight))

	assert.NotNil(t, store)
	assert.Equal(t, appPreflight, store.appPreflight)
}

func TestMemoryStore_GetTitles(t *testing.T) {
	appPreflight := types.AppPreflights{
		Titles: []string{"RBAC Check", "Image Pull Check", "Connectivity Check"},
	}
	store := NewMemoryStore(WithAppPreflight(appPreflight))

	titles, err := store.GetTitles()

	require.NoError(t, err)
	assert.NotNil(t, titles)
	assert.Equal(t, []string{"RBAC Check", "Image Pull Check", "Connectivity Check"}, titles)
}

func TestMemoryStore_GetTitles_Empty(t *testing.T) {
	appPreflight := types.AppPreflights{
		Titles: []string{},
	}
	store := NewMemoryStore(WithAppPreflight(appPreflight))

	titles, err := store.GetTitles()

	require.NoError(t, err)
	assert.NotNil(t, titles)
	assert.Empty(t, titles)
}

func TestMemoryStore_SetTitles(t *testing.T) {
	appPreflight := types.AppPreflights{
		Titles: []string{"Old Title"},
	}
	store := NewMemoryStore(WithAppPreflight(appPreflight))
	expectedTitles := []string{"Cluster Access Check", "Pod Creation Check", "Service Check"}

	err := store.SetTitles(expectedTitles)

	require.NoError(t, err)

	// Verify the titles were stored
	actualTitles, err := store.GetTitles()
	require.NoError(t, err)
	assert.Equal(t, expectedTitles, actualTitles)
}

func TestMemoryStore_GetOutput(t *testing.T) {
	output := &types.PreflightsOutput{}
	appPreflight := types.AppPreflights{
		Output: output,
	}
	store := NewMemoryStore(WithAppPreflight(appPreflight))

	result, err := store.GetOutput()

	require.NoError(t, err)
	assert.Equal(t, output, result)
}

func TestMemoryStore_GetOutput_Nil(t *testing.T) {
	appPreflight := types.AppPreflights{
		Output: nil,
	}
	store := NewMemoryStore(WithAppPreflight(appPreflight))

	result, err := store.GetOutput()

	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestMemoryStore_SetOutput(t *testing.T) {
	appPreflight := types.AppPreflights{
		Output: nil,
	}
	store := NewMemoryStore(WithAppPreflight(appPreflight))
	expectedOutput := &types.PreflightsOutput{}

	err := store.SetOutput(expectedOutput)

	require.NoError(t, err)

	// Verify the output was stored
	actualOutput, err := store.GetOutput()
	require.NoError(t, err)
	assert.Equal(t, expectedOutput, actualOutput)
}

func TestMemoryStore_SetOutput_Nil(t *testing.T) {
	appPreflight := types.AppPreflights{
		Output: &types.PreflightsOutput{},
	}
	store := NewMemoryStore(WithAppPreflight(appPreflight))

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
		Description: "Running app preflights",
		LastUpdated: time.Now(),
	}
	appPreflight := types.AppPreflights{
		Status: status,
	}
	store := NewMemoryStore(WithAppPreflight(appPreflight))

	result, err := store.GetStatus()

	require.NoError(t, err)
	assert.Equal(t, status, result)
}

func TestMemoryStore_SetStatus(t *testing.T) {
	appPreflight := types.AppPreflights{
		Status: types.Status{
			State:       types.StateFailed,
			Description: "Failed",
		},
	}
	store := NewMemoryStore(WithAppPreflight(appPreflight))
	expectedStatus := types.Status{
		State:       types.StateSucceeded,
		Description: "App preflights passed",
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
	appPreflight := types.AppPreflights{}
	store := NewMemoryStore(WithAppPreflight(appPreflight))
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
				titles := []string{"RBAC Check", "Image Pull Check", "Connectivity Check"}
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
