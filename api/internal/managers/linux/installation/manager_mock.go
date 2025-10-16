package installation

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/mock"
)

var _ InstallationManager = (*MockInstallationManager)(nil)

// MockInstallationManager is a mock implementation of the InstallationManager interface
type MockInstallationManager struct {
	mock.Mock
}

// GetConfig mocks the GetConfig method
func (m *MockInstallationManager) GetConfig(rc runtimeconfig.RuntimeConfig) (types.LinuxInstallationConfig, error) {
	args := m.Called(rc)
	if args.Get(0) == nil {
		return types.LinuxInstallationConfig{}, args.Error(1)
	}
	return args.Get(0).(types.LinuxInstallationConfig), args.Error(1)
}

// GetConfigValues mocks the GetConfigValues method
func (m *MockInstallationManager) GetConfigValues() (types.LinuxInstallationConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return types.LinuxInstallationConfig{}, args.Error(1)
	}
	return args.Get(0).(types.LinuxInstallationConfig), args.Error(1)
}

// SetConfigValues mocks the SetConfigValues method
func (m *MockInstallationManager) SetConfigValues(config types.LinuxInstallationConfig) error {
	args := m.Called(config)
	return args.Error(0)
}

// GetDefaults mocks the GetDefaults method
func (m *MockInstallationManager) GetDefaults(rc runtimeconfig.RuntimeConfig) (types.LinuxInstallationConfig, error) {
	args := m.Called(rc)
	if args.Get(0) == nil {
		return types.LinuxInstallationConfig{}, args.Error(1)
	}
	return args.Get(0).(types.LinuxInstallationConfig), args.Error(1)
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
func (m *MockInstallationManager) ValidateConfig(config types.LinuxInstallationConfig, managerPort int) error {
	args := m.Called(config, managerPort)
	return args.Error(0)
}

// ConfigureHost mocks the ConfigureHost method
func (m *MockInstallationManager) ConfigureHost(ctx context.Context, rc runtimeconfig.RuntimeConfig) error {
	args := m.Called(ctx, rc)
	return args.Error(0)
}

// CalculateRegistrySettings mocks the CalculateRegistrySettings method
func (m *MockInstallationManager) CalculateRegistrySettings(ctx context.Context, rc runtimeconfig.RuntimeConfig) (*types.RegistrySettings, error) {
	args := m.Called(ctx, rc)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.RegistrySettings), args.Error(1)
}

// GetRegistrySettings mocks the GetRegistrySettings method
func (m *MockInstallationManager) GetRegistrySettings(ctx context.Context, rc runtimeconfig.RuntimeConfig) (*types.RegistrySettings, error) {
	args := m.Called(ctx, rc)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.RegistrySettings), args.Error(1)
}
