package preflight

import (
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

type HostPreflightStore interface {
	GetTitles() ([]string, error)
	SetTitles(titles []string) error
	GetOutput() (*types.HostPreflightOutput, error)
	SetOutput(output *types.HostPreflightOutput) error
	GetStatus() (*types.Status, error)
	SetStatus(status *types.Status) error
	IsRunning() bool
}

var _ HostPreflightStore = &MemoryStore{}

type MemoryStore struct {
	mu            sync.RWMutex
	hostPreflight *types.HostPreflight
}

func NewMemoryStore(hostPreflight *types.HostPreflight) *MemoryStore {
	return &MemoryStore{
		hostPreflight: hostPreflight,
	}
}

func (s *MemoryStore) GetTitles() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.hostPreflight.Titles, nil
}

func (s *MemoryStore) SetTitles(titles []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostPreflight.Titles = titles

	return nil
}

func (s *MemoryStore) GetOutput() (*types.HostPreflightOutput, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.hostPreflight.Output, nil
}

func (s *MemoryStore) SetOutput(output *types.HostPreflightOutput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostPreflight.Output = output

	return nil
}

func (s *MemoryStore) GetStatus() (*types.Status, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.hostPreflight.Status, nil
}

func (s *MemoryStore) SetStatus(status *types.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostPreflight.Status = status

	return nil
}

func (s *MemoryStore) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.hostPreflight.Status.State == types.StateRunning
}
