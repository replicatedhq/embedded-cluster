package install

import (
	"fmt"
	"sync"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/tiendc/go-deepcopy"
)

const maxLogSize = 100 * 1024 // 100KB total log size

var _ Store = &memoryStore{}

// Store provides methods for storing and retrieving app installation state
type Store interface {
	Get() (types.AppInstall, error)
	GetStatus() (types.Status, error)
	SetStatus(status types.Status) error
	SetStatusDesc(desc string) error
	AddLogs(logs string) error
	GetLogs() (string, error)
}

// memoryStore is an in-memory implementation of Store
type memoryStore struct {
	appInstall types.AppInstall
	mu         sync.RWMutex
}

type StoreOption func(*memoryStore)

func WithAppInstall(appInstall types.AppInstall) StoreOption {
	return func(s *memoryStore) {
		s.appInstall = appInstall
	}
}

// NewMemoryStore creates a new memory store
func NewMemoryStore(opts ...StoreOption) Store {
	s := &memoryStore{
		appInstall: types.AppInstall{
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

func (s *memoryStore) Get() (types.AppInstall, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var appInstall types.AppInstall
	if err := deepcopy.Copy(&appInstall, &s.appInstall); err != nil {
		return types.AppInstall{}, err
	}

	return appInstall, nil
}

func (s *memoryStore) GetStatus() (types.Status, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var status types.Status
	if err := deepcopy.Copy(&status, &s.appInstall.Status); err != nil {
		return types.Status{}, err
	}

	return status, nil
}

func (s *memoryStore) SetStatus(status types.Status) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appInstall.Status = status
	return nil
}

func (s *memoryStore) SetStatusDesc(desc string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.appInstall.Status.State == "" {
		return fmt.Errorf("state not set")
	}

	s.appInstall.Status.Description = desc
	return nil
}

func (s *memoryStore) AddLogs(logs string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.appInstall.Logs += logs + "\n"
	if len(s.appInstall.Logs) > maxLogSize {
		s.appInstall.Logs = "... (truncated) " + s.appInstall.Logs[len(s.appInstall.Logs)-maxLogSize:]
	}

	return nil
}

func (s *memoryStore) GetLogs() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.appInstall.Logs, nil
}
