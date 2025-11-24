// Package kurlmigration provides a store implementation for managing kURL to Embedded Cluster migration state.
package kurlmigration

import (
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/tiendc/go-deepcopy"
)

var _ Store = &memoryStore{}

// Store provides methods for storing and retrieving migration state
type Store interface {
	// InitializeMigration sets up a new migration with ID, transfer mode, and config
	InitializeMigration(migrationID string, transferMode string, config types.LinuxInstallationConfig) error

	// GetMigrationID returns the current migration ID, or error if none exists
	GetMigrationID() (string, error)

	// GetStatus returns the current migration status
	GetStatus() (types.KURLMigrationStatusResponse, error)

	// SetState updates the migration state
	SetState(state types.KURLMigrationState) error

	// SetPhase updates the migration phase
	SetPhase(phase types.KURLMigrationPhase) error

	// SetMessage updates the status message
	SetMessage(message string) error

	// SetProgress updates the progress percentage
	SetProgress(progress int) error

	// SetError updates the error message
	SetError(err string) error

	// GetTransferMode returns the transfer mode
	GetTransferMode() (string, error)

	// GetConfig returns the resolved installation config
	GetConfig() (types.LinuxInstallationConfig, error)

	// GetUserConfig returns the user-provided config (may be empty if not yet submitted)
	GetUserConfig() (types.LinuxInstallationConfig, error)

	// SetUserConfig sets the user-provided config
	SetUserConfig(config types.LinuxInstallationConfig) error
}

// memoryStore is an in-memory implementation of Store
type memoryStore struct {
	migrationID  string
	transferMode string
	config       types.LinuxInstallationConfig // resolved/merged config
	userConfig   types.LinuxInstallationConfig // user-provided config
	status       types.KURLMigrationStatusResponse
	initialized  bool
	mu           sync.RWMutex
}

type StoreOption func(*memoryStore)

// WithMigrationID sets the migration ID
func WithMigrationID(id string) StoreOption {
	return func(s *memoryStore) {
		s.migrationID = id
		s.initialized = true
	}
}

// WithTransferMode sets the transfer mode
func WithTransferMode(mode string) StoreOption {
	return func(s *memoryStore) {
		s.transferMode = mode
	}
}

// WithConfig sets the installation config
func WithConfig(config types.LinuxInstallationConfig) StoreOption {
	return func(s *memoryStore) {
		s.config = config
	}
}

// WithStatus sets the migration status
func WithStatus(status types.KURLMigrationStatusResponse) StoreOption {
	return func(s *memoryStore) {
		s.status = status
	}
}

// NewMemoryStore creates a new memory store
func NewMemoryStore(opts ...StoreOption) Store {
	s := &memoryStore{
		status: types.KURLMigrationStatusResponse{
			State:    types.KURLMigrationStateNotStarted,
			Phase:    types.KURLMigrationPhaseDiscovery,
			Message:  "",
			Progress: 0,
			Error:    "",
		},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *memoryStore) InitializeMigration(migrationID string, transferMode string, config types.LinuxInstallationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.initialized {
		return types.ErrKURLMigrationAlreadyStarted
	}

	s.migrationID = migrationID
	s.transferMode = transferMode
	s.config = config
	s.initialized = true

	s.status = types.KURLMigrationStatusResponse{
		State:    types.KURLMigrationStateNotStarted,
		Phase:    types.KURLMigrationPhaseDiscovery,
		Message:  "",
		Progress: 0,
		Error:    "",
	}

	return nil
}

func (s *memoryStore) GetMigrationID() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return "", types.ErrNoActiveKURLMigration
	}

	return s.migrationID, nil
}

func (s *memoryStore) GetStatus() (types.KURLMigrationStatusResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return types.KURLMigrationStatusResponse{}, types.ErrNoActiveKURLMigration
	}

	var status types.KURLMigrationStatusResponse
	if err := deepcopy.Copy(&status, &s.status); err != nil {
		return types.KURLMigrationStatusResponse{}, err
	}

	return status, nil
}

func (s *memoryStore) SetState(state types.KURLMigrationState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return types.ErrNoActiveKURLMigration
	}

	s.status.State = state
	return nil
}

func (s *memoryStore) SetPhase(phase types.KURLMigrationPhase) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return types.ErrNoActiveKURLMigration
	}

	s.status.Phase = phase
	return nil
}

func (s *memoryStore) SetMessage(message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return types.ErrNoActiveKURLMigration
	}

	s.status.Message = message
	return nil
}

func (s *memoryStore) SetProgress(progress int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return types.ErrNoActiveKURLMigration
	}

	s.status.Progress = progress
	return nil
}

func (s *memoryStore) SetError(err string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return types.ErrNoActiveKURLMigration
	}

	s.status.Error = err
	return nil
}

func (s *memoryStore) GetTransferMode() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return "", types.ErrNoActiveKURLMigration
	}

	return s.transferMode, nil
}

func (s *memoryStore) GetConfig() (types.LinuxInstallationConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return types.LinuxInstallationConfig{}, types.ErrNoActiveKURLMigration
	}

	var config types.LinuxInstallationConfig
	if err := deepcopy.Copy(&config, &s.config); err != nil {
		return types.LinuxInstallationConfig{}, err
	}

	return config, nil
}

func (s *memoryStore) GetUserConfig() (types.LinuxInstallationConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return user config even if migration not initialized (for GET /config endpoint)
	// If not initialized, userConfig will be zero-value (empty)
	var config types.LinuxInstallationConfig
	if err := deepcopy.Copy(&config, &s.userConfig); err != nil {
		return types.LinuxInstallationConfig{}, err
	}

	return config, nil
}

func (s *memoryStore) SetUserConfig(config types.LinuxInstallationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := deepcopy.Copy(&s.userConfig, &config); err != nil {
		return err
	}

	return nil
}
