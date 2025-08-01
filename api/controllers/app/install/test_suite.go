package install

import (
	"errors"
	"testing"
	"time"

	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	appinstallmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/install"
	apppreflightmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/preflight"
	appreleasemanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/release"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AppInstallControllerTestSuite struct {
	suite.Suite
	InstallType        string
	CreateStateMachine func(initialState statemachine.State) statemachine.Interface
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
			setupMocks: func(acm *appconfig.MockAppConfigManager) {
				mock.InOrder(
					acm.On("ValidateConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(nil),
					acm.On("PatchConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(nil),
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
			setupMocks: func(acm *appconfig.MockAppConfigManager) {
				mock.InOrder(
					acm.On("ValidateConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(nil),
					acm.On("PatchConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(nil),
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
			setupMocks: func(acm *appconfig.MockAppConfigManager) {
				mock.InOrder(
					acm.On("ValidateConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(nil),
					acm.On("PatchConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(nil),
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
			setupMocks: func(acm *appconfig.MockAppConfigManager) {
				mock.InOrder(
					acm.On("ValidateConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "invalid-value"}}).Return(errors.New("validation error")),
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
			setupMocks: func(acm *appconfig.MockAppConfigManager) {
				mock.InOrder(
					acm.On("ValidateConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(nil),
					acm.On("PatchConfigValues", types.AppConfigValues{"test-item": types.AppConfigValue{Value: "new-item"}}).Return(errors.New("set config error")),
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
			setupMocks: func(acm *appconfig.MockAppConfigManager) {
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {

			appConfigManager := &appconfig.MockAppConfigManager{}
			appPreflightManager := &apppreflightmanager.MockAppPreflightManager{}
			appReleaseManager := &appreleasemanager.MockAppReleaseManager{}
			appInstallManager := &appinstallmanager.MockAppInstallManager{}
			sm := s.CreateStateMachine(tt.currentState)

			controller, err := NewInstallController(
				WithStateMachine(sm),
				WithAppConfigManager(appConfigManager),
				WithAppPreflightManager(appPreflightManager),
				WithAppReleaseManager(appReleaseManager),
				WithAppInstallManager(appInstallManager),
				WithStore(&store.MockStore{}),
				WithReleaseData(&release.ReleaseData{}),
			)
			require.NoError(t, err, "failed to create install controller")

			tt.setupMocks(appConfigManager)
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
			appConfigManager.AssertExpectations(s.T())

		})
	}
}

// func (s *AppInstallControllerTestSuite) TestRunAppPreflights() {
// 	expectedAPF := &troubleshootv1beta2.PreflightSpec{
// 		Collectors: []*troubleshootv1beta2.Collect{
// 			{
// 				ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
// 			},
// 		},
// 	}

// 	tests := []struct {
// 		name          string
// 		opts          RunAppPreflightOptions
// 		currentState  statemachine.State
// 		expectedState statemachine.State
// 		setupMocks    func(*apppreflightmanager.MockAppPreflightManager, *appreleasemanager.MockAppReleaseManager)
// 		expectedErr   bool
// 	}{
// 		{
// 			name: "successful execution with passing preflights",
// 			opts: RunAppPreflightOptions{
// 				ConfigValues: types.AppConfigValues{
// 					"test-item": types.AppConfigValue{Value: "test-value"},
// 				},
// 				PreflightBinaryPath: "/usr/bin/preflight",
// 			},
// 			currentState:  states.StateApplicationConfigured,
// 			expectedState: states.StateAppPreflightsSucceeded,
// 			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager) {
// 				mock.InOrder(
// 					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}).Return(expectedAPF, nil),
// 					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
// 						return expectedAPF == opts.AppPreflightSpec
// 					})).Return(nil),
// 					apm.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
// 						Pass: []types.PreflightsRecord{
// 							{
// 								Title:   "Test Check",
// 								Message: "Test check passed",
// 							},
// 						},
// 					}, nil),
// 				)
// 			},
// 			expectedErr: false,
// 		},
// 		{
// 			name: "successful execution from execution failed state with passing preflights",
// 			opts: RunAppPreflightOptions{
// 				ConfigValues: types.AppConfigValues{
// 					"test-item": types.AppConfigValue{Value: "test-value"},
// 				},
// 				PreflightBinaryPath: "/usr/bin/preflight",
// 			},
// 			currentState:  states.StateAppPreflightsExecutionFailed,
// 			expectedState: states.StateAppPreflightsSucceeded,
// 			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager) {
// 				mock.InOrder(
// 					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}).Return(expectedAPF, nil),
// 					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
// 						return expectedAPF == opts.AppPreflightSpec
// 					})).Return(nil),
// 					apm.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
// 						Pass: []types.PreflightsRecord{
// 							{
// 								Title:   "Test Check",
// 								Message: "Test check passed",
// 							},
// 						},
// 					}, nil),
// 				)
// 			},
// 			expectedErr: false,
// 		},
// 		{
// 			name: "successful execution from failed state with passing preflights",
// 			opts: RunAppPreflightOptions{
// 				ConfigValues: types.AppConfigValues{
// 					"test-item": types.AppConfigValue{Value: "test-value"},
// 				},
// 				PreflightBinaryPath: "/usr/bin/preflight",
// 			},
// 			currentState:  states.StateAppPreflightsFailed,
// 			expectedState: states.StateAppPreflightsSucceeded,
// 			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager) {
// 				mock.InOrder(
// 					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}).Return(expectedAPF, nil),
// 					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
// 						return expectedAPF == opts.AppPreflightSpec
// 					})).Return(nil),
// 					apm.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
// 						Pass: []types.PreflightsRecord{
// 							{
// 								Title:   "Test Check",
// 								Message: "Test check passed",
// 							},
// 						},
// 					}, nil),
// 				)
// 			},
// 			expectedErr: false,
// 		},
// 		{
// 			name: "successful execution with failing preflights",
// 			opts: RunAppPreflightOptions{
// 				ConfigValues: types.AppConfigValues{
// 					"test-item": types.AppConfigValue{Value: "test-value"},
// 				},
// 				PreflightBinaryPath: "/usr/bin/preflight",
// 			},
// 			currentState:  states.StateApplicationConfigured,
// 			expectedState: states.StateAppPreflightsFailed,
// 			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager) {
// 				mock.InOrder(
// 					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}).Return(expectedAPF, nil),
// 					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
// 						return expectedAPF == opts.AppPreflightSpec
// 					})).Return(nil),
// 					apm.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
// 						Fail: []types.PreflightsRecord{
// 							{
// 								Title:   "Test Check",
// 								Message: "Test check failed",
// 							},
// 						},
// 					}, nil),
// 				)
// 			},
// 			expectedErr: false,
// 		},
// 		{
// 			name: "execution succeeded but failed to get preflight output",
// 			opts: RunAppPreflightOptions{
// 				ConfigValues: types.AppConfigValues{
// 					"test-item": types.AppConfigValue{Value: "test-value"},
// 				},
// 				PreflightBinaryPath: "/usr/bin/preflight",
// 			},
// 			currentState:  states.StateApplicationConfigured,
// 			expectedState: states.StateAppPreflightsExecutionFailed,
// 			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager) {
// 				mock.InOrder(
// 					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}).Return(expectedAPF, nil),
// 					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
// 						return expectedAPF == opts.AppPreflightSpec
// 					})).Return(nil),
// 					apm.On("GetAppPreflightOutput", mock.Anything).Return(nil, errors.New("get output error")),
// 				)
// 			},
// 			expectedErr: false,
// 		},
// 		{
// 			name: "successful execution with nil preflight output",
// 			opts: RunAppPreflightOptions{
// 				ConfigValues: types.AppConfigValues{
// 					"test-item": types.AppConfigValue{Value: "test-value"},
// 				},
// 				PreflightBinaryPath: "/usr/bin/preflight",
// 			},
// 			currentState:  states.StateApplicationConfigured,
// 			expectedState: states.StateAppPreflightsSucceeded,
// 			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager) {
// 				mock.InOrder(
// 					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}).Return(expectedAPF, nil),
// 					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
// 						return expectedAPF == opts.AppPreflightSpec
// 					})).Return(nil),
// 					apm.On("GetAppPreflightOutput", mock.Anything).Return(nil, nil),
// 				)
// 			},
// 			expectedErr: false,
// 		},
// 		{
// 			name: "successful execution with preflight warnings",
// 			opts: RunAppPreflightOptions{
// 				ConfigValues: types.AppConfigValues{
// 					"test-item": types.AppConfigValue{Value: "test-value"},
// 				},
// 				PreflightBinaryPath: "/usr/bin/preflight",
// 			},
// 			currentState:  states.StateApplicationConfigured,
// 			expectedState: states.StateAppPreflightsSucceeded,
// 			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager) {
// 				mock.InOrder(
// 					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}).Return(expectedAPF, nil),
// 					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
// 						return expectedAPF == opts.AppPreflightSpec
// 					})).Return(nil),
// 					apm.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
// 						Warn: []types.PreflightsRecord{
// 							{
// 								Title:   "Test Check",
// 								Message: "Test check warning",
// 							},
// 						},
// 					}, nil),
// 				)
// 			},
// 			expectedErr: false,
// 		},
// 		{
// 			name: "failed to extract app preflight spec",
// 			opts: RunAppPreflightOptions{
// 				ConfigValues: types.AppConfigValues{
// 					"test-item": types.AppConfigValue{Value: "test-value"},
// 				},
// 				PreflightBinaryPath: "/usr/bin/preflight",
// 			},
// 			currentState:  states.StateApplicationConfigured,
// 			expectedState: states.StateApplicationConfigured,
// 			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager) {
// 				arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}).Return(nil, errors.New("extraction error"))
// 			},
// 			expectedErr: true,
// 		},
// 		{
// 			name: "preflight execution failed",
// 			opts: RunAppPreflightOptions{
// 				ConfigValues: types.AppConfigValues{
// 					"test-item": types.AppConfigValue{Value: "test-value"},
// 				},
// 				PreflightBinaryPath: "/usr/bin/preflight",
// 			},
// 			currentState:  states.StateApplicationConfigured,
// 			expectedState: states.StateAppPreflightsExecutionFailed,
// 			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager) {
// 				mock.InOrder(
// 					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}).Return(expectedAPF, nil),
// 					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
// 						return expectedAPF == opts.AppPreflightSpec
// 					})).Return(errors.New("run preflights error")),
// 				)
// 			},
// 			expectedErr: false,
// 		},
// 		{
// 			name: "invalid state transition",
// 			opts: RunAppPreflightOptions{
// 				ConfigValues: types.AppConfigValues{
// 					"test-item": types.AppConfigValue{Value: "test-value"},
// 				},
// 				PreflightBinaryPath: "/usr/bin/preflight",
// 			},
// 			currentState:  states.StateInfrastructureInstalling,
// 			expectedState: states.StateInfrastructureInstalling,
// 			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager) {
// 			},
// 			expectedErr: true,
// 		},
// 	}

// 	for _, tt := range tests {
// 		s.T().Run(tt.name, func(t *testing.T) {

// 			appConfigManager := &appconfig.MockAppConfigManager{}
// 			appPreflightManager := &apppreflightmanager.MockAppPreflightManager{}
// 			appReleaseManager := &appreleasemanager.MockAppReleaseManager{}
// 			sm := s.CreateStateMachine(tt.currentState)
// 			controller, err := NewInstallController(
// 				WithStateMachine(sm),
// 				WithAppConfigManager(appConfigManager),
// 				WithAppPreflightManager(appPreflightManager),
// 				WithAppReleaseManager(appReleaseManager),
// 			)
// 			require.NoError(t, err, "failed to create install controller")

// 			tt.setupMocks(appPreflightManager, appReleaseManager)
// 			err = controller.RunAppPreflights(t.Context(), tt.opts)

// 			if tt.expectedErr {
// 				assert.Error(t, err)
// 			} else {
// 				assert.NoError(t, err)
// 			}

// 			assert.Eventually(t, func() bool {
// 				return sm.CurrentState() == tt.expectedState
// 			}, 2*time.Second, 100*time.Millisecond, "state should be %s but is %s", tt.expectedState, sm.CurrentState())
// 			assert.False(t, sm.IsLockAcquired(), "state machine should not be locked after running app preflights")

// 			appPreflightManager.AssertExpectations(s.T())
// 			appReleaseManager.AssertExpectations(s.T())

// 		})
// 	}
// }
