package kubernetesinstallation

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/mock"
)

var _ Installation = (*MockKubernetesInstallation)(nil)

// MockKubernetesInstallation is a mock implementation of the KubernetesInstallation interface
type MockKubernetesInstallation struct {
	mock.Mock
}

// Get mocks the Get method
func (m *MockKubernetesInstallation) Get() *ecv1beta1.KubernetesInstallationSpec {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*ecv1beta1.KubernetesInstallationSpec)
}

// Set mocks the Set method
func (m *MockKubernetesInstallation) Set(spec *ecv1beta1.KubernetesInstallationSpec) {
	m.Called(spec)
}

// AdminConsolePort mocks the AdminConsolePort method
func (m *MockKubernetesInstallation) AdminConsolePort() int {
	args := m.Called()
	return args.Int(0)
}

// ManagerPort mocks the ManagerPort method
func (m *MockKubernetesInstallation) ManagerPort() int {
	args := m.Called()
	return args.Int(0)
}

// ProxySpec mocks the ProxySpec method
func (m *MockKubernetesInstallation) ProxySpec() *ecv1beta1.ProxySpec {
	args := m.Called()
	return args.Get(0).(*ecv1beta1.ProxySpec)
}

// SetAdminConsolePort mocks the SetAdminConsolePort method
func (m *MockKubernetesInstallation) SetAdminConsolePort(port int) {
	m.Called(port)
}

// SetManagerPort mocks the SetManagerPort method
func (m *MockKubernetesInstallation) SetManagerPort(port int) {
	m.Called(port)
}

// SetProxySpec mocks the SetProxySpec method
func (m *MockKubernetesInstallation) SetProxySpec(proxySpec *ecv1beta1.ProxySpec) {
	m.Called(proxySpec)
}
