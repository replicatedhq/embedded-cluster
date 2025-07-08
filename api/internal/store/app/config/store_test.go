package config

import (
	"sync"
	"testing"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMemoryStore() Store {
	return NewMemoryStore()
}

func newMemoryStoreWithConfigValues(configValues kotsv1beta1.ConfigValues) Store {
	return NewMemoryStore(WithConfigValues(configValues))
}

func TestNewMemoryStore(t *testing.T) {
	store := newMemoryStore()

	assert.NotNil(t, store)
	configValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, kotsv1beta1.ConfigValues{}, configValues)
}

func TestNewMemoryStoreWithConfigValues(t *testing.T) {
	initialConfigValues := kotsv1beta1.ConfigValues{
		Spec: kotsv1beta1.ConfigValuesSpec{
			Values: map[string]kotsv1beta1.ConfigValue{
				"test-key": {
					Value: "test-value",
				},
			},
		},
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
	assert.Equal(t, kotsv1beta1.ConfigValues{}, configValues)

	// Test setting config values
	newConfigValues := kotsv1beta1.ConfigValues{
		Spec: kotsv1beta1.ConfigValuesSpec{
			Values: map[string]kotsv1beta1.ConfigValue{
				"db_host": {
					Value: "db.example.com",
				},
				"db_port": {
					Value: "5432",
				},
			},
		},
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

	// Set initial config values
	configValues1 := kotsv1beta1.ConfigValues{
		Spec: kotsv1beta1.ConfigValuesSpec{
			Values: map[string]kotsv1beta1.ConfigValue{
				"group1_key": {
					Value: "group1_value",
				},
			},
		},
	}

	err := store.SetConfigValues(configValues1)
	require.NoError(t, err)

	// Verify first config values
	retrievedConfigValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, configValues1, retrievedConfigValues)

	// Set second config values
	configValues2 := kotsv1beta1.ConfigValues{
		Spec: kotsv1beta1.ConfigValuesSpec{
			Values: map[string]kotsv1beta1.ConfigValue{
				"group2_key": {
					Value: "group2_value",
				},
			},
		},
	}

	err = store.SetConfigValues(configValues2)
	require.NoError(t, err)

	// Verify second config values
	retrievedConfigValues, err = store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, configValues2, retrievedConfigValues)
}

func TestMemoryStore_DeepCopyValues(t *testing.T) {
	store := newMemoryStore()

	// Set initial config values
	initialConfigValues := kotsv1beta1.ConfigValues{
		Spec: kotsv1beta1.ConfigValuesSpec{
			Values: map[string]kotsv1beta1.ConfigValue{
				"test-item": {
					Value: "custom-value",
				},
			},
		},
	}

	err := store.SetConfigValues(initialConfigValues)
	require.NoError(t, err)

	// Get config and modify it
	retrievedConfigValues, err := store.GetConfigValues()
	require.NoError(t, err)

	// Modify the retrieved config
	modifiedValue := retrievedConfigValues.Spec.Values["test-item"]
	modifiedValue.Value = "modified-value"
	retrievedConfigValues.Spec.Values["test-item"] = modifiedValue

	// Get config again to verify it wasn't modified in store
	originalConfigValues, err := store.GetConfigValues()
	require.NoError(t, err)

	// The store should still have the original value
	assert.Equal(t, "custom-value", originalConfigValues.Spec.Values["test-item"].Value)
	assert.Equal(t, "modified-value", retrievedConfigValues.Spec.Values["test-item"].Value)
}

func TestMemoryStore_ConcurrentValuesAccess(t *testing.T) {
	store := newMemoryStore()
	var wg sync.WaitGroup
	numGoroutines := 10

	// Test concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.GetConfigValues()
			assert.NoError(t, err)
		}()
	}

	wg.Wait()

	// Test concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			configValues := kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"group": {
							Value: "value",
						},
					},
				},
			}
			err := store.SetConfigValues(configValues)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Verify final state
	configValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.NotEqual(t, kotsv1beta1.ConfigValues{}, configValues)
}

func TestMemoryStore_EmptyConfigValues(t *testing.T) {
	store := newMemoryStore()

	// Test setting empty config values
	emptyConfigValues := kotsv1beta1.ConfigValues{}
	err := store.SetConfigValues(emptyConfigValues)
	require.NoError(t, err)

	// Test getting empty config values
	configValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, emptyConfigValues, configValues)
}

func TestMemoryStore_ComplexConfigValues(t *testing.T) {
	store := newMemoryStore()

	complexConfigValues := kotsv1beta1.ConfigValues{
		Spec: kotsv1beta1.ConfigValuesSpec{
			Values: map[string]kotsv1beta1.ConfigValue{
				"db_host": {
					Value: "db.example.com",
				},
				"db_port": {
					Value: "5432",
				},
				"db_ssl": {
					Value: "true",
				},
				"redis_host": {
					Value: "redis.example.com",
				},
				"redis_port": {
					Value: "6379",
				},
			},
		},
	}

	err := store.SetConfigValues(complexConfigValues)
	require.NoError(t, err)

	// Test getting complex config values
	configValues, err := store.GetConfigValues()
	require.NoError(t, err)
	assert.Equal(t, complexConfigValues, configValues)

	// Verify specific values
	assert.Len(t, configValues.Spec.Values, 5)
	assert.Equal(t, "db.example.com", configValues.Spec.Values["db_host"].Value)
	assert.Equal(t, "redis.example.com", configValues.Spec.Values["redis_host"].Value)
}
