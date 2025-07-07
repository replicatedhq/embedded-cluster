package config

import (
	"sync"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/tiendc/go-deepcopy"
)

var _ Store = &memoryStore{}

type Store interface {
	Get() (kotsv1beta1.Config, error)
	Set(config kotsv1beta1.Config) error
}

type memoryStore struct {
	mu     sync.RWMutex
	config kotsv1beta1.Config
}

type StoreOption func(*memoryStore)

func WithConfig(config kotsv1beta1.Config) StoreOption {
	return func(s *memoryStore) {
		s.config = config
	}
}

func NewMemoryStore(opts ...StoreOption) Store {
	s := &memoryStore{}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *memoryStore) Get() (kotsv1beta1.Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var config kotsv1beta1.Config
	if err := deepcopy.Copy(&config, &s.config); err != nil {
		return kotsv1beta1.Config{}, err
	}

	return config, nil
}

func (s *memoryStore) Set(config kotsv1beta1.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config
	return nil
}
