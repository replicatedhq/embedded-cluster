package store

import (
	"github.com/replicatedhq/embedded-cluster/api/internal/store/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/store/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/store/preflight"
)

var _ Store = &memoryStore{}

// Store is the global interface that combines all substores
type Store interface {
	// PreflightStore provides access to host preflight operations
	PreflightStore() preflight.Store

	// InstallationStore provides access to installation operations
	InstallationStore() installation.Store

	// InfraStore provides access to infrastructure operations
	InfraStore() infra.Store
}

// StoreOption is a function that configures a store
type StoreOption func(*memoryStore)

// WithPreflightStore sets the preflight store
func WithPreflightStore(store preflight.Store) StoreOption {
	return func(s *memoryStore) {
		s.preflightStore = store
	}
}

// WithInstallationStore sets the installation store
func WithInstallationStore(store installation.Store) StoreOption {
	return func(s *memoryStore) {
		s.installationStore = store
	}
}

// WithInfraStore sets the infra store
func WithInfraStore(store infra.Store) StoreOption {
	return func(s *memoryStore) {
		s.infraStore = store
	}
}

// memoryStore is an in-memory implementation of the global Store interface
type memoryStore struct {
	preflightStore    preflight.Store
	installationStore installation.Store
	infraStore        infra.Store
}

// NewMemoryStore creates a new memory store with the given options
func NewMemoryStore(opts ...StoreOption) Store {
	s := &memoryStore{}

	for _, opt := range opts {
		opt(s)
	}

	if s.preflightStore == nil {
		s.preflightStore = preflight.NewMemoryStore()
	}

	if s.installationStore == nil {
		s.installationStore = installation.NewMemoryStore()
	}

	if s.infraStore == nil {
		s.infraStore = infra.NewMemoryStore()
	}

	return s
}

func (s *memoryStore) PreflightStore() preflight.Store {
	return s.preflightStore
}

func (s *memoryStore) InstallationStore() installation.Store {
	return s.installationStore
}

func (s *memoryStore) InfraStore() infra.Store {
	return s.infraStore
}
