package preflight

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/mock"
)

var _ HostPreflightManager = (*MockHostPreflightManager)(nil)

// MockHostPreflightManager is a mock implementation of the HostPreflightManager interface
type MockHostPreflightManager struct {
	mock.Mock
}

// PrepareHostPreflights mocks the PrepareHostPreflights method
func (m *MockHostPreflightManager) PrepareHostPreflights(ctx context.Context, rc runtimeconfig.RuntimeConfig, opts PrepareHostPreflightOptions) (*troubleshootv1beta2.HostPreflightSpec, error) {
	args := m.Called(ctx, rc, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*troubleshootv1beta2.HostPreflightSpec), args.Error(1)
}

// RunHostPreflights mocks the RunHostPreflights method
func (m *MockHostPreflightManager) RunHostPreflights(ctx context.Context, rc runtimeconfig.RuntimeConfig, opts RunHostPreflightOptions) (*types.PreflightsOutput, error) {
	args := m.Called(ctx, rc, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.PreflightsOutput), args.Error(1)
}

// GetHostPreflightStatus mocks the GetHostPreflightStatus method
func (m *MockHostPreflightManager) GetHostPreflightStatus(ctx context.Context) (types.Status, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return types.Status{}, args.Error(1)
	}
	return args.Get(0).(types.Status), args.Error(1)
}

// GetHostPreflightOutput mocks the GetHostPreflightOutput method
func (m *MockHostPreflightManager) GetHostPreflightOutput(ctx context.Context) (*types.PreflightsOutput, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.PreflightsOutput), args.Error(1)
}

// GetHostPreflightTitles mocks the GetHostPreflightTitles method
func (m *MockHostPreflightManager) GetHostPreflightTitles(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// ClearHostPreflightResults mocks the ClearHostPreflightResults method
func (m *MockHostPreflightManager) ClearHostPreflightResults(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
