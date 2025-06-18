package installation

import (
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/tiendc/go-deepcopy"
)

var _ Store = &MemoryStore{}

type Store interface {
	GetConfig() (types.InstallationConfig, error)
	SetConfig(cfg types.InstallationConfig) error
	GetStatus() (types.Status, error)
	SetStatus(status types.Status) error
}

type MemoryStore struct {
	mu           sync.RWMutex
	installation types.Installation
}

type StoreOption func(*MemoryStore)

func WithInstallation(installation types.Installation) StoreOption {
	return func(s *MemoryStore) {
		s.installation = installation
	}
}

func NewMemoryStore(opts ...StoreOption) *MemoryStore {
	s := &MemoryStore{}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *MemoryStore) GetConfig() (types.InstallationConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var config types.InstallationConfig
	if err := deepcopy.Copy(&config, &s.installation.Config); err != nil {
		return types.InstallationConfig{}, err
	}

	return config, nil
}

func (s *MemoryStore) SetConfig(cfg types.InstallationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.installation.Config = cfg
	return nil
}

func (s *MemoryStore) GetStatus() (types.Status, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var status types.Status
	if err := deepcopy.Copy(&status, &s.installation.Status); err != nil {
		return types.Status{}, err
	}

	return status, nil
}

func (s *MemoryStore) SetStatus(status types.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.installation.Status = status

	return nil
}
