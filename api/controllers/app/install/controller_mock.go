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
