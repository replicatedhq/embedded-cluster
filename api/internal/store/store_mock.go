package store

import (
	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/store/app/config"
	"github.com/replicatedhq/embedded-cluster/api/internal/store/infra"
	kubernetesinstallation "github.com/replicatedhq/embedded-cluster/api/internal/store/kubernetes/installation"
	linuxinstallation "github.com/replicatedhq/embedded-cluster/api/internal/store/linux/installation"
	linuxpreflight "github.com/replicatedhq/embedded-cluster/api/internal/store/linux/preflight"
)

var _ Store = (*MockStore)(nil)

// MockStore is a mock implementation of the Store interface
type MockStore struct {
	LinuxPreflightMockStore         linuxpreflight.MockStore
	LinuxInstallationMockStore      linuxinstallation.MockStore
	LinuxInfraMockStore             infra.MockStore
	KubernetesInstallationMockStore kubernetesinstallation.MockStore
	KubernetesInfraMockStore        infra.MockStore
	AppConfigMockStore              appconfig.MockStore
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
func (m *MockStore) LinuxInfraStore() infra.Store {
	return &m.LinuxInfraMockStore
}

// KubernetesInstallationStore returns the mock kubernetes installation store
func (m *MockStore) KubernetesInstallationStore() kubernetesinstallation.Store {
	return &m.KubernetesInstallationMockStore
}

// KubernetesInfraStore returns the mock kubernetes infra store
func (m *MockStore) KubernetesInfraStore() infra.Store {
	return &m.KubernetesInfraMockStore
}

// AppConfigStore returns the mock app config store
func (m *MockStore) AppConfigStore() appconfig.Store {
	return &m.AppConfigMockStore
}
