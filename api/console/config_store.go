package console

import "sync"

type configStore interface {
	read() (*Config, error)
	write(cfg *Config) error
}

var _ configStore = &configMemoryStore{}

type configMemoryStore struct {
	mu  sync.RWMutex
	cfg *Config
}

func (s *configMemoryStore) read() (*Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.cfg, nil
}

func (s *configMemoryStore) write(cfg *Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg

	return nil
}
