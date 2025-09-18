package kotscli

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/stretchr/testify/mock"
)

var _ KotsCLI = (*MockKotsCLI)(nil)

// MockKotsCLI is a mock implementation of the KotsCLI interface
type MockKotsCLI struct {
	mock.Mock
}

// Install mocks the Install method
func (m *MockKotsCLI) Install(opts InstallOptions) error {
	args := m.Called(opts)
	return args.Error(0)
}

// ResetPassword mocks the ResetPassword method
func (m *MockKotsCLI) ResetPassword(rc runtimeconfig.RuntimeConfig, password string) error {
	args := m.Called(rc, password)
	return args.Error(0)
}

// AirgapUpdate mocks the AirgapUpdate method
func (m *MockKotsCLI) AirgapUpdate(opts AirgapUpdateOptions) error {
	args := m.Called(opts)
	return args.Error(0)
}

// VeleroConfigureOtherS3 mocks the VeleroConfigureOtherS3 method
func (m *MockKotsCLI) VeleroConfigureOtherS3(opts VeleroConfigureOtherS3Options) error {
	args := m.Called(opts)
	return args.Error(0)
}

// GetJoinCommand mocks the GetJoinCommand method
func (m *MockKotsCLI) GetJoinCommand(ctx context.Context, rc runtimeconfig.RuntimeConfig) (string, error) {
	args := m.Called(ctx, rc)
	return args.String(0), args.Error(1)
}

// Deploy mocks the Deploy method
func (m *MockKotsCLI) Deploy(opts DeployOptions) error {
	args := m.Called(opts)
	return args.Error(0)
}

// MaskKotsOutputForOnline mocks the MaskKotsOutputForOnline method
func (m *MockKotsCLI) MaskKotsOutputForOnline() spinner.MaskFn {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(spinner.MaskFn)
}

// MaskKotsOutputForAirgap mocks the MaskKotsOutputForAirgap method
func (m *MockKotsCLI) MaskKotsOutputForAirgap() spinner.MaskFn {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(spinner.MaskFn)
}
