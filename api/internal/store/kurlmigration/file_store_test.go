package kurlmigration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFileStore(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)
	assert.NotNil(t, store)

	// Should return error when no kURL migration is initialized
	_, err := store.GetMigrationID()
	assert.ErrorIs(t, err, types.ErrNoActiveKURLMigration)
}

func TestFileStore_InitializeMigration(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(Store) // Optional setup before test
		migrationID string
		transferMode string
		config      types.LinuxInstallationConfig
		wantErr     error
		validate    func(*testing.T, Store) // Optional additional validation
	}{
		{
			name:         "success - creates file and initializes state",
			migrationID:  "test-migration-id",
			transferMode: "copy",
			config: types.LinuxInstallationConfig{
				DataDirectory: "/var/lib/embedded-cluster",
			},
			wantErr: nil,
			validate: func(t *testing.T, s Store) {
				// Verify migration ID
				id, err := s.GetMigrationID()
				require.NoError(t, err)
				assert.Equal(t, "test-migration-id", id)

				// Verify transfer mode
				mode, err := s.GetTransferMode()
				require.NoError(t, err)
				assert.Equal(t, "copy", mode)

				// Verify config
				cfg, err := s.GetConfig()
				require.NoError(t, err)
				assert.Equal(t, "/var/lib/embedded-cluster", cfg.DataDirectory)

				// Verify initial status
				status, err := s.GetStatus()
				require.NoError(t, err)
				assert.Equal(t, types.KURLMigrationStateNotStarted, status.State)
				assert.Equal(t, types.KURLMigrationPhaseDiscovery, status.Phase)
				assert.Equal(t, "", status.Message)
				assert.Equal(t, 0, status.Progress)
				assert.Equal(t, "", status.Error)
			},
		},
		{
			name:         "error - already initialized",
			migrationID:  "second-id",
			transferMode: "move",
			config:       types.LinuxInstallationConfig{},
			setup: func(s Store) {
				_ = s.InitializeMigration("first-id", "copy", types.LinuxInstallationConfig{})
			},
			wantErr: types.ErrKURLMigrationAlreadyStarted,
		},
		{
			name:         "success - includes pending user config",
			migrationID:  "test-id",
			transferMode: "copy",
			config: types.LinuxInstallationConfig{
				DataDirectory: "/var/lib/ec",
			},
			setup: func(s Store) {
				// Set user config before initialization
				_ = s.SetUserConfig(types.LinuxInstallationConfig{
					HTTPProxy: "http://proxy.example.com:8080",
				})
			},
			wantErr: nil,
			validate: func(t *testing.T, s Store) {
				// Verify user config was persisted
				userCfg, err := s.GetUserConfig()
				require.NoError(t, err)
				assert.Equal(t, "http://proxy.example.com:8080", userCfg.HTTPProxy)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)

			if tt.setup != nil {
				tt.setup(store)
			}

			err := store.InitializeMigration(tt.migrationID, tt.transferMode, tt.config)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)

			// Verify file was created
			statePath := filepath.Join(tmpDir, "migration-state.json")
			_, err = os.Stat(statePath)
			assert.NoError(t, err)

			if tt.validate != nil {
				tt.validate(t, store)
			}
		})
	}
}

func TestFileStore_SetState(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(Store)
		newState  types.KURLMigrationState
		wantErr   error
		wantState types.KURLMigrationState
	}{
		{
			name:     "error - no migration initialized",
			newState: types.KURLMigrationStateInProgress,
			wantErr:  types.ErrNoActiveKURLMigration,
		},
		{
			name: "success - updates state",
			setup: func(s Store) {
				_ = s.InitializeMigration("test-id", "copy", types.LinuxInstallationConfig{})
			},
			newState:  types.KURLMigrationStateInProgress,
			wantErr:   nil,
			wantState: types.KURLMigrationStateInProgress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)

			if tt.setup != nil {
				tt.setup(store)
			}

			err := store.SetState(tt.newState)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)

			status, err := store.GetStatus()
			require.NoError(t, err)
			assert.Equal(t, tt.wantState, status.State)

			// Verify persistence across instances
			store2 := NewFileStore(tmpDir)
			status2, err := store2.GetStatus()
			require.NoError(t, err)
			assert.Equal(t, tt.wantState, status2.State)
		})
	}
}

func TestFileStore_SetPhase(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	config := types.LinuxInstallationConfig{}
	err := store.InitializeMigration("test-id", "copy", config)
	require.NoError(t, err)

	err = store.SetPhase(types.KURLMigrationPhasePreparation)
	require.NoError(t, err)

	status, err := store.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, types.KURLMigrationPhasePreparation, status.Phase)

	// Verify persistence
	store2 := NewFileStore(tmpDir)
	status2, err := store2.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, types.KURLMigrationPhasePreparation, status2.Phase)
}

func TestFileStore_SetMessage(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	config := types.LinuxInstallationConfig{}
	err := store.InitializeMigration("test-id", "copy", config)
	require.NoError(t, err)

	err = store.SetMessage("Preparing migration")
	require.NoError(t, err)

	status, err := store.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, "Preparing migration", status.Message)
}

func TestFileStore_SetProgress(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	config := types.LinuxInstallationConfig{}
	err := store.InitializeMigration("test-id", "copy", config)
	require.NoError(t, err)

	err = store.SetProgress(50)
	require.NoError(t, err)

	status, err := store.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, 50, status.Progress)
}

func TestFileStore_SetError(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	config := types.LinuxInstallationConfig{}
	err := store.InitializeMigration("test-id", "copy", config)
	require.NoError(t, err)

	err = store.SetError("kURL migration failed")
	require.NoError(t, err)

	status, err := store.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, "kURL migration failed", status.Error)
}

func TestFileStore_GetConfig(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(Store)
		wantErr  error
		validate func(*testing.T, types.LinuxInstallationConfig)
	}{
		{
			name: "success - returns deep copy preventing mutation",
			setup: func(s Store) {
				cfg := types.LinuxInstallationConfig{
					DataDirectory: "/var/lib/ec",
					PodCIDR:       "10.32.0.0/20",
				}
				_ = s.InitializeMigration("test-id", "copy", cfg)
			},
			wantErr: nil,
			validate: func(t *testing.T, cfg types.LinuxInstallationConfig) {
				assert.Equal(t, "/var/lib/ec", cfg.DataDirectory)
				assert.Equal(t, "10.32.0.0/20", cfg.PodCIDR)

				// Mutate returned config
				cfg.DataDirectory = "/tmp/modified"
				cfg.PodCIDR = "192.168.0.0/16"

				// Verify mutations don't affect store
				tmpDir := t.TempDir()
				store2 := NewFileStore(tmpDir)
				_ = store2.InitializeMigration("test", "copy", types.LinuxInstallationConfig{
					DataDirectory: "/var/lib/ec",
					PodCIDR:       "10.32.0.0/20",
				})
				cfg2, _ := store2.GetConfig()
				assert.Equal(t, "/var/lib/ec", cfg2.DataDirectory)
				assert.Equal(t, "10.32.0.0/20", cfg2.PodCIDR)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)

			if tt.setup != nil {
				tt.setup(store)
			}

			cfg, err := store.GetConfig()

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)

			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestFileStore_GetUserConfig(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(Store)
		wantErr  error
		validate func(*testing.T, types.LinuxInstallationConfig)
	}{
		{
			name:    "success - returns empty when no migration",
			wantErr: nil,
			validate: func(t *testing.T, cfg types.LinuxInstallationConfig) {
				assert.Equal(t, types.LinuxInstallationConfig{}, cfg)
			},
		},
		{
			name: "success - returns pending user config before initialization",
			setup: func(s Store) {
				_ = s.SetUserConfig(types.LinuxInstallationConfig{
					HTTPProxy: "http://proxy.example.com:8080",
				})
			},
			wantErr: nil,
			validate: func(t *testing.T, cfg types.LinuxInstallationConfig) {
				assert.Equal(t, "http://proxy.example.com:8080", cfg.HTTPProxy)
			},
		},
		{
			name: "success - returns persisted user config after initialization",
			setup: func(s Store) {
				_ = s.SetUserConfig(types.LinuxInstallationConfig{
					HTTPProxy: "http://proxy.example.com:8080",
				})
				_ = s.InitializeMigration("test-id", "copy", types.LinuxInstallationConfig{})
			},
			wantErr: nil,
			validate: func(t *testing.T, cfg types.LinuxInstallationConfig) {
				assert.Equal(t, "http://proxy.example.com:8080", cfg.HTTPProxy)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)

			if tt.setup != nil {
				tt.setup(store)
			}

			cfg, err := store.GetUserConfig()

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)

			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestFileStore_SetUserConfig(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	// Set user config before initialization
	userCfg := types.LinuxInstallationConfig{
		HTTPProxy: "http://proxy.example.com:8080",
	}
	err := store.SetUserConfig(userCfg)
	require.NoError(t, err)

	// Verify it's stored in memory temporarily
	retrievedCfg, err := store.GetUserConfig()
	require.NoError(t, err)
	assert.Equal(t, userCfg.HTTPProxy, retrievedCfg.HTTPProxy)

	// Initialize migration
	err = store.InitializeMigration("test-id", "copy", types.LinuxInstallationConfig{})
	require.NoError(t, err)

	// Verify user config was persisted to file
	store2 := NewFileStore(tmpDir)
	retrievedCfg2, err := store2.GetUserConfig()
	require.NoError(t, err)
	assert.Equal(t, userCfg.HTTPProxy, retrievedCfg2.HTTPProxy)
}

func TestFileStore_GetStatus(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(Store)
		wantErr  error
		validate func(*testing.T, types.KURLMigrationStatusResponse)
	}{
		{
			name: "success - returns deep copy preventing mutation",
			setup: func(s Store) {
				_ = s.InitializeMigration("test-id", "copy", types.LinuxInstallationConfig{})
				_ = s.SetMessage("Original message")
				_ = s.SetProgress(75)
			},
			wantErr: nil,
			validate: func(t *testing.T, status types.KURLMigrationStatusResponse) {
				assert.Equal(t, "Original message", status.Message)
				assert.Equal(t, 75, status.Progress)

				// Mutate returned status
				status.Message = "Modified message"
				status.Progress = 100

				// Verify mutations don't affect store (test in separate instance)
				tmpDir := t.TempDir()
				store2 := NewFileStore(tmpDir)
				_ = store2.InitializeMigration("test", "copy", types.LinuxInstallationConfig{})
				_ = store2.SetMessage("Original message")
				_ = store2.SetProgress(75)
				status2, _ := store2.GetStatus()
				assert.Equal(t, "Original message", status2.Message)
				assert.Equal(t, 75, status2.Progress)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store := NewFileStore(tmpDir)

			if tt.setup != nil {
				tt.setup(store)
			}

			status, err := store.GetStatus()

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)

			if tt.validate != nil {
				tt.validate(t, status)
			}
		})
	}
}

func TestFileStore_PersistenceAcrossInstances(t *testing.T) {
	tmpDir := t.TempDir()

	// Create migration in first instance
	store1 := NewFileStore(tmpDir)
	config := types.LinuxInstallationConfig{
		DataDirectory: "/var/lib/embedded-cluster",
	}
	err := store1.InitializeMigration("test-migration-id", "copy", config)
	require.NoError(t, err)

	err = store1.SetState(types.KURLMigrationStateInProgress)
	require.NoError(t, err)

	err = store1.SetPhase(types.KURLMigrationPhaseDataTransfer)
	require.NoError(t, err)

	// Create second instance and verify data persists
	store2 := NewFileStore(tmpDir)

	id, err := store2.GetMigrationID()
	require.NoError(t, err)
	assert.Equal(t, "test-migration-id", id)

	status, err := store2.GetStatus()
	require.NoError(t, err)
	assert.Equal(t, types.KURLMigrationStateInProgress, status.State)
	assert.Equal(t, types.KURLMigrationPhaseDataTransfer, status.Phase)
}

func TestFileStore_AtomicWriteWithTempFile(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	config := types.LinuxInstallationConfig{}
	err := store.InitializeMigration("test-id", "copy", config)
	require.NoError(t, err)

	// Verify temp file doesn't exist after successful write
	tempPath := filepath.Join(tmpDir, "migration-state.json.tmp")
	_, err = os.Stat(tempPath)
	assert.True(t, os.IsNotExist(err), "temp file should be cleaned up")

	// Verify final file exists
	finalPath := filepath.Join(tmpDir, "migration-state.json")
	_, err = os.Stat(finalPath)
	assert.NoError(t, err)
}

func TestFileStore_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	config := types.LinuxInstallationConfig{}
	err := store.InitializeMigration("test-id", "copy", config)
	require.NoError(t, err)

	statePath := filepath.Join(tmpDir, "migration-state.json")
	info, err := os.Stat(statePath)
	require.NoError(t, err)

	// Verify file permissions are 0644
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
}

func TestFileStore_JSONFormatting(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	config := types.LinuxInstallationConfig{
		DataDirectory: "/var/lib/embedded-cluster",
	}
	err := store.InitializeMigration("test-id", "copy", config)
	require.NoError(t, err)

	// Read file and verify it's valid, pretty-printed JSON
	statePath := filepath.Join(tmpDir, "migration-state.json")
	data, err := os.ReadFile(statePath)
	require.NoError(t, err)

	// Verify it's valid JSON
	var jsonData map[string]interface{}
	err = json.Unmarshal(data, &jsonData)
	require.NoError(t, err)

	// Verify pretty-printing (should contain newlines and indentation)
	assert.Contains(t, string(data), "\n")
	assert.Contains(t, string(data), "  ")
}

func TestFileStore_CorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Write corrupted JSON file
	statePath := filepath.Join(tmpDir, "migration-state.json")
	err := os.WriteFile(statePath, []byte("corrupted json {{{"), 0644)
	require.NoError(t, err)

	// Try to read from store
	store := NewFileStore(tmpDir)
	_, err = store.GetMigrationID()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal migration state")
}

func TestFileStore_ConcurrentReads(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	config := types.LinuxInstallationConfig{}
	err := store.InitializeMigration("test-id", "copy", config)
	require.NoError(t, err)

	// Perform concurrent reads
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.GetStatus()
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
}

func TestFileStore_ConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileStore(tmpDir)

	config := types.LinuxInstallationConfig{}
	err := store.InitializeMigration("test-id", "copy", config)
	require.NoError(t, err)

	// Perform concurrent writes
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(progress int) {
			defer wg.Done()
			err := store.SetProgress(progress)
			assert.NoError(t, err)
		}(i * 10)
	}
	wg.Wait()

	// Verify final state is valid
	status, err := store.GetStatus()
	require.NoError(t, err)
	assert.GreaterOrEqual(t, status.Progress, 0)
	assert.LessOrEqual(t, status.Progress, 100)
}
