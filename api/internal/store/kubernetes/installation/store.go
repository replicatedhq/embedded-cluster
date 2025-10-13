package installation

import (
	"sync"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/tiendc/go-deepcopy"
)

var _ Store = &memoryStore{}

type Store interface {
	GetConfig() (types.KubernetesInstallationConfig, error)
	SetConfig(cfg types.KubernetesInstallationConfig) error
	GetStatus() (types.Status, error)
	SetStatus(status types.Status) error
}

type memoryStore struct {
	mu           sync.RWMutex
	installation types.KubernetesInstallation
}

type StoreOption func(*memoryStore)

func WithInstallation(installation types.KubernetesInstallation) StoreOption {
	return func(s *memoryStore) {
		s.installation = installation
	}
}

func NewMemoryStore(opts ...StoreOption) *memoryStore {
	s := &memoryStore{
		installation: types.KubernetesInstallation{
			Status: types.Status{
				State:       types.StatePending,
				LastUpdated: time.Now(),
			},
		},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *memoryStore) GetConfig() (types.KubernetesInstallationConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var config types.KubernetesInstallationConfig
	if err := deepcopy.Copy(&config, &s.installation.Config); err != nil {
		return types.KubernetesInstallationConfig{}, err
	}

	return config, nil
}

func (s *memoryStore) SetConfig(cfg types.KubernetesInstallationConfig) error {
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
