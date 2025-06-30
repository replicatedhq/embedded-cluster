package infra

import (
	"fmt"
	"sync"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/tiendc/go-deepcopy"
)

const maxLogSize = 100 * 1024 // 100KB total log size

var _ Store = &memoryStore{}

// Store provides methods for storing and retrieving infrastructure state
type Store interface {
	Get() (types.Infra, error)
	GetStatus() (types.Status, error)
	SetStatus(status types.Status) error
	SetStatusDesc(desc string) error
	RegisterComponent(name string) error
	SetComponentStatus(name string, status types.Status) error
	AddLogs(logs string) error
	GetLogs() (string, error)
}

// memoryStore is an in-memory implementation of Store
type memoryStore struct {
	infra types.Infra
	mu    sync.RWMutex
}

type StoreOption func(*memoryStore)

func WithInfra(infra types.Infra) StoreOption {
	return func(s *memoryStore) {
		s.infra = infra
	}
}

// NewMemoryStore creates a new memory store
func NewMemoryStore(opts ...StoreOption) Store {
	s := &memoryStore{}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *memoryStore) Get() (types.Infra, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var infra types.Infra
	if err := deepcopy.Copy(&infra, &s.infra); err != nil {
		return types.Infra{}, err
	}

	return infra, nil
}

func (s *memoryStore) GetStatus() (types.Status, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var status types.Status
	if err := deepcopy.Copy(&status, &s.infra.Status); err != nil {
		return types.Status{}, err
	}

	return status, nil
}

func (s *memoryStore) SetStatus(status types.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.infra.Status = status
	return nil
}

func (s *memoryStore) SetStatusDesc(desc string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.infra.Status.State == "" {
		return fmt.Errorf("state not set")
	}

	s.infra.Status.Description = desc
	return nil
}

func (s *memoryStore) RegisterComponent(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.infra.Components = append(s.infra.Components, types.InfraComponent{
		Name: name,
		Status: types.Status{
			State:       types.StatePending,
			Description: "",
			LastUpdated: time.Now(),
		},
	})

	return nil
}

func (s *memoryStore) SetComponentStatus(name string, status types.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, component := range s.infra.Components {
		if component.Name == name {
			s.infra.Components[i].Status = status
			return nil
		}
	}

	return fmt.Errorf("component %s not found", name)
}

func (s *memoryStore) AddLogs(logs string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.infra.Logs += logs + "\n"
	if len(s.infra.Logs) > maxLogSize {
		s.infra.Logs = "... (truncated) " + s.infra.Logs[len(s.infra.Logs)-maxLogSize:]
	}

	return nil
}

func (s *memoryStore) GetLogs() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.infra.Logs, nil
}
