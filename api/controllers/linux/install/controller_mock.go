package install

import (
	"context"

	appcontroller "github.com/replicatedhq/embedded-cluster/api/controllers/app/install"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/mock"
)

var _ Controller = (*MockController)(nil)

// MockController is a mock implementation of the Controller interface
type MockController struct {
	mock.Mock
}

// GetInstallationConfig mocks the GetInstallationConfig method
func (m *MockController) GetInstallationConfig(ctx context.Context) (types.LinuxInstallationConfig, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return types.LinuxInstallationConfig{}, args.Error(1)
	}
	return args.Get(0).(types.LinuxInstallationConfig), args.Error(1)
}

// ConfigureInstallation mocks the ConfigureInstallation method
func (m *MockController) ConfigureInstallation(ctx context.Context, config types.LinuxInstallationConfig) error {
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

// RunHostPreflights mocks the RunHostPreflights method
func (m *MockController) RunHostPreflights(ctx context.Context, opts RunHostPreflightsOptions) error {
	args := m.Called(ctx, opts)
	return args.Error(0)
}

// GetHostPreflightStatus mocks the GetHostPreflightStatus method
func (m *MockController) GetHostPreflightStatus(ctx context.Context) (types.Status, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return types.Status{}, args.Error(1)
	}
	return args.Get(0).(types.Status), args.Error(1)
}

// GetHostPreflightOutput mocks the GetHostPreflightOutput method
func (m *MockController) GetHostPreflightOutput(ctx context.Context) (*types.PreflightsOutput, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.PreflightsOutput), args.Error(1)
}

// GetHostPreflightTitles mocks the GetHostPreflightTitles method
func (m *MockController) GetHostPreflightTitles(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// SetupInfra mocks the SetupInfra method
func (m *MockController) SetupInfra(ctx context.Context, ignoreHostPreflights bool) error {
	args := m.Called(ctx, ignoreHostPreflights)
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

// GetAppPreflightOutput mocks the GetAppPreflightOutput method
func (m *MockController) GetAppPreflightOutput(ctx context.Context) (*types.PreflightsOutput, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.PreflightsOutput), args.Error(1)
}

// GetAppPreflightStatus mocks the GetAppPreflightStatus method
func (m *MockController) GetAppPreflightStatus(ctx context.Context) (types.Status, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return types.Status{}, args.Error(1)
	}
	return args.Get(0).(types.Status), args.Error(1)
}

// GetAppPreflightTitles mocks the GetAppPreflightTitles method
func (m *MockController) GetAppPreflightTitles(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// RunAppPreflights mocks the RunAppPreflights method
func (m *MockController) RunAppPreflights(ctx context.Context, opts appcontroller.RunAppPreflightOptions) error {
	args := m.Called(ctx, opts)
	return args.Error(0)
}

// InstallApp mocks the InstallApp method
func (m *MockController) InstallApp(ctx context.Context, opts appcontroller.InstallAppOptions) error {
	args := m.Called(ctx, opts)
	return args.Error(0)
}

// GetAppInstallStatus mocks the GetAppInstallStatus method
func (m *MockController) GetAppInstallStatus(ctx context.Context) (types.AppInstall, error) {
	args := m.Called(ctx)
	return args.Get(0).(types.AppInstall), args.Error(1)
}

// CalculateRegistrySettings mocks the CalculateRegistrySettings method
func (m *MockController) CalculateRegistrySettings(ctx context.Context) (*types.RegistrySettings, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.RegistrySettings), args.Error(1)
}
