package metrics

import (
	"context"
	"os"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/mock"
)

var _ ReporterInterface = (*MockReporter)(nil)

// MockReporter is a mock implementation of the ReporterInterface
type MockReporter struct {
	mock.Mock
}

// ReportInstallationStarted mocks the ReportInstallationStarted method
func (m *MockReporter) ReportInstallationStarted(ctx context.Context, licenseID string, appSlug string) {
	m.Called(ctx, licenseID, appSlug)
}

// ReportInstallationSucceeded mocks the ReportInstallationSucceeded method
func (m *MockReporter) ReportInstallationSucceeded(ctx context.Context) {
	m.Called(ctx)
}

// ReportInstallationFailed mocks the ReportInstallationFailed method
func (m *MockReporter) ReportInstallationFailed(ctx context.Context, err error) {
	m.Called(ctx, err)
}

// ReportJoinStarted mocks the ReportJoinStarted method
func (m *MockReporter) ReportJoinStarted(ctx context.Context) {
	m.Called(ctx)
}

// ReportJoinSucceeded mocks the ReportJoinSucceeded method
func (m *MockReporter) ReportJoinSucceeded(ctx context.Context) {
	m.Called(ctx)
}

// ReportJoinFailed mocks the ReportJoinFailed method
func (m *MockReporter) ReportJoinFailed(ctx context.Context, err error) {
	m.Called(ctx, err)
}

// ReportPreflightsFailed mocks the ReportPreflightsFailed method
func (m *MockReporter) ReportPreflightsFailed(ctx context.Context, output *apitypes.HostPreflightOutput) {
	m.Called(ctx, output)
}

// ReportPreflightsBypassed mocks the ReportPreflightsBypassed method
func (m *MockReporter) ReportPreflightsBypassed(ctx context.Context, output *apitypes.HostPreflightOutput) {
	m.Called(ctx, output)
}

// ReportSignalAborted mocks the ReportSignalAborted method
func (m *MockReporter) ReportSignalAborted(ctx context.Context, signal os.Signal) {
	m.Called(ctx, signal)
}
