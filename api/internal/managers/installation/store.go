package installation

import (
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

type InstallationStore interface {
	ReadConfig() (*types.InstallationConfig, error)
	WriteConfig(cfg types.InstallationConfig) error
	ReadStatus() (*types.Status, error)
	WriteStatus(status types.Status) error
}

var _ InstallationStore = &MemoryStore{}

type MemoryStore struct {
	mu           sync.RWMutex
	installation *types.Installation
}

func NewMemoryStore(installation *types.Installation) *MemoryStore {
	return &MemoryStore{
		installation: installation,
	}
}

func (s *MemoryStore) ReadConfig() (*types.InstallationConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.installation.Config, nil
}

func (s *MemoryStore) WriteConfig(cfg types.InstallationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.installation.Config = &cfg

	return nil
}

func (s *MemoryStore) ReadStatus() (*types.Status, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.installation.Status, nil
}

func (s *MemoryStore) WriteStatus(status types.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.installation.Status = &status

	return nil
}
