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

// TODO: all the methods in this file aren't passing over the context.Context to avoid potential data races when using this struct in state machine event handler tests. See: https://github.com/stretchr/testify/issues/1597

// ReportInstallationStarted mocks the ReportInstallationStarted method
func (m *MockReporter) ReportInstallationStarted(ctx context.Context, licenseID string, appSlug string) {
	m.Called(mock.Anything, licenseID, appSlug)
}

// ReportInstallationSucceeded mocks the ReportInstallationSucceeded method
func (m *MockReporter) ReportInstallationSucceeded(ctx context.Context) {
	m.Called(mock.Anything)
}

// ReportInstallationFailed mocks the ReportInstallationFailed method
func (m *MockReporter) ReportInstallationFailed(ctx context.Context, err error) {
	m.Called(mock.Anything, err)
}

// ReportJoinStarted mocks the ReportJoinStarted method
func (m *MockReporter) ReportJoinStarted(ctx context.Context) {
	m.Called(mock.Anything)
}

// ReportJoinSucceeded mocks the ReportJoinSucceeded method
func (m *MockReporter) ReportJoinSucceeded(ctx context.Context) {
	m.Called(mock.Anything)
}

// ReportJoinFailed mocks the ReportJoinFailed method
func (m *MockReporter) ReportJoinFailed(ctx context.Context, err error) {
	m.Called(mock.Anything, err)
}

// ReportPreflightsFailed mocks the ReportPreflightsFailed method
func (m *MockReporter) ReportPreflightsFailed(ctx context.Context, output *apitypes.PreflightsOutput) {
	m.Called(mock.Anything, output)
}

// ReportPreflightsBypassed mocks the ReportPreflightsBypassed method
func (m *MockReporter) ReportPreflightsBypassed(ctx context.Context, output *apitypes.PreflightsOutput) {
	m.Called(mock.Anything, output)
}

// ReportSignalAborted mocks the ReportSignalAborted method
func (m *MockReporter) ReportSignalAborted(ctx context.Context, signal os.Signal) {
	m.Called(mock.Anything, signal)
}
