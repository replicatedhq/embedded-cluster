// Package kurlmigration provides a store implementation for managing kURL to Embedded Cluster migration state.
//
// This package provides two implementations of the Store interface:
//   - memoryStore: In-memory storage for testing and development
//   - fileStore: File-based persistent storage for production use
//
// The fileStore implementation provides:
//   - Atomic writes using temp file + os.Rename pattern
//   - Thread-safe operations using sync.RWMutex
//   - Data isolation via deep copy
//   - Persistence across process restarts
package kurlmigration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/tiendc/go-deepcopy"
)

var _ Store = &fileStore{}

// fileStore implements Store interface with file-based persistence
type fileStore struct {
	dataDir           string // Base directory (e.g., /var/lib/embedded-cluster)
	mu                sync.RWMutex
	pendingUserConfig *types.LinuxInstallationConfig // Temporary storage for user config set before initialization
}

// persistedState represents the structure saved to migration-state.json
type persistedState struct {
	MigrationID  string                            `json:"migrationId"`
	TransferMode string                            `json:"transferMode"`
	Config       types.LinuxInstallationConfig     `json:"config"`     // resolved/merged config
	UserConfig   types.LinuxInstallationConfig     `json:"userConfig"` // user-provided config
	Status       types.KURLMigrationStatusResponse `json:"status"`
}

// NewFileStore creates a new file-based store
// dataDir is the base directory where migration-state.json will be stored
func NewFileStore(dataDir string) Store {
	return &fileStore{
		dataDir: dataDir,
	}
}

// statePath returns the full path to the migration state file
func (f *fileStore) statePath() string {
	return filepath.Join(f.dataDir, "migration-state.json")
}

// readState reads and unmarshals state from file
// Returns ErrNoActiveKURLMigration if file doesn't exist
func (f *fileStore) readState() (*persistedState, error) {
	data, err := os.ReadFile(f.statePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, types.ErrNoActiveKURLMigration
		}
		return nil, fmt.Errorf("read migration state file: %w", err)
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal migration state: %w", err)
	}

	return &state, nil
}

// writeState writes state to file atomically using temp file + rename pattern
func (f *fileStore) writeState(state *persistedState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal migration state: %w", err)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(f.dataDir, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	// Create temp file in same directory for atomic rename
	tmpPath := f.statePath() + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write temp migration state file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, f.statePath()); err != nil {
		// Clean up temp file on error
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp migration state file: %w", err)
	}

	return nil
}

func (f *fileStore) InitializeMigration(migrationID string, transferMode string, config types.LinuxInstallationConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if file already exists
	if _, err := os.Stat(f.statePath()); err == nil {
		return types.ErrKURLMigrationAlreadyStarted
	} else if !os.IsNotExist(err) {
		// Other filesystem errors (permission denied, I/O errors, etc.)
		return fmt.Errorf("check migration state file: %w", err)
	}

	// Use pending user config if it was set before initialization
	userConfig := types.LinuxInstallationConfig{}
	if f.pendingUserConfig != nil {
		if err := deepcopy.Copy(&userConfig, f.pendingUserConfig); err != nil {
			return fmt.Errorf("copy pending user config: %w", err)
		}
	}

	// Create new state
	state := &persistedState{
		MigrationID:  migrationID,
		TransferMode: transferMode,
		Config:       config,
		UserConfig:   userConfig,
		Status: types.KURLMigrationStatusResponse{
			State:    types.KURLMigrationStateNotStarted,
			Phase:    types.KURLMigrationPhaseDiscovery,
			Message:  "",
			Progress: 0,
			Error:    "",
		},
	}

	if err := f.writeState(state); err != nil {
		return fmt.Errorf("initialize migration: %w", err)
	}

	// Clear pending config only after successful write
	f.pendingUserConfig = nil

	return nil
}

func (f *fileStore) GetMigrationID() (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	state, err := f.readState()
	if err != nil {
		return "", err
	}

	return state.MigrationID, nil
}

func (f *fileStore) GetStatus() (types.KURLMigrationStatusResponse, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	state, err := f.readState()
	if err != nil {
		return types.KURLMigrationStatusResponse{}, err
	}

	var status types.KURLMigrationStatusResponse
	if err := deepcopy.Copy(&status, &state.Status); err != nil {
		return types.KURLMigrationStatusResponse{}, fmt.Errorf("deep copy status: %w", err)
	}

	return status, nil
}

func (f *fileStore) SetState(newState types.KURLMigrationState) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	state, err := f.readState()
	if err != nil {
		return err
	}

	state.Status.State = newState

	if err := f.writeState(state); err != nil {
		return fmt.Errorf("set state: %w", err)
	}

	return nil
}

func (f *fileStore) SetPhase(phase types.KURLMigrationPhase) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	state, err := f.readState()
	if err != nil {
		return err
	}

	state.Status.Phase = phase

	if err := f.writeState(state); err != nil {
		return fmt.Errorf("set phase: %w", err)
	}

	return nil
}

func (f *fileStore) SetMessage(message string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	state, err := f.readState()
	if err != nil {
		return err
	}

	state.Status.Message = message

	if err := f.writeState(state); err != nil {
		return fmt.Errorf("set message: %w", err)
	}

	return nil
}

func (f *fileStore) SetProgress(progress int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	state, err := f.readState()
	if err != nil {
		return err
	}

	state.Status.Progress = progress

	if err := f.writeState(state); err != nil {
		return fmt.Errorf("set progress: %w", err)
	}

	return nil
}

func (f *fileStore) SetError(errMsg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	state, err := f.readState()
	if err != nil {
		return err
	}

	state.Status.Error = errMsg

	if err := f.writeState(state); err != nil {
		return fmt.Errorf("set error: %w", err)
	}

	return nil
}

func (f *fileStore) GetTransferMode() (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	state, err := f.readState()
	if err != nil {
		return "", err
	}

	return state.TransferMode, nil
}

func (f *fileStore) GetConfig() (types.LinuxInstallationConfig, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	state, err := f.readState()
	if err != nil {
		return types.LinuxInstallationConfig{}, err
	}

	var config types.LinuxInstallationConfig
	if err := deepcopy.Copy(&config, &state.Config); err != nil {
		return types.LinuxInstallationConfig{}, fmt.Errorf("deep copy config: %w", err)
	}

	return config, nil
}

func (f *fileStore) GetUserConfig() (types.LinuxInstallationConfig, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	state, err := f.readState()
	if err != nil {
		// Return empty config even if no migration exists (matches memoryStore behavior)
		if err == types.ErrNoActiveKURLMigration {
			// If there's a pending user config, return that instead of empty
			if f.pendingUserConfig != nil {
				var config types.LinuxInstallationConfig
				if err := deepcopy.Copy(&config, f.pendingUserConfig); err != nil {
					return types.LinuxInstallationConfig{}, fmt.Errorf("deep copy pending user config: %w", err)
				}
				return config, nil
			}
			return types.LinuxInstallationConfig{}, nil
		}
		return types.LinuxInstallationConfig{}, err
	}

	var config types.LinuxInstallationConfig
	if err := deepcopy.Copy(&config, &state.UserConfig); err != nil {
		return types.LinuxInstallationConfig{}, fmt.Errorf("deep copy user config: %w", err)
	}

	return config, nil
}

func (f *fileStore) SetUserConfig(config types.LinuxInstallationConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	state, err := f.readState()
	if err != nil {
		// If no migration exists yet, store config in memory temporarily
		// It will be persisted when InitializeMigration is called
		if err == types.ErrNoActiveKURLMigration {
			tempConfig := &types.LinuxInstallationConfig{}
			if err := deepcopy.Copy(tempConfig, &config); err != nil {
				return fmt.Errorf("deep copy pending user config: %w", err)
			}
			// Only assign after successful copy
			f.pendingUserConfig = tempConfig
			return nil
		}
		return err
	}

	if err := deepcopy.Copy(&state.UserConfig, &config); err != nil {
		return fmt.Errorf("deep copy user config: %w", err)
	}

	if err := f.writeState(state); err != nil {
		return fmt.Errorf("set user config: %w", err)
	}

	return nil
}
