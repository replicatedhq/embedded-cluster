package preflight

import (
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/tiendc/go-deepcopy"
)

var _ Store = &memoryStore{}

type Store interface {
	GetTitles() ([]string, error)
	SetTitles(titles []string) error
	GetOutput() (*types.HostPreflightsOutput, error)
	SetOutput(output *types.HostPreflightsOutput) error
	GetStatus() (types.Status, error)
	SetStatus(status types.Status) error
}

type memoryStore struct {
	mu            sync.RWMutex
	hostPreflight types.HostPreflights
}

type StoreOption func(*memoryStore)

func WithHostPreflight(hostPreflight types.HostPreflights) StoreOption {
	return func(s *memoryStore) {
		s.hostPreflight = hostPreflight
	}
}

func NewMemoryStore(opts ...StoreOption) *memoryStore {
	s := &memoryStore{}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *memoryStore) GetTitles() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var titles []string
	if err := deepcopy.Copy(&titles, &s.hostPreflight.Titles); err != nil {
		return nil, err
	}

	return titles, nil
}

func (s *memoryStore) SetTitles(titles []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostPreflight.Titles = titles

	return nil
}

func (s *memoryStore) GetOutput() (*types.HostPreflightsOutput, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.hostPreflight.Output == nil {
		return nil, nil
	}

	var output *types.HostPreflightsOutput
	if err := deepcopy.Copy(&output, &s.hostPreflight.Output); err != nil {
		return nil, err
	}

	return output, nil
}

func (s *memoryStore) SetOutput(output *types.HostPreflightsOutput) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.hostPreflight.Output = output
	return nil
}

func (s *memoryStore) GetStatus() (types.Status, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var status types.Status
	if err := deepcopy.Copy(&status, &s.hostPreflight.Status); err != nil {
		return types.Status{}, err
	}

	return status, nil
}

func (s *memoryStore) SetStatus(status types.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.hostPreflight.Status = status
	return nil
}
