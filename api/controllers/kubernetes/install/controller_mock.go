package install

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/mock"
)

var _ Controller = (*MockController)(nil)

// MockController is a mock implementation of the Controller interface
type MockController struct {
	mock.Mock
}

// GetInstallationConfig mocks the GetInstallationConfig method
func (m *MockController) GetInstallationConfig(ctx context.Context) (types.KubernetesInstallationConfig, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return types.KubernetesInstallationConfig{}, args.Error(1)
	}
	return args.Get(0).(types.KubernetesInstallationConfig), args.Error(1)
}

// ConfigureInstallation mocks the ConfigureInstallation method
func (m *MockController) ConfigureInstallation(ctx context.Context, config types.KubernetesInstallationConfig) error {
	args := m.Called(ctx, config)
	return args.Error(0)
}

// GetInstallationStatus mocks the GetInstallationStatus method
func (m *MockController) GetInstallationStatus(ctx context.Context) (types.Status, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return types.Status{}, args.Error(1)
	}
	return args.Get(0).(types.Status), args.Error(1)
}

// SetupInfra mocks the SetupInfra method
func (m *MockController) SetupInfra(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// GetInfra mocks the GetInfra method
func (m *MockController) GetInfra(ctx context.Context) (types.Infra, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return types.Infra{}, args.Error(1)
	}
	return args.Get(0).(types.Infra), args.Error(1)
}

// TemplateAppConfig mocks the TemplateAppConfig method
func (m *MockController) TemplateAppConfig(ctx context.Context, values types.AppConfigValues, maskPasswords bool) (types.AppConfig, error) {
	args := m.Called(ctx, values, maskPasswords)
	return args.Get(0).(types.AppConfig), args.Error(1)
}

// PatchAppConfigValues mocks the PatchAppConfigValues method
func (m *MockController) PatchAppConfigValues(ctx context.Context, values types.AppConfigValues) error {
	args := m.Called(ctx, values)
	return args.Error(0)
}

// GetAppConfigValues mocks the GetAppConfigValues method
func (m *MockController) GetAppConfigValues(ctx context.Context) (types.AppConfigValues, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(types.AppConfigValues), args.Error(1)
}
