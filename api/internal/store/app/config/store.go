package config

import (
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/tiendc/go-deepcopy"
)

var _ Store = &memoryStore{}

type Store interface {
	GetConfigValues() (types.AppConfigValues, error)
	SetConfigValues(configValues types.AppConfigValues) error
}

type memoryStore struct {
	mu           sync.RWMutex
	configValues types.AppConfigValues
}

type StoreOption func(*memoryStore)

func WithConfigValues(configValues types.AppConfigValues) StoreOption {
	return func(s *memoryStore) {
		s.configValues = configValues
	}
}

func NewMemoryStore(opts ...StoreOption) Store {
	s := &memoryStore{
		configValues: types.AppConfigValues{},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *memoryStore) GetConfigValues() (types.AppConfigValues, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var configValues types.AppConfigValues
	if err := deepcopy.Copy(&configValues, &s.configValues); err != nil {
		return nil, err
	}

	return configValues, nil
}

func (s *memoryStore) SetConfigValues(configValues types.AppConfigValues) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.configValues = configValues
	return nil
}
