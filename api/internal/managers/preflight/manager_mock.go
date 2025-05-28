package preflight

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/mock"
)

var _ HostPreflightManager = (*MockHostPreflightManager)(nil)

// MockHostPreflightManager is a mock implementation of the HostPreflightManager interface
type MockHostPreflightManager struct {
	mock.Mock
}

// PrepareHostPreflights mocks the PrepareHostPreflights method
func (m *MockHostPreflightManager) PrepareHostPreflights(ctx context.Context, opts PrepareHostPreflightOptions) (*troubleshootv1beta2.HostPreflightSpec, *ecv1beta1.ProxySpec, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	if args.Get(1) == nil {
		return args.Get(0).(*troubleshootv1beta2.HostPreflightSpec), nil, args.Error(2)
	}
	return args.Get(0).(*troubleshootv1beta2.HostPreflightSpec), args.Get(1).(*ecv1beta1.ProxySpec), args.Error(2)
}

// RunHostPreflights mocks the RunHostPreflights method
func (m *MockHostPreflightManager) RunHostPreflights(ctx context.Context, opts RunHostPreflightOptions) error {
	args := m.Called(ctx, opts)
	return args.Error(0)
}

// GetHostPreflightStatus mocks the GetHostPreflightStatus method
func (m *MockHostPreflightManager) GetHostPreflightStatus(ctx context.Context) (*types.Status, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Status), args.Error(1)
}

// GetHostPreflightOutput mocks the GetHostPreflightOutput method
func (m *MockHostPreflightManager) GetHostPreflightOutput(ctx context.Context) (*types.HostPreflightOutput, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.HostPreflightOutput), args.Error(1)
}

// GetHostPreflightTitles mocks the GetHostPreflightTitles method
func (m *MockHostPreflightManager) GetHostPreflightTitles(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}
