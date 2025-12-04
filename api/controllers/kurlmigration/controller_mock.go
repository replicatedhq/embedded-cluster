package kurlmigration

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
func (m *MockController) GetInstallationConfig(ctx context.Context) (types.LinuxInstallationConfigResponse, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return types.LinuxInstallationConfigResponse{}, args.Error(1)
	}
	return args.Get(0).(types.LinuxInstallationConfigResponse), args.Error(1)
}

// StartKURLMigration mocks the StartKURLMigration method
func (m *MockController) StartKURLMigration(ctx context.Context, transferMode types.TransferMode, config types.LinuxInstallationConfig) (string, error) {
	args := m.Called(ctx, transferMode, config)
	return args.String(0), args.Error(1)
}

// GetKURLMigrationStatus mocks the GetKURLMigrationStatus method
func (m *MockController) GetKURLMigrationStatus(ctx context.Context) (types.KURLMigrationStatusResponse, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return types.KURLMigrationStatusResponse{}, args.Error(1)
	}
	return args.Get(0).(types.KURLMigrationStatusResponse), args.Error(1)
}

// Run mocks the Run method
func (m *MockController) Run(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
