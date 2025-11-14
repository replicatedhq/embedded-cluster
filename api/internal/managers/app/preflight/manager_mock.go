package preflight

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/mock"
)

var _ AppPreflightManager = (*MockAppPreflightManager)(nil)

// MockAppPreflightManager is a mock implementation of the AppPreflightManager interface
type MockAppPreflightManager struct {
	mock.Mock
}

// RunAppPreflights mocks the RunAppPreflights method
func (m *MockAppPreflightManager) RunAppPreflights(ctx context.Context, opts RunAppPreflightOptions) (*types.PreflightsOutput, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.PreflightsOutput), args.Error(1)
}

// GetAppPreflightStatus mocks the GetAppPreflightStatus method
func (m *MockAppPreflightManager) GetAppPreflightStatus(ctx context.Context) (types.Status, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return types.Status{}, args.Error(1)
	}
	return args.Get(0).(types.Status), args.Error(1)
}

// GetAppPreflightOutput mocks the GetAppPreflightOutput method
func (m *MockAppPreflightManager) GetAppPreflightOutput(ctx context.Context) (*types.PreflightsOutput, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.PreflightsOutput), args.Error(1)
}

// GetAppPreflightTitles mocks the GetAppPreflightTitles method
func (m *MockAppPreflightManager) GetAppPreflightTitles(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// ClearAppPreflightResults mocks the ClearAppPreflightResults method
func (m *MockAppPreflightManager) ClearAppPreflightResults(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
