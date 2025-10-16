package appupgrade

import (
	"fmt"
	"sync"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/tiendc/go-deepcopy"
)

const maxLogSize = 100 * 1024 // 100KB total log size

var _ Store = &memoryStore{}

// Store provides methods for storing and retrieving app upgrade state
type Store interface {
	Get() (types.AppUpgrade, error)
	GetStatus() (types.Status, error)
	SetStatus(status types.Status) error
	SetStatusDesc(desc string) error
	AddLogs(logs string) error
	GetLogs() (string, error)
}

// memoryStore is an in-memory implementation of Store
type memoryStore struct {
	appUpgrade types.AppUpgrade
	mu         sync.RWMutex
}

type StoreOption func(*memoryStore)

func WithAppUpgrade(appUpgrade types.AppUpgrade) StoreOption {
	return func(s *memoryStore) {
		s.appUpgrade = appUpgrade
	}
}

// NewMemoryStore creates a new memory store
func NewMemoryStore(opts ...StoreOption) Store {
	s := &memoryStore{
		appUpgrade: types.AppUpgrade{
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

func (s *memoryStore) Get() (types.AppUpgrade, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var appUpgrade types.AppUpgrade
	if err := deepcopy.Copy(&appUpgrade, &s.appUpgrade); err != nil {
		return types.AppUpgrade{}, err
	}

	return appUpgrade, nil
}

func (s *memoryStore) GetStatus() (types.Status, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var status types.Status
	if err := deepcopy.Copy(&status, &s.appUpgrade.Status); err != nil {
		return types.Status{}, err
	}

	return status, nil
}

func (s *memoryStore) SetStatus(status types.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appUpgrade.Status = status
	return nil
}

func (s *memoryStore) SetStatusDesc(desc string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.appUpgrade.Status.State == "" {
		return fmt.Errorf("state not set")
	}

	s.appUpgrade.Status.Description = desc
	return nil
}

func (s *memoryStore) AddLogs(logs string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.appUpgrade.Logs += logs + "\n"
	if len(s.appUpgrade.Logs) > maxLogSize {
		s.appUpgrade.Logs = "... (truncated) " + s.appUpgrade.Logs[len(s.appUpgrade.Logs)-maxLogSize:]
	}

	return nil
}

func (s *memoryStore) GetLogs() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.appUpgrade.Logs, nil
}
