package store

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/internal/store/airgap"
	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/store/app/config"
	appinstall "github.com/replicatedhq/embedded-cluster/api/internal/store/app/install"
	apppreflight "github.com/replicatedhq/embedded-cluster/api/internal/store/app/preflight"
	appupgrade "github.com/replicatedhq/embedded-cluster/api/internal/store/app/upgrade"
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
	AppPreflightMockStore           apppreflight.MockStore
	AppInstallMockStore             appinstall.MockStore
	AppUpgradeMockStore             appupgrade.MockStore
	AirgapMockStore                 airgap.MockStore
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

// AppPreflightStore returns the mock app preflight store
func (m *MockStore) AppPreflightStore() apppreflight.Store {
	return &m.AppPreflightMockStore
}

// AppInstallStore returns the mock app install store
func (m *MockStore) AppInstallStore() appinstall.Store {
	return &m.AppInstallMockStore
}

// AppUpgradeStore returns the mock app upgrade store
func (m *MockStore) AppUpgradeStore() appupgrade.Store {
	return &m.AppUpgradeMockStore
}

// AirgapStore returns the mock airgap store
func (m *MockStore) AirgapStore() airgap.Store {
	return &m.AirgapMockStore
}

func (m *MockStore) AssertExpectations(t *testing.T) {
	m.LinuxPreflightMockStore.AssertExpectations(t)
	m.LinuxInstallationMockStore.AssertExpectations(t)
	m.LinuxInfraMockStore.AssertExpectations(t)
	m.KubernetesInstallationMockStore.AssertExpectations(t)
	m.KubernetesInfraMockStore.AssertExpectations(t)
	m.AppConfigMockStore.AssertExpectations(t)
	m.AppPreflightMockStore.AssertExpectations(t)
	m.AppInstallMockStore.AssertExpectations(t)
	m.AppUpgradeMockStore.AssertExpectations(t)
	m.AirgapMockStore.AssertExpectations(t)
}
