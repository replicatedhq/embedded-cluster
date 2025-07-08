package config

import (
	"sync"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/tiendc/go-deepcopy"
)

var _ Store = &memoryStore{}

type Store interface {
	GetConfigValues() (kotsv1beta1.ConfigValues, error)
	SetConfigValues(configValues kotsv1beta1.ConfigValues) error
}

type memoryStore struct {
	mu           sync.RWMutex
	configValues kotsv1beta1.ConfigValues
}

type StoreOption func(*memoryStore)

func WithConfigValues(configValues kotsv1beta1.ConfigValues) StoreOption {
	return func(s *memoryStore) {
		s.configValues = configValues
	}
}

func NewMemoryStore(opts ...StoreOption) Store {
	s := &memoryStore{}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *memoryStore) GetConfigValues() (kotsv1beta1.ConfigValues, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var configValues kotsv1beta1.ConfigValues
	if err := deepcopy.Copy(&configValues, &s.configValues); err != nil {
		return kotsv1beta1.ConfigValues{}, err
	}

	return configValues, nil
}

func (s *memoryStore) SetConfigValues(configValues kotsv1beta1.ConfigValues) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.configValues = configValues
	return nil
}
