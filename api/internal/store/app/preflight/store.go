package preflight

import (
	"sync"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/tiendc/go-deepcopy"
)

var _ Store = &memoryStore{}

type Store interface {
	GetTitles() ([]string, error)
	SetTitles(titles []string) error
	GetOutput() (*types.PreflightsOutput, error)
	SetOutput(output *types.PreflightsOutput) error
	GetStatus() (types.Status, error)
	SetStatus(status types.Status) error
	Clear() error
}

type memoryStore struct {
	mu           sync.RWMutex
	appPreflight types.AppPreflights
}

type StoreOption func(*memoryStore)

func WithAppPreflight(appPreflight types.AppPreflights) StoreOption {
	return func(s *memoryStore) {
		s.appPreflight = appPreflight
	}
}

func NewMemoryStore(opts ...StoreOption) *memoryStore {
	s := &memoryStore{
		appPreflight: types.AppPreflights{
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

func (s *memoryStore) GetTitles() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var titles []string
	if err := deepcopy.Copy(&titles, &s.appPreflight.Titles); err != nil {
		return nil, err
	}

	return titles, nil
}

func (s *memoryStore) SetTitles(titles []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appPreflight.Titles = titles

	return nil
}

func (s *memoryStore) GetOutput() (*types.PreflightsOutput, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.appPreflight.Output == nil {
		return nil, nil
	}

	var output *types.PreflightsOutput
	if err := deepcopy.Copy(&output, &s.appPreflight.Output); err != nil {
		return nil, err
	}

	return output, nil
}

func (s *memoryStore) SetOutput(output *types.PreflightsOutput) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.appPreflight.Output = output
	return nil
}

func (s *memoryStore) GetStatus() (types.Status, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var status types.Status
	if err := deepcopy.Copy(&status, &s.appPreflight.Status); err != nil {
		return types.Status{}, err
	}

	return status, nil
}

func (s *memoryStore) SetStatus(status types.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.appPreflight.Status = status
	return nil
}

func (s *memoryStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.appPreflight = types.AppPreflights{}
	return nil
}
