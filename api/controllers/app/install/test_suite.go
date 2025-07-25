package install

import (
	"errors"
	"testing"
	"time"

	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AppInstallControllerTestSuite struct {
	suite.Suite
	installType        string
	createStateMachine func(initialState statemachine.State) statemachine.Interface
}

func (s *AppInstallControllerTestSuite) TestPatchAppConfigValues() {
	tests := []struct {
		name          string
		values        types.AppConfigValues
		currentState  statemachine.State
		expectedState statemachine.State
		setupMocks    func(*appconfig.MockAppConfigManager)
		expectedErr   bool
	}{
		{
			name: "successful set app config values",
			values: types.AppConfigValues{
				"test-item": types.AppConfigValue{Value: "new-item"},
			},
			currentState:  states.StateNew,
			expectedState: states.StateApplicationConfigured,
			setupMocks: func(am *appconfig.MockAppConfigManager) {
				mock.InOrder(
					am.On("ValidateConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(nil),
					am.On("PatchConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "successful set app config values from application configuration failed state",
			values: types.AppConfigValues{
				"test-item": types.AppConfigValue{Value: "new-item"},
			},
			currentState:  states.StateApplicationConfigurationFailed,
			expectedState: states.StateApplicationConfigured,
			setupMocks: func(am *appconfig.MockAppConfigManager) {
				mock.InOrder(
					am.On("ValidateConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(nil),
					am.On("PatchConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "successful set app config values from application configured state",
			values: types.AppConfigValues{
				"test-item": types.AppConfigValue{Value: "new-item"},
			},
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateApplicationConfigured,
			setupMocks: func(am *appconfig.MockAppConfigManager) {
				mock.InOrder(
					am.On("ValidateConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(nil),
					am.On("PatchConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "validation error",
			values: types.AppConfigValues{
				"test-item": types.AppConfigValue{Value: "invalid-value"},
			},
			currentState:  states.StateNew,
			expectedState: states.StateApplicationConfigurationFailed,
			setupMocks: func(am *appconfig.MockAppConfigManager) {
				mock.InOrder(
					am.On("ValidateConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "invalid-value"}}).Return(errors.New("validation error")),
				)
			},
			expectedErr: true,
		},
		{
			name: "set config values error",
			values: types.AppConfigValues{
				"test-item": types.AppConfigValue{Value: "new-item"},
			},
			currentState:  states.StateNew,
			expectedState: states.StateApplicationConfigurationFailed,
			setupMocks: func(am *appconfig.MockAppConfigManager) {
				mock.InOrder(
					am.On("ValidateConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(nil),
					am.On("PatchConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(errors.New("set config error")),
				)
			},
			expectedErr: true,
		},
		{
			name: "invalid state transition",
			values: types.AppConfigValues{
				"test-item": types.AppConfigValue{Value: "new-item"},
			},
			currentState:  states.StateInfrastructureInstalling,
			expectedState: states.StateInfrastructureInstalling,
			setupMocks: func(am *appconfig.MockAppConfigManager) {
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {

			manager := &appconfig.MockAppConfigManager{}
			sm := s.createStateMachine(tt.currentState)
			controller, err := NewInstallController(
				WithStateMachine(sm),
				WithAppConfigManager(manager),
			)
			require.NoError(t, err, "failed to create install controller")

			tt.setupMocks(manager)
			err = controller.PatchAppConfigValues(t.Context(), tt.values)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Eventually(t, func() bool {
				return sm.CurrentState() == tt.expectedState
			}, time.Second, 100*time.Millisecond, "state should be %s but is %s", tt.expectedState, sm.CurrentState())
			assert.False(t, sm.IsLockAcquired(), "state machine should not be locked after setting app config values")
			manager.AssertExpectations(s.T())

		})
	}
}
