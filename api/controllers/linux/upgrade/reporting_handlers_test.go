package upgrade

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

func TestUpgradeController_ReportingHandlers(t *testing.T) {
	tests := []struct {
		name                 string
		currentState         statemachine.State
		targetState          statemachine.State
		eventData            interface{}
		requiresInfraUpgrade bool
		targetVersion        string
		initialVersion       string
		setupMocks           func(*metrics.MockReporter)
	}{
		{
			name:                 "report upgrade succeeded",
			currentState:         states.StateAppUpgrading,
			targetState:          states.StateSucceeded,
			eventData:            nil,
			requiresInfraUpgrade: false,
			targetVersion:        "1.0.0",
			initialVersion:       "0.9.0",
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportUpgradeSucceeded", mock.Anything, "1.0.0", "0.9.0")
			},
		},
		{
			name:                 "report infrastructure upgrade failed",
			currentState:         states.StateInfrastructureUpgrading,
			targetState:          states.StateInfrastructureUpgradeFailed,
			eventData:            errors.New("infrastructure upgrade failed"),
			requiresInfraUpgrade: true,
			targetVersion:        "1.0.0",
			initialVersion:       "0.9.0",
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportUpgradeFailed", mock.Anything, mock.MatchedBy(func(err error) bool {
					return err.Error() == "infrastructure upgrade failed"
				}), "1.0.0", "0.9.0")
			},
		},
		{
			name:                 "report app upgrade failed",
			currentState:         states.StateAppUpgrading,
			targetState:          states.StateAppUpgradeFailed,
			eventData:            errors.New("app upgrade failed"),
			requiresInfraUpgrade: false,
			targetVersion:        "1.0.0",
			initialVersion:       "0.9.0",
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportUpgradeFailed", mock.Anything, mock.MatchedBy(func(err error) bool {
					return err.Error() == "app upgrade failed"
				}), "1.0.0", "0.9.0")
			},
		},
		{
			name:                 "report app preflights succeeded",
			currentState:         states.StateAppPreflightsRunning,
			targetState:          states.StateAppPreflightsSucceeded,
			eventData:            nil,
			requiresInfraUpgrade: false,
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
						Title:   "Test Check",
						Message: "Test check failed",
					},
				},
			},
			requiresInfraUpgrade: false,
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportAppPreflightsFailed", mock.Anything, mock.MatchedBy(func(output *types.PreflightsOutput) bool {
					return output.Fail[0].Title == "Test Check" && output.Fail[0].Message == "Test check failed"
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
						Title:   "Test Check",
						Message: "Test check failed",
					},
				},
			},
			requiresInfraUpgrade: false,
			setupMocks: func(mr *metrics.MockReporter) {
				mr.On("ReportAppPreflightsBypassed", mock.Anything, mock.MatchedBy(func(output *types.PreflightsOutput) bool {
					return output.Fail[0].Title == "Test Check" && output.Fail[0].Message == "Test check failed"
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

			// Mock RequiresUpgrade which is called during controller initialization
			mockInfraManager.On("RequiresUpgrade", mock.Anything, mock.Anything).Return(tt.requiresInfraUpgrade, nil)

			// Create state machine starting in the current state
			sm := NewStateMachine(
				WithCurrentState(tt.currentState),
				WithRequiresInfraUpgrade(tt.requiresInfraUpgrade),
			)

			// Create app controller (required for upgrade controller)
			appController, err := appcontroller.NewAppController(
				appcontroller.WithStateMachine(sm),
				appcontroller.WithStore(mockStore),
				appcontroller.WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
				appcontroller.WithHelmClient(&helm.MockClient{}),
				appcontroller.WithLogger(testutils.TestLogger(t)),
			)
			require.NoError(t, err)

			controller, err := NewUpgradeController(
				WithStateMachine(sm),
				WithAppController(appController),
				WithMetricsReporter(mockMetricsReporter),
				WithStore(mockStore),
				WithInfraManager(mockInfraManager),
				WithTargetVersion(tt.targetVersion),
				WithInitialVersion(tt.initialVersion),
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
