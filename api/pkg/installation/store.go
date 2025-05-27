package installation

import (
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

type InstallationStore interface {
	ReadConfig() (*types.InstallationConfig, error)
	WriteConfig(cfg types.InstallationConfig) error
	ReadStatus() (*types.InstallationStatus, error)
	WriteStatus(status types.InstallationStatus) error
}

var _ InstallationStore = &MemoryStore{}

type MemoryStore struct {
	mu     sync.RWMutex
	config *types.InstallationConfig
	status *types.InstallationStatus
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		config: &types.InstallationConfig{},
		status: &types.InstallationStatus{},
	}
}

func (s *MemoryStore) ReadConfig() (*types.InstallationConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config, nil
}

func (s *MemoryStore) WriteConfig(cfg types.InstallationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = &cfg

	return nil
}

func (s *MemoryStore) ReadStatus() (*types.InstallationStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.status, nil
}

func (s *MemoryStore) WriteStatus(status types.InstallationStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = &status

	return nil
}
