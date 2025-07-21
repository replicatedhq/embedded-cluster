package config

import (
	apitemplate "github.com/replicatedhq/embedded-cluster/api/pkg/template"
	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/mock"
)

var _ AppConfigManager = (*MockAppConfigManager)(nil)

// MockAppConfigManager is a mock implementation of the AppConfigManager interface
type MockAppConfigManager struct {
	mock.Mock
}

// GetConfig mocks the GetConfig method
func (m *MockAppConfigManager) GetConfig(config apitemplate.InstallationConfig) (types.AppConfig, error) {
	args := m.Called(config)
	return args.Get(0).(types.AppConfig), args.Error(1)
}

// GetConfigValues mocks the GetConfigValues method
func (m *MockAppConfigManager) GetConfigValues(maskPasswords bool, config apitemplate.InstallationConfig) (types.AppConfigValues, error) {
	args := m.Called(maskPasswords, config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(types.AppConfigValues), args.Error(1)
}

// ValidateConfigValues mocks the ValidateConfigValues method
func (m *MockAppConfigManager) ValidateConfigValues(configValues types.AppConfigValues, config apitemplate.InstallationConfig) error {
	args := m.Called(configValues, config)
	return args.Error(0)
}

// PatchConfigValues mocks the PatchConfigValues method
func (m *MockAppConfigManager) PatchConfigValues(values types.AppConfigValues, config apitemplate.InstallationConfig) error {
	args := m.Called(values, config)
	return args.Error(0)
}

// GetKotsadmConfigValues mocks the GetKotsadmConfigValues method
func (m *MockAppConfigManager) GetKotsadmConfigValues(config apitemplate.InstallationConfig) (kotsv1beta1.ConfigValues, error) {
	args := m.Called(config)
	return args.Get(0).(kotsv1beta1.ConfigValues), args.Error(1)
}

// TemplateConfig mocks the TemplateConfig method
func (m *MockAppConfigManager) TemplateConfig(configValues types.AppConfigValues, config apitemplate.InstallationConfig) (types.AppConfig, error) {
	args := m.Called(configValues, config)
	return args.Get(0).(types.AppConfig), args.Error(1)
}
