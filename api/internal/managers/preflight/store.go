package preflight

import (
	"sync"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

type HostPreflightStore interface {
	ReadTitles() ([]string, error)
	WriteTitles(titles []string) error
	ReadOutput() (*types.HostPreflightOutput, error)
	WriteOutput(output *types.HostPreflightOutput) error
	ReadStatus() (*types.Status, error)
	WriteStatus(status *types.Status) error
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

func (s *MemoryStore) ReadTitles() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.hostPreflight.Titles, nil
}

func (s *MemoryStore) WriteTitles(titles []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostPreflight.Titles = titles

	return nil
}

func (s *MemoryStore) ReadOutput() (*types.HostPreflightOutput, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.hostPreflight.Output, nil
}

func (s *MemoryStore) WriteOutput(output *types.HostPreflightOutput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostPreflight.Output = output

	return nil
}

func (s *MemoryStore) ReadStatus() (*types.Status, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.hostPreflight.Status, nil
}

func (s *MemoryStore) WriteStatus(status *types.Status) error {
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
