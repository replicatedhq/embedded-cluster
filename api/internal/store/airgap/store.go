package airgap

import (
	"fmt"
	"sync"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/tiendc/go-deepcopy"
)

const maxLogSize = 100 * 1024 // 100KB total log size

var _ Store = &memoryStore{}

// Store provides methods for storing and retrieving airgap processing state
type Store interface {
	Get() (types.Airgap, error)
	GetStatus() (types.Status, error)
	SetStatus(status types.Status) error
	SetStatusDesc(desc string) error
	AddLogs(logs string) error
	GetLogs() (string, error)
}

// memoryStore is an in-memory implementation of Store
type memoryStore struct {
	airgap types.Airgap
	mu     sync.RWMutex
}

type StoreOption func(*memoryStore)

func WithAirgap(airgap types.Airgap) StoreOption {
	return func(s *memoryStore) {
		s.airgap = airgap
	}
}

// NewMemoryStore creates a new memory store
func NewMemoryStore(opts ...StoreOption) Store {
	s := &memoryStore{
		airgap: types.Airgap{
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

func (s *memoryStore) Get() (types.Airgap, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var airgap types.Airgap
	if err := deepcopy.Copy(&airgap, &s.airgap); err != nil {
		return types.Airgap{}, err
	}

	return airgap, nil
}

func (s *memoryStore) GetStatus() (types.Status, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var status types.Status
	if err := deepcopy.Copy(&status, &s.airgap.Status); err != nil {
		return types.Status{}, err
	}

	return status, nil
}

func (s *memoryStore) SetStatus(status types.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.airgap.Status = status
	return nil
}

func (s *memoryStore) SetStatusDesc(desc string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.airgap.Status.State == "" {
		return fmt.Errorf("state not set")
	}

	s.airgap.Status.Description = desc
	return nil
}

func (s *memoryStore) AddLogs(logs string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.airgap.Logs += logs + "\n"
	if len(s.airgap.Logs) > maxLogSize {
		s.airgap.Logs = "... (truncated) " + s.airgap.Logs[len(s.airgap.Logs)-maxLogSize:]
	}

	return nil
}

func (s *memoryStore) GetLogs() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.airgap.Logs, nil
}
