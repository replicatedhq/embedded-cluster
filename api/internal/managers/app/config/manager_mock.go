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

// Get mocks the Get method
func (m *MockAppConfigManager) Get() (kotsv1beta1.Config, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return kotsv1beta1.Config{}, args.Error(1)
	}
	return args.Get(0).(kotsv1beta1.Config), args.Error(1)
}

// Set mocks the Set method
func (m *MockAppConfigManager) Set(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// RenderAppConfigValues mocks the RenderAppConfigValues method
func (m *MockAppConfigManager) RenderAppConfigValues() (kotsv1beta1.ConfigValues, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return kotsv1beta1.ConfigValues{}, args.Error(1)
	}
	return args.Get(0).(kotsv1beta1.ConfigValues), args.Error(1)
}
