package config

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMemoryStore() Store {
	return NewMemoryStore()
}

func newMemoryStoreWithConfigValues(configValues map[string]string) Store {
	return NewMemoryStore(WithConfigValues(configValues))
}

func TestNewMemoryStore(t *testing.T) {
	store := newMemoryStore()

	assert.NotNil(t, store)
	configValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{}, configValues)
}

func TestNewMemoryStoreWithConfigValues(t *testing.T) {
	initialConfigValues := map[string]string{
		"test-key": "test-value",
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
	assert.Equal(t, map[string]string{}, configValues)

	// Test setting config values
	newConfigValues := map[string]string{
		"db_host": "db.example.com",
		"db_port": "5432",
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
	initialValues := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	err := store.SetConfigValues(initialValues)
	require.NoError(t, err)

	// Verify initial values
	retrievedValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, initialValues, retrievedValues)

	// Set new values
	newValues := map[string]string{
		"key3": "value3",
		"key4": "value4",
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
	initialValues := map[string]string{
		"initial-key": "initial-value",
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
				value := fmt.Sprintf("value-%d-%d", id, j)
				err := store.SetConfigValues(map[string]string{key: value})
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
	initialValues := map[string]string{
		"test-item": "original-value",
	}
	err := store.SetConfigValues(initialValues)
	require.NoError(t, err)

	// Get values and modify the returned map
	retrievedConfigValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, "original-value", retrievedConfigValues["test-item"])

	// Modify the retrieved values
	retrievedConfigValues["test-item"] = "modified-value"

	// Get values again and verify they weren't affected by the modification
	originalConfigValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, "original-value", originalConfigValues["test-item"])
	assert.Equal(t, "modified-value", retrievedConfigValues["test-item"])
}

func TestMemoryStore_EmptyConfigValues(t *testing.T) {
	store := newMemoryStore()

	// Set empty values
	err := store.SetConfigValues(map[string]string{})
	require.NoError(t, err)

	// Get values
	configValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{}, configValues)
}

func TestMemoryStore_ComplexConfigValues(t *testing.T) {
	store := newMemoryStore()

	// Set complex values with various string types
	complexValues := map[string]string{
		"empty":          "",
		"simple":         "value",
		"with_spaces":    "value with spaces",
		"with_special":   "value!@#$%^&*()",
		"with_unicode":   "value with unicode: 🚀",
		"with_newlines":  "value\nwith\nnewlines",
		"with_quotes":    `value with "quotes"`,
		"very_long":      "very long value that might exceed some buffer sizes and cause issues with memory allocation or string handling",
		"numeric_string": "12345",
		"boolean_string": "true",
	}

	err := store.SetConfigValues(complexValues)
	require.NoError(t, err)

	// Get values
	retrievedValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, complexValues, retrievedValues)
}
