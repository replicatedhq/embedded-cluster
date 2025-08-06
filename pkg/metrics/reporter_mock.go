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

// ReportInfraInstallationSucceeded mocks the ReportInfraInstallationSucceeded method
func (m *MockReporter) ReportInfraInstallationSucceeded(ctx context.Context) {
	m.Called(mock.Anything)
}

// ReportInfraInstallationFailed mocks the ReportInfraInstallationFailed method
func (m *MockReporter) ReportInfraInstallationFailed(ctx context.Context, err error) {
	m.Called(mock.Anything, err)
}

// ReportAppInstallationFailed mocks the ReportAppInstallationFailed method
func (m *MockReporter) ReportAppInstallationFailed(ctx context.Context, err error) {
	m.Called(mock.Anything, err)
}

// ReportAppInstallationSucceeded mocks the ReportAppInstallationSucceeded method
func (m *MockReporter) ReportAppInstallationSucceeded(ctx context.Context) {
	m.Called(mock.Anything)
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

// ReportHostPreflightsFailed mocks the ReportHostPreflightsFailed method
func (m *MockReporter) ReportHostPreflightsFailed(ctx context.Context, output *apitypes.PreflightsOutput) {
	m.Called(mock.Anything, output)
}

// ReportHostPreflightsBypassed mocks the ReportHostPreflightsBypassed method
func (m *MockReporter) ReportHostPreflightsBypassed(ctx context.Context, output *apitypes.PreflightsOutput) {
	m.Called(mock.Anything, output)
}

// ReportHostPreflightsSucceeded mocks the ReportHostPreflightsSucceeded method
func (m *MockReporter) ReportHostPreflightsSucceeded(ctx context.Context) {
	m.Called(mock.Anything)
}

// ReportAppPreflightsFailed mocks the ReportAppPreflightsFailed method
func (m *MockReporter) ReportAppPreflightsFailed(ctx context.Context, output *apitypes.PreflightsOutput) {
	m.Called(mock.Anything, output)
}

// ReportAppPreflightsBypassed mocks the ReportAppPreflightsBypassed method
func (m *MockReporter) ReportAppPreflightsBypassed(ctx context.Context, output *apitypes.PreflightsOutput) {
	m.Called(mock.Anything, output)
}

// ReportAppPreflightsSucceeded mocks the ReportAppPreflightsSucceeded method
func (m *MockReporter) ReportAppPreflightsSucceeded(ctx context.Context) {
	m.Called(mock.Anything)
}

// ReportSignalAborted mocks the ReportSignalAborted method
func (m *MockReporter) ReportSignalAborted(ctx context.Context, signal os.Signal) {
	m.Called(mock.Anything, signal)
}
