package kurlmigration

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/mock"
)

var _ Manager = (*MockManager)(nil)

// MockManager is a mock implementation of the Manager interface
type MockManager struct {
	mock.Mock
}

// GetKurlConfig mocks the GetKurlConfig method
func (m *MockManager) GetKurlConfig(ctx context.Context) (types.LinuxInstallationConfig, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return types.LinuxInstallationConfig{}, args.Error(1)
	}
	return args.Get(0).(types.LinuxInstallationConfig), args.Error(1)
}

// GetECDefaults mocks the GetECDefaults method
func (m *MockManager) GetECDefaults(ctx context.Context) (types.LinuxInstallationConfig, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return types.LinuxInstallationConfig{}, args.Error(1)
	}
	return args.Get(0).(types.LinuxInstallationConfig), args.Error(1)
}

// MergeConfigs mocks the MergeConfigs method
func (m *MockManager) MergeConfigs(userConfig, kurlConfig, defaults types.LinuxInstallationConfig) types.LinuxInstallationConfig {
	args := m.Called(userConfig, kurlConfig, defaults)
	return args.Get(0).(types.LinuxInstallationConfig)
}

// ValidateTransferMode mocks the ValidateTransferMode method
func (m *MockManager) ValidateTransferMode(mode types.TransferMode) error {
	args := m.Called(mode)
	return args.Error(0)
}

// ExecutePhase mocks the ExecutePhase method
func (m *MockManager) ExecutePhase(ctx context.Context, phase types.KURLMigrationPhase) error {
	args := m.Called(ctx, phase)
	return args.Error(0)
}
