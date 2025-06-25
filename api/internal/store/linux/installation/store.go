package installation

import (
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/tiendc/go-deepcopy"
)

var _ Store = &memoryStore{}

type Store interface {
	GetConfig() (types.LinuxInstallationConfig, error)
	SetConfig(cfg types.LinuxInstallationConfig) error
	GetStatus() (types.Status, error)
	SetStatus(status types.Status) error
}

type memoryStore struct {
	mu           sync.RWMutex
	installation types.LinuxInstallation
}

type StoreOption func(*memoryStore)

func WithInstallation(installation types.LinuxInstallation) StoreOption {
	return func(s *memoryStore) {
		s.installation = installation
	}
}

func NewMemoryStore(opts ...StoreOption) *memoryStore {
	s := &memoryStore{}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *memoryStore) GetConfig() (types.LinuxInstallationConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var config types.LinuxInstallationConfig
	if err := deepcopy.Copy(&config, &s.installation.Config); err != nil {
		return types.LinuxInstallationConfig{}, err
	}

	return config, nil
}

func (s *memoryStore) SetConfig(cfg types.LinuxInstallationConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.installation.Config = cfg
	return nil
}

func (s *memoryStore) GetStatus() (types.Status, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var status types.Status
	if err := deepcopy.Copy(&status, &s.installation.Status); err != nil {
		return types.Status{}, err
	}

	return status, nil
}

func (s *memoryStore) SetStatus(status types.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.installation.Status = status

	return nil
}
