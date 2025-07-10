package config

import (
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/mock"
)

var _ AppConfigManager = (*MockAppConfigManager)(nil)

// MockAppConfigManager is a mock implementation of the AppConfigManager interface
type MockAppConfigManager struct {
	mock.Mock
}

// GetConfig mocks the GetConfig method
func (m *MockAppConfigManager) GetConfig(config kotsv1beta1.Config) (kotsv1beta1.Config, error) {
	args := m.Called(config)
	return args.Get(0).(kotsv1beta1.Config), args.Error(1)
}

// GetConfigValues mocks the GetConfigValues method
func (m *MockAppConfigManager) GetConfigValues() (map[string]string, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

// SetConfigValues mocks the SetConfigValues method
func (m *MockAppConfigManager) SetConfigValues(config kotsv1beta1.Config, configValues map[string]string) error {
	args := m.Called(config, configValues)
	return args.Error(0)
}
