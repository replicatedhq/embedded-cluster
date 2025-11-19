package preflights

import (
	"context"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/mock"
)

var _ PreflightRunnerInterface = (*MockPreflightRunner)(nil)

// MockPreflightRunner is a mock implementation of the PreflightRunnerInterface
type MockPreflightRunner struct {
	mock.Mock
}

// RunHostPreflights mocks the RunHostPreflights method
func (m *MockPreflightRunner) RunHostPreflights(ctx context.Context, spec *troubleshootv1beta2.HostPreflightSpec, opts RunOptions) (*apitypes.PreflightsOutput, string, error) {
	args := m.Called(ctx, spec, opts)
	if args.Get(0) == nil {
		return nil, args.String(1), args.Error(2)
	}
	return args.Get(0).(*apitypes.PreflightsOutput), args.String(1), args.Error(2)
}

// RunAppPreflights mocks the RunAppPreflights method
func (m *MockPreflightRunner) RunAppPreflights(ctx context.Context, spec *troubleshootv1beta2.PreflightSpec, opts RunOptions) (*apitypes.PreflightsOutput, string, error) {
	args := m.Called(ctx, spec, opts)
	if args.Get(0) == nil {
		return nil, args.String(1), args.Error(2)
	}
	return args.Get(0).(*apitypes.PreflightsOutput), args.String(1), args.Error(2)
}
