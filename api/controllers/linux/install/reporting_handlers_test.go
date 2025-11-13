package install

import (
	"errors"
	"testing"
	"time"

	appcontroller "github.com/replicatedhq/embedded-cluster/api/controllers/app"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/internal/testutils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestInstallController_ReportingHandlers(t *testing.T) {
	tests := []struct {
		name         string
		currentState statemachine.State
		targetState  statemachine.State
		eventData    any
		setupMocks   func(*metrics.MockReporter)
	}{
		{
			name:         "report install succeeded",
			currentState: states.StateAppInstalling,
			targetState:  states.StateSucceeded,
			eventData:    nil,
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportInstallationSucceeded", mock.Anything)
			},
		},
		{
			name:         "report infrastructure install failed",
			currentState: states.StateInfrastructureInstalling,
			targetState:  states.StateInfrastructureInstallFailed,
			eventData:    errors.New("infrastructure install failed"),
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportInstallationFailed", mock.Anything, mock.MatchedBy(func(err error) bool {
					return err.Error() == "infrastructure install failed"
				}))
			},
		},
		{
			name:         "report app install failed",
			currentState: states.StateAppInstalling,
			targetState:  states.StateAppInstallFailed,
			eventData:    errors.New("app install failed"),
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportInstallationFailed", mock.Anything, mock.MatchedBy(func(err error) bool {
					return err.Error() == "app install failed"
				}))
			},
		},
		{
			name:         "report host configuration failed",
			currentState: states.StateHostConfiguring,
			targetState:  states.StateHostConfigurationFailed,
			eventData:    errors.New("host configuration failed"),
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportInstallationFailed", mock.Anything, mock.MatchedBy(func(err error) bool {
					return err.Error() == "host configuration failed"
				}))
			},
		},
		{
			name:         "report installation configuration failed",
			currentState: states.StateInstallationConfiguring,
			targetState:  states.StateInstallationConfigurationFailed,
			eventData:    errors.New("installation configuration failed"),
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportInstallationFailed", mock.Anything, mock.MatchedBy(func(err error) bool {
					return err.Error() == "installation configuration failed"
				}))
			},
		},
		{
			name:         "report host preflights succeeded",
			currentState: states.StateHostPreflightsRunning,
			targetState:  states.StateHostPreflightsSucceeded,
			eventData:    nil,
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportHostPreflightsSucceeded", mock.Anything)
			},
		},
		{
			name:         "report host preflights failed",
			currentState: states.StateHostPreflightsRunning,
			targetState:  states.StateHostPreflightsFailed,
			eventData: &types.PreflightsOutput{
				Fail: []types.PreflightsRecord{
					{
						Title:   "Host Check",
						Message: "Host check failed",
					},
				},
			},
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportHostPreflightsFailed", mock.Anything, mock.MatchedBy(func(output *types.PreflightsOutput) bool {
					return output.Fail[0].Title == "Host Check" && output.Fail[0].Message == "Host check failed"
				}))
			},
		},
		{
			name:         "report host preflights bypassed",
			currentState: states.StateHostPreflightsFailed,
			targetState:  states.StateHostPreflightsFailedBypassed,
			eventData: &types.PreflightsOutput{
				Fail: []types.PreflightsRecord{
					{
						Title:   "Host Check",
						Message: "Host check failed",
					},
				},
			},
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportHostPreflightsBypassed", mock.Anything, mock.MatchedBy(func(output *types.PreflightsOutput) bool {
					return output.Fail[0].Title == "Host Check" && output.Fail[0].Message == "Host check failed"
				}))
			},
		},
		{
			name:         "report app preflights succeeded",
			currentState: states.StateAppPreflightsRunning,
			targetState:  states.StateAppPreflightsSucceeded,
			eventData:    nil,
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportAppPreflightsSucceeded", mock.Anything)
			},
		},
		{
			name:         "report app preflights failed",
			currentState: states.StateAppPreflightsRunning,
			targetState:  states.StateAppPreflightsFailed,
			eventData: &types.PreflightsOutput{
				Fail: []types.PreflightsRecord{
					{
						Title:   "App Check",
						Message: "App check failed",
					},
				},
			},
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportAppPreflightsFailed", mock.Anything, mock.MatchedBy(func(output *types.PreflightsOutput) bool {
					return output.Fail[0].Title == "App Check" && output.Fail[0].Message == "App check failed"
				}))
			},
		},
		{
			name:         "report app preflights bypassed",
			currentState: states.StateAppPreflightsFailed,
			targetState:  states.StateAppPreflightsFailedBypassed,
			eventData: &types.PreflightsOutput{
				Fail: []types.PreflightsRecord{
					{
						Title:   "App Check",
						Message: "App check failed",
					},
				},
			},
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportAppPreflightsBypassed", mock.Anything, mock.MatchedBy(func(output *types.PreflightsOutput) bool {
					return output.Fail[0].Title == "App Check" && output.Fail[0].Message == "App check failed"
				}))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMetricsReporter := &metrics.MockReporter{}
			mockInfraManager := &infra.MockInfraManager{}

			tt.setupMocks(mockMetricsReporter)

			mockStore := &store.MockStore{}
			mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

			// Create state machine starting in the current state
			sm := NewStateMachine(
				WithCurrentState(tt.currentState),
			)

			// Create app controller (required for install controller)
			appController, err := appcontroller.NewAppController(
				appcontroller.WithStateMachine(sm),
				appcontroller.WithStore(mockStore),
				appcontroller.WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
				appcontroller.WithHelmClient(&helm.MockClient{}),
				appcontroller.WithLogger(testutils.TestLogger(t)),
			)
			require.NoError(t, err)

			controller, err := NewInstallController(
				WithStateMachine(sm),
				WithAppController(appController),
				WithMetricsReporter(mockMetricsReporter),
				WithStore(mockStore),
				WithInfraManager(mockInfraManager),
				WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
				WithHelmClient(&helm.MockClient{}),
				WithLogger(testutils.TestLogger(t)),
			)
			require.NoError(t, err)

			// Trigger the state transition
			lock, err := sm.AcquireLock()
			require.NoError(t, err)
			defer lock.Release()

			err = sm.Transition(lock, tt.targetState, tt.eventData)
			require.NoError(t, err)

			// Wait for the event handler goroutine to complete
			time.Sleep(1 * time.Second)

			// Verify that the metrics reporter was called as expected
			mockMetricsReporter.AssertExpectations(t)

			// Avoid unused variable error
			_ = controller
		})
	}
}
