package config

import (
	"fmt"
	"sync"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMemoryStore() Store {
	return NewMemoryStore()
}

func newMemoryStoreWithConfigValues(configValues types.AppConfigValues) Store {
	return NewMemoryStore(WithConfigValues(configValues))
}

func TestNewMemoryStore(t *testing.T) {
	store := newMemoryStore()

	assert.NotNil(t, store)
	configValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, types.AppConfigValues{}, configValues)
}

func TestNewMemoryStoreWithConfigValues(t *testing.T) {
	initialConfigValues := types.AppConfigValues{
		"test-key": types.AppConfigValue{Value: "test-value"},
	}

	store := newMemoryStoreWithConfigValues(initialConfigValues)

	assert.NotNil(t, store)
	configValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, initialConfigValues, configValues)
}

func TestMemoryStore_GetAndSetConfigValues(t *testing.T) {
	store := newMemoryStore()

	// Test initial empty config values
	configValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, types.AppConfigValues{}, configValues)

	// Test setting config values
	newConfigValues := types.AppConfigValues{
		"db_host": types.AppConfigValue{Value: "db.example.com"},
		"db_port": types.AppConfigValue{Value: "5432"},
	}

	err = store.SetConfigValues(newConfigValues)
	require.NoError(t, err)

	// Test getting updated config values
	configValues, err = store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, newConfigValues, configValues)
}

func TestMemoryStore_SetConfigValuesMultipleTimes(t *testing.T) {
	store := newMemoryStore()

	// Set initial values
	initialValues := types.AppConfigValues{
		"key1": types.AppConfigValue{Value: "value1"},
		"key2": types.AppConfigValue{Value: "value2"},
	}
	err := store.SetConfigValues(initialValues)
	require.NoError(t, err)

	// Verify initial values
	retrievedValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, initialValues, retrievedValues)

	// Set new values
	newValues := types.AppConfigValues{
		"key3": types.AppConfigValue{Value: "value3"},
		"key4": types.AppConfigValue{Value: "value4"},
	}
	err = store.SetConfigValues(newValues)
	require.NoError(t, err)

	// Verify new values
	retrievedValues, err = store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, newValues, retrievedValues)

	// Verify old values are not present
	assert.NotContains(t, retrievedValues, "key1")
	assert.NotContains(t, retrievedValues, "key2")
}

func TestMemoryStore_ConcurrentValuesAccess(t *testing.T) {
	store := newMemoryStore()
	var wg sync.WaitGroup

	// Set initial config values first
	initialValues := types.AppConfigValues{
		"initial-key": types.AppConfigValue{Value: "initial-value"},
	}
	err := store.SetConfigValues(initialValues)
	require.NoError(t, err)

	numGoroutines := 10
	numOperations := 50

	// Concurrent config values operations
	wg.Add(numGoroutines * 2)
	for i := 0; i < numGoroutines; i++ {
		// Concurrent config values writes
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := fmt.Sprintf("goroutine-%d-key-%d", id, j)
				value := types.AppConfigValue{Value: fmt.Sprintf("value-%d-%d", id, j)}
				err := store.SetConfigValues(types.AppConfigValues{key: value})
				assert.NoError(t, err)
			}
		}(i)

		// Concurrent config values reads
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				_, err := store.GetConfigValues()
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()
}

func TestMemoryStore_DeepCopy(t *testing.T) {
	store := newMemoryStore()

	// Set initial values
	initialValues := types.AppConfigValues{
		"test-item": types.AppConfigValue{Value: "original-value"},
	}
	err := store.SetConfigValues(initialValues)
	require.NoError(t, err)

	// Get values and modify the returned map
	retrievedConfigValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, "original-value", retrievedConfigValues["test-item"])

	// Modify the retrieved values
	retrievedConfigValues["test-item"] = types.AppConfigValue{Value: "modified-value"}

	// Get values again and verify they weren't affected by the modification
	originalConfigValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, "original-value", originalConfigValues["test-item"])
	assert.Equal(t, "modified-value", retrievedConfigValues["test-item"])
}

func TestMemoryStore_EmptyConfigValues(t *testing.T) {
	store := newMemoryStore()

	// Set empty values
	err := store.SetConfigValues(types.AppConfigValues{})
	require.NoError(t, err)

	// Get values
	configValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, types.AppConfigValues{}, configValues)
}

func TestMemoryStore_ComplexConfigValues(t *testing.T) {
	store := newMemoryStore()

	// Set complex values with various string types
	complexValues := types.AppConfigValues{
		"empty":          types.AppConfigValue{Value: ""},
		"simple":         types.AppConfigValue{Value: "value"},
		"with_spaces":    types.AppConfigValue{Value: "value with spaces"},
		"with_special":   types.AppConfigValue{Value: "value!@#$%^&*()"},
		"with_unicode":   types.AppConfigValue{Value: "value with unicode: 🚀"},
		"with_newlines":  types.AppConfigValue{Value: "value\nwith\nnewlines"},
		"with_quotes":    types.AppConfigValue{Value: `value with "quotes"`},
		"very_long":      types.AppConfigValue{Value: "very long value that might exceed some buffer sizes and cause issues with memory allocation or string handling"},
		"numeric_string": types.AppConfigValue{Value: "12345"},
		"boolean_string": types.AppConfigValue{Value: "true"},
	}

	err := store.SetConfigValues(complexValues)
	require.NoError(t, err)

	// Get values
	retrievedValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, complexValues, retrievedValues)
}
