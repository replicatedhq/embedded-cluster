package installation

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/stretchr/testify/mock"
)

var _ InstallationManager = (*MockInstallationManager)(nil)

// MockInstallationManager is a mock implementation of the InstallationManager interface
type MockInstallationManager struct {
	mock.Mock
}

// GetConfig mocks the GetConfig method
func (m *MockInstallationManager) GetConfig() (types.KubernetesInstallationConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return types.KubernetesInstallationConfig{}, args.Error(1)
	}
	return args.Get(0).(types.KubernetesInstallationConfig), args.Error(1)
}

// GetConfigValues mocks the GetConfigValues method
func (m *MockInstallationManager) GetConfigValues() (types.KubernetesInstallationConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return types.KubernetesInstallationConfig{}, args.Error(1)
	}
	return args.Get(0).(types.KubernetesInstallationConfig), args.Error(1)
}

// SetConfigValues mocks the SetConfigValues method
func (m *MockInstallationManager) SetConfigValues(config types.KubernetesInstallationConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

// GetDefaults mocks the GetDefaults method
func (m *MockInstallationManager) GetDefaults() (types.KubernetesInstallationConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return types.KubernetesInstallationConfig{}, args.Error(1)
	}
	return args.Get(0).(types.KubernetesInstallationConfig), args.Error(1)
}

// GetStatus mocks the GetStatus method
func (m *MockInstallationManager) GetStatus() (types.Status, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return types.Status{}, args.Error(1)
	}
	return args.Get(0).(types.Status), args.Error(1)
}

// SetStatus mocks the SetStatus method
func (m *MockInstallationManager) SetStatus(status types.Status) error {
	args := m.Called(status)
	return args.Error(0)
}

// ValidateConfig mocks the ValidateConfig method
func (m *MockInstallationManager) ValidateConfig(config types.KubernetesInstallationConfig, managerPort int) error {
	args := m.Called(config, managerPort)
	return args.Error(0)
}


// ConfigureInstallation mocks the ConfigureInstallation method
func (m *MockInstallationManager) ConfigureInstallation(ctx context.Context, ki kubernetesinstallation.Installation, config types.KubernetesInstallationConfig) (finalErr error) {
	args := m.Called(ctx, ki, config)
	return args.Error(0)
}
