package config

import (
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
func (m *MockAppConfigManager) GetConfig() (types.AppConfig, error) {
	args := m.Called()
	return args.Get(0).(types.AppConfig), args.Error(1)
}

// GetConfigValues mocks the GetConfigValues method
func (m *MockAppConfigManager) GetConfigValues(maskPasswords bool) (types.AppConfigValues, error) {
	args := m.Called(maskPasswords)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(types.AppConfigValues), args.Error(1)
}

// ValidateConfigValues mocks the ValidateConfigValues method
func (m *MockAppConfigManager) ValidateConfigValues(configValues types.AppConfigValues) error {
	args := m.Called(configValues)
	return args.Error(0)
}

// PatchConfigValues mocks the PatchConfigValues method
func (m *MockAppConfigManager) PatchConfigValues(values types.AppConfigValues) error {
	args := m.Called(values)
	return args.Error(0)
}

// GetKotsadmConfigValues mocks the GetKotsadmConfigValues method
func (m *MockAppConfigManager) GetKotsadmConfigValues() (kotsv1beta1.ConfigValues, error) {
	args := m.Called()
	return args.Get(0).(kotsv1beta1.ConfigValues), args.Error(1)
}

// TemplateConfig mocks the TemplateConfig method
func (m *MockAppConfigManager) TemplateConfig(configValues types.AppConfigValues) (types.AppConfig, error) {
	args := m.Called(configValues)
	return args.Get(0).(types.AppConfig), args.Error(1)
}
