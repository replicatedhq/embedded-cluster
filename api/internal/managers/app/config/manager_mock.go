package config

import (
	"context"

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

// PatchConfigValues mocks the PatchConfigValues method
func (m *MockAppConfigManager) PatchConfigValues(ctx context.Context, config kotsv1beta1.Config, values map[string]string) error {
	args := m.Called(ctx, config, values)
	return args.Error(0)
}

// GetConfigValues mocks the GetConfigValues method
func (m *MockAppConfigManager) GetConfigValues(ctx context.Context, appConfig kotsv1beta1.Config, maskPasswords bool) (map[string]string, error) {
	args := m.Called(ctx, appConfig, maskPasswords)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

// GetKotsadmConfigValues mocks the GetKotsadmConfigValues method
func (m *MockAppConfigManager) GetKotsadmConfigValues(config kotsv1beta1.Config) (kotsv1beta1.ConfigValues, error) {
	args := m.Called(config)
	return args.Get(0).(kotsv1beta1.ConfigValues), args.Error(1)
}
