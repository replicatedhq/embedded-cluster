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
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
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

func (s *AppInstallControllerTestSuite) TestRunAppPreflights() {
	expectedAPF := &troubleshootv1beta2.PreflightSpec{
		Collectors: []*troubleshootv1beta2.Collect{
			{
				ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
			},
		},
	}

	tests := []struct {
		name          string
		opts          RunAppPreflightOptions
		currentState  statemachine.State
		expectedState statemachine.State
		setupMocks    func(*apppreflightmanager.MockAppPreflightManager, *appreleasemanager.MockAppReleaseManager, *appconfig.MockAppConfigManager)
		expectedErr   bool
	}{
		{
			name: "successful execution with passing preflights",
			opts: RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateInfrastructureInstalled,
			expectedState: states.StateAppPreflightsSucceeded,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager) {
				mock.InOrder(
					acm.On("GetConfigValues").Return(types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, nil),
					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, mock.Anything, mock.Anything).Return(expectedAPF, nil),
					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
						return expectedAPF == opts.AppPreflightSpec
					})).Return(nil),
					apm.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
						Pass: []types.PreflightsRecord{
							{
								Title:   "Test Check",
								Message: "Test check passed",
							},
						},
					}, nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "successful execution from execution failed state with passing preflights",
			opts: RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateAppPreflightsExecutionFailed,
			expectedState: states.StateAppPreflightsSucceeded,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager) {
				mock.InOrder(
					acm.On("GetConfigValues").Return(types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, nil),
					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, mock.Anything, mock.Anything).Return(expectedAPF, nil),
					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
						return expectedAPF == opts.AppPreflightSpec
					})).Return(nil),
					apm.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
						Pass: []types.PreflightsRecord{
							{
								Title:   "Test Check",
								Message: "Test check passed",
							},
						},
					}, nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "successful execution from failed state with passing preflights",
			opts: RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateAppPreflightsFailed,
			expectedState: states.StateAppPreflightsSucceeded,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager) {
				mock.InOrder(
					acm.On("GetConfigValues").Return(types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, nil),
					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, mock.Anything, mock.Anything).Return(expectedAPF, nil),
					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
						return expectedAPF == opts.AppPreflightSpec
					})).Return(nil),
					apm.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
						Pass: []types.PreflightsRecord{
							{
								Title:   "Test Check",
								Message: "Test check passed",
							},
						},
					}, nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "successful execution with failing preflights",
			opts: RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateInfrastructureInstalled,
			expectedState: states.StateAppPreflightsFailed,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager) {
				mock.InOrder(
					acm.On("GetConfigValues").Return(types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, nil),
					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, mock.Anything, mock.Anything).Return(expectedAPF, nil),
					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
						return expectedAPF == opts.AppPreflightSpec
					})).Return(nil),
					apm.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
						Fail: []types.PreflightsRecord{
							{
								Title:   "Test Check",
								Message: "Test check failed",
							},
						},
					}, nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "execution succeeded but failed to get preflight output",
			opts: RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateInfrastructureInstalled,
			expectedState: states.StateAppPreflightsExecutionFailed,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager) {
				mock.InOrder(
					acm.On("GetConfigValues").Return(types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, nil),
					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, mock.Anything, mock.Anything).Return(expectedAPF, nil),
					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
						return expectedAPF == opts.AppPreflightSpec
					})).Return(nil),
					apm.On("GetAppPreflightOutput", mock.Anything).Return(nil, errors.New("get output error")),
				)
			},
			expectedErr: false,
		},
		{
			name: "successful execution with nil preflight output",
			opts: RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateInfrastructureInstalled,
			expectedState: states.StateAppPreflightsSucceeded,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager) {
				mock.InOrder(
					acm.On("GetConfigValues").Return(types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, nil),
					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, mock.Anything, mock.Anything).Return(expectedAPF, nil),
					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
						return expectedAPF == opts.AppPreflightSpec
					})).Return(nil),
					apm.On("GetAppPreflightOutput", mock.Anything).Return(nil, nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "successful execution with preflight warnings",
			opts: RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateInfrastructureInstalled,
			expectedState: states.StateAppPreflightsSucceeded,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager) {
				mock.InOrder(
					acm.On("GetConfigValues").Return(types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, nil),
					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, mock.Anything, mock.Anything).Return(expectedAPF, nil),
					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
						return expectedAPF == opts.AppPreflightSpec
					})).Return(nil),
					apm.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
						Warn: []types.PreflightsRecord{
							{
								Title:   "Test Check",
								Message: "Test check warning",
							},
						},
					}, nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "failed to extract app preflight spec",
			opts: RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateInfrastructureInstalled,
			expectedState: states.StateInfrastructureInstalled,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager) {
				mock.InOrder(
					acm.On("GetConfigValues").Return(types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, nil),
					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, mock.Anything, mock.Anything).Return(nil, errors.New("extraction error")),
				)
			},
			expectedErr: true,
		},
		{
			name: "preflight execution failed",
			opts: RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateInfrastructureInstalled,
			expectedState: states.StateAppPreflightsExecutionFailed,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager) {
				mock.InOrder(
					acm.On("GetConfigValues").Return(types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, nil),
					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, mock.Anything, mock.Anything).Return(expectedAPF, nil),
					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
						return expectedAPF == opts.AppPreflightSpec
					})).Return(errors.New("run preflights error")),
				)
			},
			expectedErr: false,
		},
		{
			name: "invalid state transition",
			opts: RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateInfrastructureInstalling,
			expectedState: states.StateInfrastructureInstalling,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager) {
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {

			appConfigManager := &appconfig.MockAppConfigManager{}
			appPreflightManager := &apppreflightmanager.MockAppPreflightManager{}
			appReleaseManager := &appreleasemanager.MockAppReleaseManager{}
			sm := s.CreateStateMachine(tt.currentState)
			controller, err := NewInstallController(
				WithStateMachine(sm),
				WithAppConfigManager(appConfigManager),
				WithAppPreflightManager(appPreflightManager),
				WithAppReleaseManager(appReleaseManager),
				WithStore(&store.MockStore{}),
				WithReleaseData(&release.ReleaseData{}),
			)
			require.NoError(t, err, "failed to create install controller")

			tt.setupMocks(appPreflightManager, appReleaseManager, appConfigManager)
			err = controller.RunAppPreflights(t.Context(), tt.opts)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Eventually(t, func() bool {
				return sm.CurrentState() == tt.expectedState
			}, 2*time.Second, 100*time.Millisecond, "state should be %s but is %s", tt.expectedState, sm.CurrentState())
			assert.False(t, sm.IsLockAcquired(), "state machine should not be locked after running app preflights")

			appPreflightManager.AssertExpectations(s.T())
			appReleaseManager.AssertExpectations(s.T())
			appConfigManager.AssertExpectations(s.T())

		})
	}
}

func (s *AppInstallControllerTestSuite) TestGetAppInstallStatus() {
	expectedAppInstall := types.AppInstall{
		Status: types.Status{
			State:       types.StateRunning,
			Description: "Installing application",
			LastUpdated: time.Now(),
		},
		Logs: "Installation logs\n",
	}

	tests := []struct {
		name        string
		setupMocks  func(*appinstallmanager.MockAppInstallManager)
		expectedErr bool
	}{
		{
			name: "successful status retrieval",
			setupMocks: func(aim *appinstallmanager.MockAppInstallManager) {
				aim.On("GetStatus").Return(expectedAppInstall, nil)
			},
			expectedErr: false,
		},
		{
			name: "manager returns error",
			setupMocks: func(aim *appinstallmanager.MockAppInstallManager) {
				aim.On("GetStatus").Return(types.AppInstall{}, errors.New("status error"))
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
			sm := s.CreateStateMachine(states.StateNew)

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

			tt.setupMocks(appInstallManager)
			result, err := controller.GetAppInstallStatus(t.Context())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Equal(t, types.AppInstall{}, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, expectedAppInstall, result)
			}

			appInstallManager.AssertExpectations(s.T())
		})
	}
}

func (s *AppInstallControllerTestSuite) TestInstallApp() {
	tests := []struct {
		name                string
		ignoreAppPreflights bool
		proxySpec           *ecv1beta1.ProxySpec
		currentState        statemachine.State
		expectedState       statemachine.State
		setupMocks          func(*appconfig.MockAppConfigManager, *appreleasemanager.MockAppReleaseManager, *appinstallmanager.MockAppInstallManager)
		expectedErr         bool
	}{
		{
			name:          "invalid state transition from succeeded state",
			currentState:  states.StateSucceeded,
			expectedState: states.StateSucceeded,
			setupMocks: func(acm *appconfig.MockAppConfigManager, arm *appreleasemanager.MockAppReleaseManager, aim *appinstallmanager.MockAppInstallManager) {
				// No mocks needed for invalid state transition
			},
			expectedErr: true,
		},
		{
			name:          "invalid state transition from infrastructure installing state",
			currentState:  states.StateInfrastructureInstalling,
			expectedState: states.StateInfrastructureInstalling,
			setupMocks: func(acm *appconfig.MockAppConfigManager, arm *appreleasemanager.MockAppReleaseManager, aim *appinstallmanager.MockAppInstallManager) {
				// No mocks needed for invalid state transition
			},
			expectedErr: true,
		},
		{
			name:          "successful app installation from app preflights succeeded state with helm charts",
			currentState:  states.StateAppPreflightsSucceeded,
			expectedState: states.StateSucceeded,
			setupMocks: func(acm *appconfig.MockAppConfigManager, arm *appreleasemanager.MockAppReleaseManager, aim *appinstallmanager.MockAppInstallManager) {
				configValues := kotsv1beta1.ConfigValues{
					Spec: kotsv1beta1.ConfigValuesSpec{
						Values: map[string]kotsv1beta1.ConfigValue{
							"test-key": {Value: "test-value"},
						},
					},
				}
				expectedCharts := []types.InstallableHelmChart{
					{
						Archive: []byte("chart-archive-data"),
						Values:  map[string]any{"key": "value"},
					},
				}
				appConfigValues := types.AppConfigValues{
					"test-key": types.AppConfigValue{Value: "test-value"},
				}
				mock.InOrder(
					acm.On("GetConfigValues").Return(appConfigValues, nil),
					acm.On("GetKotsadmConfigValues").Return(configValues, nil),
					arm.On("ExtractInstallableHelmCharts", mock.Anything, appConfigValues, mock.AnythingOfType("*v1beta1.ProxySpec")).Return(expectedCharts, nil),
					aim.On("Install", mock.Anything, expectedCharts, configValues).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "successful app installation from app preflights failed bypassed state",
			currentState:  states.StateAppPreflightsFailedBypassed,
			expectedState: states.StateSucceeded,
			setupMocks: func(acm *appconfig.MockAppConfigManager, arm *appreleasemanager.MockAppReleaseManager, aim *appinstallmanager.MockAppInstallManager) {
				configValues := kotsv1beta1.ConfigValues{
					Spec: kotsv1beta1.ConfigValuesSpec{
						Values: map[string]kotsv1beta1.ConfigValue{
							"test-key": {Value: "test-value"},
						},
					},
				}
				appConfigValues := types.AppConfigValues{
					"test-key": types.AppConfigValue{Value: "test-value"},
				}
				mock.InOrder(
					acm.On("GetConfigValues").Return(appConfigValues, nil),
					acm.On("GetKotsadmConfigValues").Return(configValues, nil),
					arm.On("ExtractInstallableHelmCharts", mock.Anything, appConfigValues, mock.AnythingOfType("*v1beta1.ProxySpec")).Return([]types.InstallableHelmChart{}, nil),
					aim.On("Install", mock.Anything, []types.InstallableHelmChart{}, configValues).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "get config values error",
			currentState:  states.StateAppPreflightsSucceeded,
			expectedState: states.StateAppPreflightsSucceeded,
			setupMocks: func(acm *appconfig.MockAppConfigManager, arm *appreleasemanager.MockAppReleaseManager, aim *appinstallmanager.MockAppInstallManager) {
				appConfigValues := types.AppConfigValues{
					"test-key": types.AppConfigValue{Value: "test-value"},
				}
				acm.On("GetConfigValues").Return(appConfigValues, nil)
				acm.On("GetKotsadmConfigValues").Return(kotsv1beta1.ConfigValues{}, errors.New("config values error"))
			},
			expectedErr: true,
		},
		{
			name:                "successful app installation with failed preflights - ignored",
			ignoreAppPreflights: true,
			currentState:        states.StateAppPreflightsFailed,
			expectedState:       states.StateSucceeded,
			setupMocks: func(acm *appconfig.MockAppConfigManager, arm *appreleasemanager.MockAppReleaseManager, aim *appinstallmanager.MockAppInstallManager) {
				configValues := kotsv1beta1.ConfigValues{
					Spec: kotsv1beta1.ConfigValuesSpec{
						Values: map[string]kotsv1beta1.ConfigValue{
							"test-key": {Value: "test-value"},
						},
					},
				}
				appConfigValues := types.AppConfigValues{
					"test-key": types.AppConfigValue{Value: "test-value"},
				}
				mock.InOrder(
					acm.On("GetConfigValues").Return(appConfigValues, nil),
					acm.On("GetKotsadmConfigValues").Return(configValues, nil),
					arm.On("ExtractInstallableHelmCharts", mock.Anything, appConfigValues, mock.AnythingOfType("*v1beta1.ProxySpec")).Return([]types.InstallableHelmChart{}, nil),
					aim.On("Install", mock.Anything, []types.InstallableHelmChart{}, configValues).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:                "failed app installation with failed preflights - not ignored",
			ignoreAppPreflights: false,
			currentState:        states.StateAppPreflightsFailed,
			expectedState:       states.StateAppPreflightsFailed,
			setupMocks: func(acm *appconfig.MockAppConfigManager, arm *appreleasemanager.MockAppReleaseManager, aim *appinstallmanager.MockAppInstallManager) {
				// No mocks needed as method should return early with error
			},
			expectedErr: true,
		},
		{
			name:          "successful app installation with proxy spec passed to helm chart extraction",
			currentState:  states.StateAppPreflightsSucceeded,
			expectedState: states.StateSucceeded,
			proxySpec: &ecv1beta1.ProxySpec{
				HTTPProxy:  "http://proxy.example.com:8080",
				HTTPSProxy: "https://proxy.example.com:8080",
				NoProxy:    "localhost,127.0.0.1",
			},
			setupMocks: func(acm *appconfig.MockAppConfigManager, arm *appreleasemanager.MockAppReleaseManager, aim *appinstallmanager.MockAppInstallManager) {
				configValues := kotsv1beta1.ConfigValues{
					Spec: kotsv1beta1.ConfigValuesSpec{
						Values: map[string]kotsv1beta1.ConfigValue{
							"test-key": {Value: "test-value"},
						},
					},
				}
				appConfigValues := types.AppConfigValues{
					"test-key": types.AppConfigValue{Value: "test-value"},
				}
				expectedCharts := []types.InstallableHelmChart{
					{
						Archive: []byte("chart-with-proxy-template"),
						Values:  map[string]any{"proxy_url": "http://proxy.example.com:8080"},
					},
				}
				mock.InOrder(
					acm.On("GetConfigValues").Return(appConfigValues, nil),
					acm.On("GetKotsadmConfigValues").Return(configValues, nil),
					arm.On("ExtractInstallableHelmCharts", mock.Anything, appConfigValues, mock.MatchedBy(func(proxySpec *ecv1beta1.ProxySpec) bool {
						return proxySpec != nil
					})).Return(expectedCharts, nil),
					aim.On("Install", mock.Anything, expectedCharts, configValues).Return(nil),
				)
			},
			expectedErr: false,
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

			tt.setupMocks(appConfigManager, appReleaseManager, appInstallManager)
			err = controller.InstallApp(t.Context(), tt.ignoreAppPreflights, tt.proxySpec)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Wait for the goroutine to complete and state to transition
			assert.Eventually(t, func() bool {
				return sm.CurrentState() == tt.expectedState
			}, 2*time.Second, 100*time.Millisecond, "state should be %s but is %s", tt.expectedState, sm.CurrentState())
			assert.False(t, sm.IsLockAcquired(), "state machine should not be locked after app installation")

			appConfigManager.AssertExpectations(s.T())
			appReleaseManager.AssertExpectations(s.T())
			appInstallManager.AssertExpectations(s.T())
		})
	}
}
