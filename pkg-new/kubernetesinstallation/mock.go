package kubernetesinstallation

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/mock"
	helmcli "helm.sh/helm/v3/pkg/cli"
)

var _ Installation = (*MockInstallation)(nil)

// MockInstallation is a mock implementation of the KubernetesInstallation interface
type MockInstallation struct {
	mock.Mock
}

// Get mocks the Get method
func (m *MockInstallation) Get() *ecv1beta1.KubernetesInstallation {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*ecv1beta1.KubernetesInstallation)
}

// Set mocks the Set method
func (m *MockInstallation) Set(installation *ecv1beta1.KubernetesInstallation) {
	m.Called(installation)
}

// GetSpec mocks the GetSpec method
func (m *MockInstallation) GetSpec() ecv1beta1.KubernetesInstallationSpec {
	args := m.Called()
	return args.Get(0).(ecv1beta1.KubernetesInstallationSpec)
}

// SetSpec mocks the SetSpec method
func (m *MockInstallation) SetSpec(spec ecv1beta1.KubernetesInstallationSpec) {
	m.Called(spec)
}

// GetStatus mocks the GetStatus method
func (m *MockInstallation) GetStatus() ecv1beta1.KubernetesInstallationStatus {
	args := m.Called()
	return args.Get(0).(ecv1beta1.KubernetesInstallationStatus)
}

// SetStatus mocks the SetStatus method
func (m *MockInstallation) SetStatus(status ecv1beta1.KubernetesInstallationStatus) {
	m.Called(status)
}

// AdminConsolePort mocks the AdminConsolePort method
func (m *MockInstallation) AdminConsolePort() int {
	args := m.Called()
	return args.Int(0)
}

// ManagerPort mocks the ManagerPort method
func (m *MockInstallation) ManagerPort() int {
	args := m.Called()
	return args.Int(0)
}

// ProxySpec mocks the ProxySpec method
func (m *MockInstallation) ProxySpec() *ecv1beta1.ProxySpec {
	args := m.Called()
	return args.Get(0).(*ecv1beta1.ProxySpec)
}

// SetAdminConsolePort mocks the SetAdminConsolePort method
func (m *MockInstallation) SetAdminConsolePort(port int) {
	m.Called(port)
}

// SetManagerPort mocks the SetManagerPort method
func (m *MockInstallation) SetManagerPort(port int) {
	m.Called(port)
}

// SetProxySpec mocks the SetProxySpec method
func (m *MockInstallation) SetProxySpec(proxySpec *ecv1beta1.ProxySpec) {
	m.Called(proxySpec)
}

// PathToEmbeddedBinary mocks the PathToEmbeddedBinary method
func (m *MockInstallation) PathToEmbeddedBinary(binaryName string) (string, error) {
	args := m.Called(binaryName)
	return args.String(0), args.Error(1)
}

// SetKubernetesEnvSettings mocks the SetKubernetesEnvSettings method
func (m *MockInstallation) SetKubernetesEnvSettings(envSettings *helmcli.EnvSettings) {
	m.Called(envSettings)
}

// GetKubernetesEnvSettings mocks the GetKubernetesEnvSettings method
func (m *MockInstallation) GetKubernetesEnvSettings() *helmcli.EnvSettings {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*helmcli.EnvSettings)
}
