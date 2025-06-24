package kubernetesinstallation

import (
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/mock"
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

// SetStatus mocks the SetStatus method
func (m *MockInstallation) SetStatus(status ecv1beta1.KubernetesInstallationStatus) {
	m.Called(status)
}

// GetStatus mocks the GetStatus method
func (m *MockInstallation) GetStatus() ecv1beta1.KubernetesInstallationStatus {
	args := m.Called()
	return args.Get(0).(ecv1beta1.KubernetesInstallationStatus)
}
