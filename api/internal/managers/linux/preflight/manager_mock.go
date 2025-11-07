package preflight

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecmock "github.com/replicatedhq/embedded-cluster/pkg-new/mock"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

var _ HostPreflightManager = (*MockHostPreflightManager)(nil)

// MockHostPreflightManager is a mock implementation of the HostPreflightManager interface.
// It embeds ecmock.Mock which provides MaybeRegisterCall for automatic default stub behavior.
type MockHostPreflightManager struct {
	ecmock.Mock
}

// PrepareHostPreflights mocks the PrepareHostPreflights method
func (m *MockHostPreflightManager) PrepareHostPreflights(ctx context.Context, rc runtimeconfig.RuntimeConfig, opts PrepareHostPreflightOptions) (*troubleshootv1beta2.HostPreflightSpec, error) {
	if registered, args := m.MaybeRegisterCall(ctx, rc, opts); registered {
		if args.Get(0) == nil {
			return nil, args.Error(1)
		}
		return args.Get(0).(*troubleshootv1beta2.HostPreflightSpec), args.Error(1)
	}

	// Default stub: return empty spec, no error
	return &troubleshootv1beta2.HostPreflightSpec{}, nil
}

// RunHostPreflights mocks the RunHostPreflights method
func (m *MockHostPreflightManager) RunHostPreflights(ctx context.Context, rc runtimeconfig.RuntimeConfig, opts RunHostPreflightOptions) error {
	if registered, args := m.MaybeRegisterCall(ctx, rc, opts); registered {
		return args.Error(0)
	}

	// Default stub: succeed
	return nil
}

// GetHostPreflightStatus mocks the GetHostPreflightStatus method
func (m *MockHostPreflightManager) GetHostPreflightStatus(ctx context.Context) (types.Status, error) {
	if registered, args := m.MaybeRegisterCall(ctx); registered {
		if args.Get(0) == nil {
			return types.Status{}, args.Error(1)
		}
		return args.Get(0).(types.Status), args.Error(1)
	}

	// Default stub: return running state, no error
	return types.Status{State: types.StateRunning}, nil
}

// GetHostPreflightOutput mocks the GetHostPreflightOutput method
func (m *MockHostPreflightManager) GetHostPreflightOutput(ctx context.Context) (*types.PreflightsOutput, error) {
	if registered, args := m.MaybeRegisterCall(ctx); registered {
		if args.Get(0) == nil {
			return nil, args.Error(1)
		}
		return args.Get(0).(*types.PreflightsOutput), args.Error(1)
	}

	// Default stub: return empty successful output, no error
	return &types.PreflightsOutput{}, nil
}

// GetHostPreflightTitles mocks the GetHostPreflightTitles method
func (m *MockHostPreflightManager) GetHostPreflightTitles(ctx context.Context) ([]string, error) {
	if registered, args := m.MaybeRegisterCall(ctx); registered {
		if args.Get(0) == nil {
			return nil, args.Error(1)
		}
		return args.Get(0).([]string), args.Error(1)
	}

	// Default stub: return empty list, no error
	return []string{}, nil
}

// ClearHostPreflightResults mocks the ClearHostPreflightResults method
func (m *MockHostPreflightManager) ClearHostPreflightResults(ctx context.Context) error {
	if registered, args := m.MaybeRegisterCall(ctx); registered {
		return args.Error(0)
	}

	// Default stub: succeed
	return nil
}
