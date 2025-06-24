package preflights

import (
	"context"
	"io"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/mock"
)

var _ PreflightsRunnerInterface = (*MockPreflightRunner)(nil)

// MockPreflightRunner is a mock implementation of the PreflightRunnerInterface
type MockPreflightRunner struct {
	mock.Mock
}

// Prepare mocks the Prepare method
func (m *MockPreflightRunner) Prepare(ctx context.Context, opts PrepareOptions) (*troubleshootv1beta2.HostPreflightSpec, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*troubleshootv1beta2.HostPreflightSpec), args.Error(1)
}

// Run mocks the Run method
func (m *MockPreflightRunner) Run(ctx context.Context, spec *troubleshootv1beta2.HostPreflightSpec, rc runtimeconfig.RuntimeConfig) (*apitypes.HostPreflightsOutput, string, error) {
	args := m.Called(ctx, spec, rc)
	if args.Get(0) == nil {
		return nil, args.String(1), args.Error(2)
	}
	return args.Get(0).(*apitypes.HostPreflightsOutput), args.String(1), args.Error(2)
}

// CopyBundleTo mocks the CopyBundleTo method
func (m *MockPreflightRunner) CopyBundleTo(dst string) error {
	args := m.Called(dst)
	return args.Error(0)
}

// SaveToDisk mocks the SaveToDisk method
func (m *MockPreflightRunner) SaveToDisk(output *apitypes.HostPreflightsOutput, path string) error {
	args := m.Called(output, path)
	return args.Error(0)
}

// OutputFromReader mocks the OutputFromReader method
func (m *MockPreflightRunner) OutputFromReader(reader io.Reader) (*apitypes.HostPreflightsOutput, error) {
	args := m.Called(reader)
	return args.Get(0).(*apitypes.HostPreflightsOutput), args.Error(1)
}

// PrintTable mocks the PrintTable method
func (m *MockPreflightRunner) PrintTable(o *apitypes.HostPreflightsOutput) {
	m.Called(o)
}

// PrintTableWithoutInfo mocks the PrintTableWithoutInfo method
func (m *MockPreflightRunner) PrintTableWithoutInfo(o *apitypes.HostPreflightsOutput) {
	m.Called(o)
}
