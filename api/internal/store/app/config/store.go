package config

import (
	"sync"

	"github.com/tiendc/go-deepcopy"
)

var _ Store = &memoryStore{}

type Store interface {
	GetConfigValues() (map[string]string, error)
	SetConfigValues(configValues map[string]string) error
}

type memoryStore struct {
	mu           sync.RWMutex
	configValues map[string]string
}

type StoreOption func(*memoryStore)

func WithConfigValues(configValues map[string]string) StoreOption {
	return func(s *memoryStore) {
		s.configValues = configValues
	}
}

func NewMemoryStore(opts ...StoreOption) Store {
	s := &memoryStore{
		configValues: map[string]string{},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *memoryStore) GetConfigValues() (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var configValues map[string]string
	if err := deepcopy.Copy(&configValues, &s.configValues); err != nil {
		return nil, err
	}

	return configValues, nil
}

func (s *memoryStore) SetConfigValues(configValues map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.configValues = configValues
	return nil
}
