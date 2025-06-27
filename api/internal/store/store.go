package store

import (
	kubernetesinfra "github.com/replicatedhq/embedded-cluster/api/internal/store/kubernetes/infra"
	kubernetesinstallation "github.com/replicatedhq/embedded-cluster/api/internal/store/kubernetes/installation"
	linuxinfra "github.com/replicatedhq/embedded-cluster/api/internal/store/linux/infra"
	linuxinstallation "github.com/replicatedhq/embedded-cluster/api/internal/store/linux/installation"
	linuxpreflight "github.com/replicatedhq/embedded-cluster/api/internal/store/linux/preflight"
)

var _ Store = &memoryStore{}

// Store is the global interface that combines all substores
type Store interface {
	// LinuxPreflightStore provides access to host preflight operations
	LinuxPreflightStore() linuxpreflight.Store

	// LinuxInstallationStore provides access to installation operations
	LinuxInstallationStore() linuxinstallation.Store

	// LinuxInfraStore provides access to infrastructure operations
	LinuxInfraStore() linuxinfra.Store

	// KubernetesInstallationStore provides access to kubernetes installation operations
	KubernetesInstallationStore() kubernetesinstallation.Store

	// KubernetesInfraStore provides access to kubernetes infrastructure operations
	KubernetesInfraStore() kubernetesinfra.Store
}

// StoreOption is a function that configures a store
type StoreOption func(*memoryStore)

// WithLinuxPreflightStore sets the preflight store
func WithLinuxPreflightStore(store linuxpreflight.Store) StoreOption {
	return func(s *memoryStore) {
		s.linuxPreflightStore = store
	}
}

// WithLinuxInstallationStore sets the installation store
func WithLinuxInstallationStore(store linuxinstallation.Store) StoreOption {
	return func(s *memoryStore) {
		s.linuxInstallationStore = store
	}
}

// WithLinuxInfraStore sets the infra store
func WithLinuxInfraStore(store linuxinfra.Store) StoreOption {
	return func(s *memoryStore) {
		s.linuxInfraStore = store
	}
}

// WithKubernetesInstallationStore sets the kubernetes installation store
func WithKubernetesInstallationStore(store kubernetesinstallation.Store) StoreOption {
	return func(s *memoryStore) {
		s.kubernetesInstallationStore = store
	}
}

// WithKubernetesInfraStore sets the kubernetes infra store
func WithKubernetesInfraStore(store kubernetesinfra.Store) StoreOption {
	return func(s *memoryStore) {
		s.kubernetesInfraStore = store
	}
}

// memoryStore is an in-memory implementation of the global Store interface
type memoryStore struct {
	linuxPreflightStore    linuxpreflight.Store
	linuxInstallationStore linuxinstallation.Store
	linuxInfraStore        linuxinfra.Store

	kubernetesInstallationStore kubernetesinstallation.Store
	kubernetesInfraStore        kubernetesinfra.Store
}

// NewMemoryStore creates a new memory store with the given options
func NewMemoryStore(opts ...StoreOption) Store {
	s := &memoryStore{}

	for _, opt := range opts {
		opt(s)
	}

	if s.linuxPreflightStore == nil {
		s.linuxPreflightStore = linuxpreflight.NewMemoryStore()
	}

	if s.linuxInstallationStore == nil {
		s.linuxInstallationStore = linuxinstallation.NewMemoryStore()
	}

	if s.linuxInfraStore == nil {
		s.linuxInfraStore = linuxinfra.NewMemoryStore()
	}

	if s.kubernetesInstallationStore == nil {
		s.kubernetesInstallationStore = kubernetesinstallation.NewMemoryStore()
	}

	if s.kubernetesInfraStore == nil {
		s.kubernetesInfraStore = kubernetesinfra.NewMemoryStore()
	}

	return s
}

func (s *memoryStore) LinuxPreflightStore() linuxpreflight.Store {
	return s.linuxPreflightStore
}

func (s *memoryStore) LinuxInstallationStore() linuxinstallation.Store {
	return s.linuxInstallationStore
}

func (s *memoryStore) LinuxInfraStore() linuxinfra.Store {
	return s.linuxInfraStore
}

func (s *memoryStore) KubernetesInstallationStore() kubernetesinstallation.Store {
	return s.kubernetesInstallationStore
}

func (s *memoryStore) KubernetesInfraStore() kubernetesinfra.Store {
	return s.kubernetesInfraStore
}
