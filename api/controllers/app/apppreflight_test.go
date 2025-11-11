package app

import (
	"errors"
	"testing"

	apppreflightmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/preflight"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/app/release"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/types"
	pkgrelease "github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	failedPreflightOutput = &types.PreflightsOutput{
		Pass: []types.PreflightsRecord{},
		Warn: []types.PreflightsRecord{},
		Fail: []types.PreflightsRecord{
			{
				Title:   "Test Check Failed",
				Message: "This check failed",
			},
		},
	}

	successfulPreflightOutput = &types.PreflightsOutput{
		Pass: []types.PreflightsRecord{
			{
				Title:   "Test Check Passed",
				Message: "This check passed",
			},
		},
		Warn: []types.PreflightsRecord{},
		Fail: []types.PreflightsRecord{},
	}
)

func Test_AppController_getStateFromAppPreflightsOutput(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*apppreflightmanager.MockAppPreflightManager)
		expectedState statemachine.State
	}{
		{
			name: "status succeeded returns StateAppPreflightsSucceeded",
			setupMocks: func(m *apppreflightmanager.MockAppPreflightManager) {
				m.On("GetAppPreflightStatus", mock.Anything).Return(types.Status{
					State: types.StateSucceeded,
				}, nil)
			},
			expectedState: states.StateAppPreflightsSucceeded,
		},
		{
			name: "status failed with failures returns StateAppPreflightsFailed",
			setupMocks: func(m *apppreflightmanager.MockAppPreflightManager) {
				m.On("GetAppPreflightStatus", mock.Anything).Return(types.Status{
					State: types.StateFailed,
				}, nil)
				m.On("GetAppPreflightOutput", mock.Anything).Return(failedPreflightOutput, nil)
			},
			expectedState: states.StateAppPreflightsFailed,
		},
		{
			name: "status failed with nil output returns StateAppPreflightsExecutionFailed",
			setupMocks: func(m *apppreflightmanager.MockAppPreflightManager) {
				m.On("GetAppPreflightStatus", mock.Anything).Return(types.Status{
					State: types.StateFailed,
				}, nil)
				m.On("GetAppPreflightOutput", mock.Anything).Return(nil, nil)
			},
			expectedState: states.StateAppPreflightsExecutionFailed,
		},
		{
			name: "status failed with no failures returns StateAppPreflightsExecutionFailed",
			setupMocks: func(m *apppreflightmanager.MockAppPreflightManager) {
				m.On("GetAppPreflightStatus", mock.Anything).Return(types.Status{
					State: types.StateFailed,
				}, nil)
				m.On("GetAppPreflightOutput", mock.Anything).Return(successfulPreflightOutput, nil)
			},
			expectedState: states.StateAppPreflightsExecutionFailed,
		},
		{
			name: "status failed with output error returns StateAppPreflightsExecutionFailed",
			setupMocks: func(m *apppreflightmanager.MockAppPreflightManager) {
				m.On("GetAppPreflightStatus", mock.Anything).Return(types.Status{
					State: types.StateFailed,
				}, nil)
				m.On("GetAppPreflightOutput", mock.Anything).Return(nil, errors.New("get output error"))
			},
			expectedState: states.StateAppPreflightsExecutionFailed,
		},
		{
			name: "get status error returns StateAppPreflightsExecutionFailed",
			setupMocks: func(m *apppreflightmanager.MockAppPreflightManager) {
				m.On("GetAppPreflightStatus", mock.Anything).Return(types.Status{}, errors.New("get status error"))
			},
			expectedState: states.StateAppPreflightsExecutionFailed,
		},
		{
			name: "unexpected state returns StateAppPreflightsExecutionFailed",
			setupMocks: func(m *apppreflightmanager.MockAppPreflightManager) {
				m.On("GetAppPreflightStatus", mock.Anything).Return(types.Status{
					State: types.StateRunning, // Unexpected state
				}, nil)
			},
			expectedState: states.StateAppPreflightsExecutionFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPreflightManager := &apppreflightmanager.MockAppPreflightManager{}
			tt.setupMocks(mockPreflightManager)

			// Create a simple state machine with minimal transitions
			sm := statemachine.New(states.StateNew, map[statemachine.State][]statemachine.State{
				states.StateNew: {states.StateApplicationConfigured},
			})
			mockStore := &store.MockStore{}
			// Mock GetConfigValues call that happens during controller initialization
			mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

			controller, err := NewAppController(
				WithAppPreflightManager(mockPreflightManager),
				WithAppReleaseManager(&release.MockAppReleaseManager{}),
				WithStateMachine(sm),
				WithStore(mockStore),
				WithReleaseData(&pkgrelease.ReleaseData{
					AppConfig: &kotsv1beta1.Config{},
				}),
			)
			require.NoError(t, err)

			state := controller.getStateFromAppPreflightsOutput(t.Context())

			assert.Equal(t, tt.expectedState, state, "expected state %s but got %s", tt.expectedState, state)

			mockPreflightManager.AssertExpectations(t)
		})
	}
}
