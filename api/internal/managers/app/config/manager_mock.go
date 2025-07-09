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

// GetConfigValues mocks the GetConfigValues method
func (m *MockAppConfigManager) GetConfigValues() (map[string]string, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

// SetConfigValues mocks the SetConfigValues method
func (m *MockAppConfigManager) SetConfigValues(ctx context.Context, values map[string]string) error {
	args := m.Called(ctx, values)
	return args.Error(0)
}

// ApplyValuesToConfig mocks the ApplyValuesToConfig method
func (m *MockAppConfigManager) ApplyValuesToConfig(config kotsv1beta1.Config, configValues map[string]string) (kotsv1beta1.Config, error) {
	args := m.Called(config, configValues)
	return args.Get(0).(kotsv1beta1.Config), args.Error(1)
}
