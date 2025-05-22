package installation

import (
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

type ConfigStore interface {
	Read() (*types.InstallationConfig, error)
	Write(cfg types.InstallationConfig) error
}

var _ ConfigStore = &ConfigMemoryStore{}

type ConfigMemoryStore struct {
	mu  sync.RWMutex
	cfg *types.InstallationConfig
}

func NewConfigMemoryStore() *ConfigMemoryStore {
	return &ConfigMemoryStore{
		cfg: &types.InstallationConfig{},
	}
}

func (s *ConfigMemoryStore) Read() (*types.InstallationConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.cfg, nil
}

func (s *ConfigMemoryStore) Write(cfg types.InstallationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = &cfg

	return nil
}
