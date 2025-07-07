package config

import (
	"sync"
	"testing"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMemoryStore() Store {
	return NewMemoryStore()
}

func newMemoryStoreWithConfig(config kotsv1beta1.Config) Store {
	return NewMemoryStore(WithConfig(config))
}

func TestNewMemoryStore(t *testing.T) {
	store := newMemoryStore()

	assert.NotNil(t, store)
	config, err := store.Get()
	require.NoError(t, err)
	assert.Equal(t, kotsv1beta1.Config{}, config)
}

func TestNewMemoryStoreWithConfig(t *testing.T) {
	initialConfig := kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:        "test-group",
					Title:       "Test Group",
					Description: "A test group",
				},
			},
		},
	}

	store := newMemoryStoreWithConfig(initialConfig)

	assert.NotNil(t, store)
	config, err := store.Get()
	require.NoError(t, err)
	assert.Equal(t, initialConfig, config)
}

func TestMemoryStore_GetAndSet(t *testing.T) {
	store := newMemoryStore()

	// Test initial empty config
	config, err := store.Get()
	require.NoError(t, err)
	assert.Equal(t, kotsv1beta1.Config{}, config)

	// Test setting config
	newConfig := kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:        "database",
					Title:       "Database Configuration",
					Description: "Database settings",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "db_host",
							Type:    "text",
							Title:   "Database Host",
							Default: multitype.BoolOrString{StrVal: "localhost"},
							Value:   multitype.BoolOrString{StrVal: "db.example.com"},
						},
					},
				},
			},
		},
	}

	err = store.Set(newConfig)
	require.NoError(t, err)

	// Test getting updated config
	config, err = store.Get()
	require.NoError(t, err)
	assert.Equal(t, newConfig, config)
}

func TestMemoryStore_SetMultipleTimes(t *testing.T) {
	store := newMemoryStore()

	// Set initial config
	config1 := kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "group1",
					Title: "Group 1",
				},
			},
		},
	}

	err := store.Set(config1)
	require.NoError(t, err)

	// Verify first config
	retrievedConfig, err := store.Get()
	require.NoError(t, err)
	assert.Equal(t, config1, retrievedConfig)

	// Set second config
	config2 := kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "group2",
					Title: "Group 2",
				},
			},
		},
	}

	err = store.Set(config2)
	require.NoError(t, err)

	// Verify second config
	retrievedConfig, err = store.Get()
	require.NoError(t, err)
	assert.Equal(t, config2, retrievedConfig)
}

func TestMemoryStore_DeepCopy(t *testing.T) {
	store := newMemoryStore()

	// Set initial config
	initialConfig := kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "test-group",
					Title: "Test Group",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "test-item",
							Type:    "text",
							Title:   "Test Item",
							Default: multitype.BoolOrString{StrVal: "default-value"},
							Value:   multitype.BoolOrString{StrVal: "custom-value"},
						},
					},
				},
			},
		},
	}

	err := store.Set(initialConfig)
	require.NoError(t, err)

	// Get config and modify it
	retrievedConfig, err := store.Get()
	require.NoError(t, err)

	// Modify the retrieved config
	retrievedConfig.Spec.Groups[0].Items[0].Value = multitype.BoolOrString{StrVal: "modified-value"}

	// Get config again to verify it wasn't modified in store
	originalConfig, err := store.Get()
	require.NoError(t, err)

	// The store should still have the original value
	assert.Equal(t, "custom-value", originalConfig.Spec.Groups[0].Items[0].Value.StrVal)
	assert.Equal(t, "modified-value", retrievedConfig.Spec.Groups[0].Items[0].Value.StrVal)
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := newMemoryStore()
	var wg sync.WaitGroup
	numGoroutines := 10

	// Test concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Get()
			assert.NoError(t, err)
		}()
	}

	wg.Wait()

	// Test concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			config := kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "group",
							Title: "Group",
						},
					},
				},
			}
			err := store.Set(config)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	// Verify final state
	config, err := store.Get()
	require.NoError(t, err)
	assert.NotEqual(t, kotsv1beta1.Config{}, config)
}

func TestMemoryStore_EmptyConfig(t *testing.T) {
	store := newMemoryStore()

	// Test setting empty config
	emptyConfig := kotsv1beta1.Config{}
	err := store.Set(emptyConfig)
	require.NoError(t, err)

	// Test getting empty config
	config, err := store.Get()
	require.NoError(t, err)
	assert.Equal(t, emptyConfig, config)
}

func TestMemoryStore_ComplexConfig(t *testing.T) {
	store := newMemoryStore()

	complexConfig := kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:        "database",
					Title:       "Database Configuration",
					Description: "Database connection settings",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "db_host",
							Type:    "text",
							Title:   "Database Host",
							Default: multitype.BoolOrString{StrVal: "localhost"},
							Value:   multitype.BoolOrString{StrVal: "db.example.com"},
						},
						{
							Name:    "db_port",
							Type:    "number",
							Title:   "Database Port",
							Default: multitype.BoolOrString{StrVal: "5432"},
							Value:   multitype.BoolOrString{StrVal: "5432"},
						},
						{
							Name:    "db_ssl",
							Type:    "bool",
							Title:   "Use SSL",
							Default: multitype.BoolOrString{BoolVal: true},
							Value:   multitype.BoolOrString{BoolVal: true},
						},
					},
				},
				{
					Name:        "redis",
					Title:       "Redis Configuration",
					Description: "Redis connection settings",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "redis_host",
							Type:    "text",
							Title:   "Redis Host",
							Default: multitype.BoolOrString{StrVal: "localhost"},
							Value:   multitype.BoolOrString{StrVal: "redis.example.com"},
						},
						{
							Name:    "redis_port",
							Type:    "number",
							Title:   "Redis Port",
							Default: multitype.BoolOrString{StrVal: "6379"},
							Value:   multitype.BoolOrString{StrVal: "6379"},
						},
					},
				},
			},
		},
	}

	err := store.Set(complexConfig)
	require.NoError(t, err)

	// Test getting complex config
	config, err := store.Get()
	require.NoError(t, err)
	assert.Equal(t, complexConfig, config)

	// Verify specific values
	assert.Len(t, config.Spec.Groups, 2)
	assert.Equal(t, "database", config.Spec.Groups[0].Name)
	assert.Equal(t, "redis", config.Spec.Groups[1].Name)
	assert.Len(t, config.Spec.Groups[0].Items, 3)
	assert.Len(t, config.Spec.Groups[1].Items, 2)
}
