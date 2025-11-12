package install

import (
	"errors"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/preflight"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_InstallController_getStateFromPreflightsOutput(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*preflight.MockHostPreflightManager)
		expectedState statemachine.State
	}{
		{
			name: "status succeeded returns StateHostPreflightsSucceeded",
			setupMocks: func(m *preflight.MockHostPreflightManager) {
				m.On("GetHostPreflightStatus", mock.Anything).Return(types.Status{
					State: types.StateSucceeded,
				}, nil)
			},
			expectedState: states.StateHostPreflightsSucceeded,
		},
		{
			name: "status failed with failures returns StateHostPreflightsFailed",
			setupMocks: func(m *preflight.MockHostPreflightManager) {
				m.On("GetHostPreflightStatus", mock.Anything).Return(types.Status{
					State: types.StateFailed,
				}, nil)
				m.On("GetHostPreflightOutput", mock.Anything).Return(failedPreflightOutput, nil)
			},
			expectedState: states.StateHostPreflightsFailed,
		},
		{
			name: "status failed with nil output returns StateHostPreflightsExecutionFailed",
			setupMocks: func(m *preflight.MockHostPreflightManager) {
				m.On("GetHostPreflightStatus", mock.Anything).Return(types.Status{
					State: types.StateFailed,
				}, nil)
				m.On("GetHostPreflightOutput", mock.Anything).Return(nil, nil)
			},
			expectedState: states.StateHostPreflightsExecutionFailed,
		},
		{
			name: "status failed with no failures returns StateHostPreflightsExecutionFailed",
			setupMocks: func(m *preflight.MockHostPreflightManager) {
				m.On("GetHostPreflightStatus", mock.Anything).Return(types.Status{
					State: types.StateFailed,
				}, nil)
				m.On("GetHostPreflightOutput", mock.Anything).Return(successfulPreflightOutput, nil)
			},
			expectedState: states.StateHostPreflightsExecutionFailed,
		},
		{
			name: "status failed with output error returns StateHostPreflightsExecutionFailed",
			setupMocks: func(m *preflight.MockHostPreflightManager) {
				m.On("GetHostPreflightStatus", mock.Anything).Return(types.Status{
					State: types.StateFailed,
				}, nil)
				m.On("GetHostPreflightOutput", mock.Anything).Return(nil, errors.New("get output error"))
			},
			expectedState: states.StateHostPreflightsExecutionFailed,
		},
		{
			name: "get status error returns StateHostPreflightsExecutionFailed",
			setupMocks: func(m *preflight.MockHostPreflightManager) {
				m.On("GetHostPreflightStatus", mock.Anything).Return(types.Status{}, errors.New("get status error"))
			},
			expectedState: states.StateHostPreflightsExecutionFailed,
		},
		{
			name: "unexpected state returns StateHostPreflightsExecutionFailed",
			setupMocks: func(m *preflight.MockHostPreflightManager) {
				m.On("GetHostPreflightStatus", mock.Anything).Return(types.Status{
					State: types.StateRunning, // Unexpected state
				}, nil)
			},
			expectedState: states.StateHostPreflightsExecutionFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPreflightManager := &preflight.MockHostPreflightManager{}
			tt.setupMocks(mockPreflightManager)

			controller, err := NewInstallController(
				WithHostPreflightManager(mockPreflightManager),
				WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
				WithHelmClient(&helm.MockClient{}),
			)
			require.NoError(t, err)

			state := controller.getStateFromPreflightsOutput(t.Context())

			assert.Equal(t, tt.expectedState, state, "expected state %s but got %s", tt.expectedState, state)

			mockPreflightManager.AssertExpectations(t)
		})
	}
}
