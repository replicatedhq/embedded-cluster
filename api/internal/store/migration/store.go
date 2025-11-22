// Package migration provides a store implementation for managing kURL to Embedded Cluster migration state.
package migration

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
	GetStatus() (types.MigrationStatusResponse, error)

	// SetState updates the migration state
	SetState(state types.MigrationState) error

	// SetPhase updates the migration phase
	SetPhase(phase types.MigrationPhase) error

	// SetMessage updates the status message
	SetMessage(message string) error

	// SetProgress updates the progress percentage
	SetProgress(progress int) error

	// SetError updates the error message
	SetError(err string) error

	// GetTransferMode returns the transfer mode
	GetTransferMode() (string, error)

	// GetConfig returns the installation config
	GetConfig() (types.LinuxInstallationConfig, error)
}

// memoryStore is an in-memory implementation of Store
type memoryStore struct {
	migrationID  string
	transferMode string
	config       types.LinuxInstallationConfig
	status       types.MigrationStatusResponse
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
func WithStatus(status types.MigrationStatusResponse) StoreOption {
	return func(s *memoryStore) {
		s.status = status
	}
}

// NewMemoryStore creates a new memory store
func NewMemoryStore(opts ...StoreOption) Store {
	s := &memoryStore{
		status: types.MigrationStatusResponse{
			State:    types.MigrationStateNotStarted,
			Phase:    types.MigrationPhaseDiscovery,
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
		return types.ErrMigrationAlreadyStarted
	}

	s.migrationID = migrationID
	s.transferMode = transferMode
	s.config = config
	s.initialized = true

	s.status = types.MigrationStatusResponse{
		State:    types.MigrationStateNotStarted,
		Phase:    types.MigrationPhaseDiscovery,
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
		return "", types.ErrNoActiveMigration
	}

	return s.migrationID, nil
}

func (s *memoryStore) GetStatus() (types.MigrationStatusResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return types.MigrationStatusResponse{}, types.ErrNoActiveMigration
	}

	var status types.MigrationStatusResponse
	if err := deepcopy.Copy(&status, &s.status); err != nil {
		return types.MigrationStatusResponse{}, err
	}

	return status, nil
}

func (s *memoryStore) SetState(state types.MigrationState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return types.ErrNoActiveMigration
	}

	s.status.State = state
	return nil
}

func (s *memoryStore) SetPhase(phase types.MigrationPhase) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return types.ErrNoActiveMigration
	}

	s.status.Phase = phase
	return nil
}

func (s *memoryStore) SetMessage(message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return types.ErrNoActiveMigration
	}

	s.status.Message = message
	return nil
}

func (s *memoryStore) SetProgress(progress int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return types.ErrNoActiveMigration
	}

	s.status.Progress = progress
	return nil
}

func (s *memoryStore) SetError(err string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return types.ErrNoActiveMigration
	}

	s.status.Error = err
	return nil
}

func (s *memoryStore) GetTransferMode() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return "", types.ErrNoActiveMigration
	}

	return s.transferMode, nil
}

func (s *memoryStore) GetConfig() (types.LinuxInstallationConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.initialized {
		return types.LinuxInstallationConfig{}, types.ErrNoActiveMigration
	}

	var config types.LinuxInstallationConfig
	if err := deepcopy.Copy(&config, &s.config); err != nil {
		return types.LinuxInstallationConfig{}, err
	}

	return config, nil
}
