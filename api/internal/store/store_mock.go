package store

import (
	kubernetesinstallation "github.com/replicatedhq/embedded-cluster/api/internal/store/kubernetes/installation"
	linuxinfra "github.com/replicatedhq/embedded-cluster/api/internal/store/linux/infra"
	linuxinstallation "github.com/replicatedhq/embedded-cluster/api/internal/store/linux/installation"
	linuxpreflight "github.com/replicatedhq/embedded-cluster/api/internal/store/linux/preflight"
)

var _ Store = (*MockStore)(nil)

// MockStore is a mock implementation of the Store interface
type MockStore struct {
	LinuxPreflightMockStore         linuxpreflight.MockStore
	LinuxInstallationMockStore      linuxinstallation.MockStore
	LinuxInfraMockStore             linuxinfra.MockStore
	KubernetesInstallationMockStore kubernetesinstallation.MockStore
}

// LinuxPreflightStore returns the mock linux preflight store
func (m *MockStore) LinuxPreflightStore() linuxpreflight.Store {
	return &m.LinuxPreflightMockStore
}

// LinuxInstallationStore returns the mock linux installation store
func (m *MockStore) LinuxInstallationStore() linuxinstallation.Store {
	return &m.LinuxInstallationMockStore
}

// LinuxInfraStore returns the mock linux infra store
func (m *MockStore) LinuxInfraStore() linuxinfra.Store {
	return &m.LinuxInfraMockStore
}

// KubernetesInstallationStore returns the mock kubernetes installation store
func (m *MockStore) KubernetesInstallationStore() kubernetesinstallation.Store {
	return &m.KubernetesInstallationMockStore
}
