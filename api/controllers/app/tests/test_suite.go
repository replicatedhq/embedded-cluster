package tests

import (
	"errors"
	"strings"
	"testing"
	"time"

	appcontroller "github.com/replicatedhq/embedded-cluster/api/controllers/app"
	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	appinstallmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/install"
	apppreflightmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/preflight"
	appreleasemanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/release"
	appupgrademanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/upgrade"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AppControllerTestSuite struct {
	suite.Suite
	InstallType               string
	CreateInstallStateMachine func(initialState statemachine.State) statemachine.Interface
	CreateUpgradeStateMachine func(initialState statemachine.State) statemachine.Interface
}

func (s *AppControllerTestSuite) TestPatchAppConfigValues() {
	scenarios := []struct {
		name     string
		scenario string
		createSM func(initialState statemachine.State) statemachine.Interface
	}{
		{
			name:     "install",
			scenario: "install",
			createSM: s.CreateInstallStateMachine,
		},
		{
			name:     "upgrade",
			scenario: "upgrade",
			createSM: s.CreateUpgradeStateMachine,
		},
	}

	for _, scenario := range scenarios {
		s.T().Run(scenario.name, func(t *testing.T) {
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
				t.Run(tt.name, func(t *testing.T) {

					appPreflightManager := &apppreflightmanager.MockAppPreflightManager{}
					appReleaseManager := &appreleasemanager.MockAppReleaseManager{}
					appInstallManager := &appinstallmanager.MockAppInstallManager{}
					sm := scenario.createSM(tt.currentState)

					appConfigManager := &appconfig.MockAppConfigManager{}
					appConfigManager.On("TemplateConfig", types.AppConfigValues{}, false, false).Return(types.AppConfig{}, nil)

					controller, err := appcontroller.NewAppController(
						appcontroller.WithStateMachine(sm),
						appcontroller.WithAppConfigManager(appConfigManager),
						appcontroller.WithAppPreflightManager(appPreflightManager),
						appcontroller.WithAppReleaseManager(appReleaseManager),
						appcontroller.WithAppInstallManager(appInstallManager),
						appcontroller.WithStore(&store.MockStore{}),
						appcontroller.WithReleaseData(&release.ReleaseData{}),
					)
					require.NoError(t, err, "failed to create app controller")

					tt.setupMocks(appConfigManager)
					err = controller.PatchAppConfigValues(t.Context(), tt.values)

					if tt.expectedErr {
						assert.Error(t, err)
					} else {
						assert.NoError(t, err)
					}

					// Wait for the goroutine to complete and state to transition
					assert.Eventually(t, func() bool {
						return sm.CurrentState() == tt.expectedState
					}, time.Second, 100*time.Millisecond, "state should be %s but is %s", tt.expectedState, sm.CurrentState())

					assert.Eventually(t, func() bool {
						return !sm.IsLockAcquired()
					}, time.Second, 100*time.Millisecond, "state machine should not be locked")

					appConfigManager.AssertExpectations(t)

				})
			}
		})
	}
}

func (s *AppControllerTestSuite) TestRunAppPreflights() {
	expectedAPF := &troubleshootv1beta2.PreflightSpec{
		Collectors: []*troubleshootv1beta2.Collect{
			{
				ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
			},
		},
	}

	tests := []struct {
		name          string
		opts          appcontroller.RunAppPreflightOptions
		currentState  statemachine.State
		expectedState statemachine.State
		setupMocks    func(*apppreflightmanager.MockAppPreflightManager, *appreleasemanager.MockAppReleaseManager, *appconfig.MockAppConfigManager, *store.MockStore)
		expectedErr   bool
	}{
		{
			name: "successful execution with passing preflights",
			opts: appcontroller.RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateInfrastructureInstalled,
			expectedState: states.StateAppPreflightsSucceeded,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager, store *store.MockStore) {
				mock.InOrder(
					acm.On("GetConfigValues").Return(types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, nil),

					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, mock.Anything, mock.Anything).Return(expectedAPF, nil),

					store.AppPreflightMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateRunning
					})).Return(nil),

					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
						return expectedAPF == opts.AppPreflightSpec
					})).Return(&types.PreflightsOutput{
						Pass: []types.PreflightsRecord{
							{
								Title:   "Test Check",
								Message: "Test check passed",
							},
						},
					}, nil),

					store.AppPreflightMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateSucceeded
					})).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "successful execution from execution failed state with passing preflights",
			opts: appcontroller.RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateAppPreflightsExecutionFailed,
			expectedState: states.StateAppPreflightsSucceeded,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager, store *store.MockStore) {
				mock.InOrder(
					acm.On("GetConfigValues").Return(types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, nil),

					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, mock.Anything, mock.Anything).Return(expectedAPF, nil),

					store.AppPreflightMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateRunning
					})).Return(nil),

					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
						return expectedAPF == opts.AppPreflightSpec
					})).Return(&types.PreflightsOutput{
						Pass: []types.PreflightsRecord{
							{
								Title:   "Test Check",
								Message: "Test check passed",
							},
						},
					}, nil),

					store.AppPreflightMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateSucceeded
					})).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "successful execution from failed state with passing preflights",
			opts: appcontroller.RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateAppPreflightsFailed,
			expectedState: states.StateAppPreflightsSucceeded,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager, store *store.MockStore) {
				mock.InOrder(
					acm.On("GetConfigValues").Return(types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, nil),

					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, mock.Anything, mock.Anything).Return(expectedAPF, nil),

					store.AppPreflightMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateRunning
					})).Return(nil),

					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
						return expectedAPF == opts.AppPreflightSpec
					})).Return(&types.PreflightsOutput{
						Pass: []types.PreflightsRecord{
							{
								Title:   "Test Check",
								Message: "Test check passed",
							},
						},
					}, nil),

					store.AppPreflightMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateSucceeded
					})).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "successful execution with failing preflights",
			opts: appcontroller.RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateInfrastructureInstalled,
			expectedState: states.StateAppPreflightsFailed,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager, store *store.MockStore) {
				mock.InOrder(
					acm.On("GetConfigValues").Return(types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, nil),

					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, mock.Anything, mock.Anything).Return(expectedAPF, nil),

					store.AppPreflightMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateRunning
					})).Return(nil),

					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
						return expectedAPF == opts.AppPreflightSpec
					})).Return(&types.PreflightsOutput{
						Fail: []types.PreflightsRecord{
							{
								Title:   "Test Check",
								Message: "Test check failed",
							},
						},
					}, nil),

					store.AppPreflightMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateFailed
					})).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "failed to extract app preflight spec",
			opts: appcontroller.RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateInfrastructureInstalled,
			expectedState: states.StateInfrastructureInstalled,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager, store *store.MockStore) {
				mock.InOrder(
					acm.On("GetConfigValues").Return(types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, nil),

					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, mock.Anything, mock.Anything).Return(nil, errors.New("extraction error")),
				)
			},
			expectedErr: true,
		},
		{
			name: "preflight execution failed",
			opts: appcontroller.RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateInfrastructureInstalled,
			expectedState: states.StateAppPreflightsExecutionFailed,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager, store *store.MockStore) {
				mock.InOrder(
					acm.On("GetConfigValues").Return(types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, nil),

					arm.On("ExtractAppPreflightSpec", mock.Anything, types.AppConfigValues{"test-item": types.AppConfigValue{Value: "test-value"}}, mock.Anything, mock.Anything).Return(expectedAPF, nil),

					store.AppPreflightMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateRunning
					})).Return(nil),

					apm.On("RunAppPreflights", mock.Anything, mock.MatchedBy(func(opts apppreflightmanager.RunAppPreflightOptions) bool {
						return expectedAPF == opts.AppPreflightSpec
					})).Return(nil, errors.New("run preflights error")),

					store.AppPreflightMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateFailed && strings.Contains(status.Description, "run preflights error")
					})).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "invalid state transition",
			opts: appcontroller.RunAppPreflightOptions{
				PreflightBinaryPath: "/usr/bin/preflight",
			},
			currentState:  states.StateInfrastructureInstalling,
			expectedState: states.StateInfrastructureInstalling,
			setupMocks: func(apm *apppreflightmanager.MockAppPreflightManager, arm *appreleasemanager.MockAppReleaseManager, acm *appconfig.MockAppConfigManager, store *store.MockStore) {
				// No mocks needed for invalid state transition
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {

			appPreflightManager := &apppreflightmanager.MockAppPreflightManager{}
			appReleaseManager := &appreleasemanager.MockAppReleaseManager{}
			sm := s.CreateInstallStateMachine(tt.currentState)

			appConfigManager := &appconfig.MockAppConfigManager{}
			appConfigManager.On("TemplateConfig", types.AppConfigValues{}, false, false).Return(types.AppConfig{}, nil)

			store := &store.MockStore{}

			controller, err := appcontroller.NewAppController(
				appcontroller.WithStateMachine(sm),
				appcontroller.WithAppConfigManager(appConfigManager),
				appcontroller.WithAppPreflightManager(appPreflightManager),
				appcontroller.WithAppReleaseManager(appReleaseManager),
				appcontroller.WithStore(store),
				appcontroller.WithReleaseData(&release.ReleaseData{}),
			)
			require.NoError(t, err, "failed to create install controller")

			tt.setupMocks(appPreflightManager, appReleaseManager, appConfigManager, store)
			err = controller.RunAppPreflights(t.Context(), tt.opts)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Wait for the goroutine to complete and state to transition
			assert.Eventually(t, func() bool {
				return sm.CurrentState() == tt.expectedState
			}, 2*time.Second, 100*time.Millisecond, "state should be %s but is %s", tt.expectedState, sm.CurrentState())

			assert.Eventually(t, func() bool {
				return !sm.IsLockAcquired()
			}, 2*time.Second, 100*time.Millisecond, "state machine should not be locked")

			appPreflightManager.AssertExpectations(s.T())
			appReleaseManager.AssertExpectations(s.T())
			appConfigManager.AssertExpectations(s.T())

		})
	}
}

func (s *AppControllerTestSuite) TestInstallApp() {
	tests := []struct {
		name                string
		ignoreAppPreflights bool
		currentState        statemachine.State
		expectedState       statemachine.State
		setupMocks          func(*appconfig.MockAppConfigManager, *appinstallmanager.MockAppInstallManager, *apppreflightmanager.MockAppPreflightManager, *store.MockStore)
		expectedErr         bool
	}{
		{
			name:          "invalid state transition from succeeded state",
			currentState:  states.StateSucceeded,
			expectedState: states.StateSucceeded,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aim *appinstallmanager.MockAppInstallManager, apm *apppreflightmanager.MockAppPreflightManager, store *store.MockStore) {
				// No mocks needed for invalid state transition
			},
			expectedErr: true,
		},
		{
			name:          "invalid state transition from infrastructure installing state",
			currentState:  states.StateInfrastructureInstalling,
			expectedState: states.StateInfrastructureInstalling,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aim *appinstallmanager.MockAppInstallManager, apm *apppreflightmanager.MockAppPreflightManager, store *store.MockStore) {
				// No mocks needed for invalid state transition
			},
			expectedErr: true,
		},
		{
			name:          "successful app installation from app preflights succeeded state",
			currentState:  states.StateAppPreflightsSucceeded,
			expectedState: states.StateSucceeded,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aim *appinstallmanager.MockAppInstallManager, apm *apppreflightmanager.MockAppPreflightManager, store *store.MockStore) {
				mock.InOrder(
					acm.On("GetKotsadmConfigValues").Return(kotsv1beta1.ConfigValues{
						Spec: kotsv1beta1.ConfigValuesSpec{
							Values: map[string]kotsv1beta1.ConfigValue{
								"test-key": {Value: "test-value"},
							},
						},
					}, nil),

					store.AppInstallMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateRunning
					})).Return(nil),

					aim.On("Install", mock.Anything, mock.MatchedBy(func(cv kotsv1beta1.ConfigValues) bool {
						return cv.Spec.Values["test-key"].Value == "test-value"
					}), mock.Anything).Return(nil),

					store.AppInstallMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateSucceeded
					})).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "successful app installation from app preflights failed bypassed state",
			currentState:  states.StateAppPreflightsFailedBypassed,
			expectedState: states.StateSucceeded,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aim *appinstallmanager.MockAppInstallManager, apm *apppreflightmanager.MockAppPreflightManager, store *store.MockStore) {
				mock.InOrder(
					acm.On("GetKotsadmConfigValues").Return(kotsv1beta1.ConfigValues{
						Spec: kotsv1beta1.ConfigValuesSpec{
							Values: map[string]kotsv1beta1.ConfigValue{
								"test-key": {Value: "test-value"},
							},
						},
					}, nil),

					store.AppInstallMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateRunning
					})).Return(nil),

					aim.On("Install", mock.Anything, mock.MatchedBy(func(cv kotsv1beta1.ConfigValues) bool {
						return cv.Spec.Values["test-key"].Value == "test-value"
					}), mock.Anything).Return(nil),

					store.AppInstallMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateSucceeded
					})).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "failed app installation from app preflights succeeded state",
			currentState:  states.StateAppPreflightsSucceeded,
			expectedState: states.StateAppInstallFailed,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aim *appinstallmanager.MockAppInstallManager, apm *apppreflightmanager.MockAppPreflightManager, store *store.MockStore) {
				mock.InOrder(
					acm.On("GetKotsadmConfigValues").Return(kotsv1beta1.ConfigValues{
						Spec: kotsv1beta1.ConfigValuesSpec{
							Values: map[string]kotsv1beta1.ConfigValue{
								"test-key": {Value: "test-value"},
							},
						},
					}, nil),

					store.AppInstallMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateRunning
					})).Return(nil),

					aim.On("Install", mock.Anything, mock.MatchedBy(func(cv kotsv1beta1.ConfigValues) bool {
						return cv.Spec.Values["test-key"].Value == "test-value"
					}), mock.Anything).Return(errors.New("install error")),

					store.AppInstallMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateFailed && strings.Contains(status.Description, "install error")
					})).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "get config values error",
			currentState:  states.StateAppPreflightsSucceeded,
			expectedState: states.StateAppPreflightsSucceeded,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aim *appinstallmanager.MockAppInstallManager, apm *apppreflightmanager.MockAppPreflightManager, store *store.MockStore) {
				mock.InOrder(
					acm.On("GetKotsadmConfigValues").Return(kotsv1beta1.ConfigValues{}, errors.New("config values error")),
				)
			},
			expectedErr: true,
		},
		{
			name:                "successful app installation with failed preflights - ignored",
			ignoreAppPreflights: true,
			currentState:        states.StateAppPreflightsFailed,
			expectedState:       states.StateSucceeded,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aim *appinstallmanager.MockAppInstallManager, apm *apppreflightmanager.MockAppPreflightManager, store *store.MockStore) {
				mock.InOrder(
					// Mock GetAppPreflightOutput to return non-strict failures (can be bypassed)
					apm.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
						Fail: []types.PreflightsRecord{
							{
								Title:   "Non-strict preflight failure",
								Message: "This is a non-strict failure",
								Strict:  false, // This allows bypass
							},
						},
					}, nil),

					acm.On("GetKotsadmConfigValues").Return(kotsv1beta1.ConfigValues{
						Spec: kotsv1beta1.ConfigValuesSpec{
							Values: map[string]kotsv1beta1.ConfigValue{
								"test-key": {Value: "test-value"},
							},
						},
					}, nil),

					store.AppInstallMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateRunning
					})).Return(nil),

					aim.On("Install", mock.Anything, mock.MatchedBy(func(cv kotsv1beta1.ConfigValues) bool {
						return cv.Spec.Values["test-key"].Value == "test-value"
					}), mock.Anything).Return(nil),

					store.AppInstallMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateSucceeded
					})).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:                "failed app installation with failed preflights - not ignored",
			ignoreAppPreflights: false,
			currentState:        states.StateAppPreflightsFailed,
			expectedState:       states.StateAppPreflightsFailed,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aim *appinstallmanager.MockAppInstallManager, apm *apppreflightmanager.MockAppPreflightManager, store *store.MockStore) {
				mock.InOrder(
					// Mock GetAppPreflightOutput to return non-strict failures (method should be called but bypass denied)
					apm.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
						Fail: []types.PreflightsRecord{
							{
								Title:   "Non-strict preflight failure",
								Message: "This is a non-strict failure",
								Strict:  false, // Non-strict but bypass still denied due to ignoreAppPreflights=false
							},
						},
					}, nil),
				)
			},
			expectedErr: true,
		},
		{
			name:                "strict app preflight bypass blocked",
			ignoreAppPreflights: true,
			currentState:        states.StateAppPreflightsFailed,
			expectedState:       states.StateAppPreflightsFailed,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aim *appinstallmanager.MockAppInstallManager, apm *apppreflightmanager.MockAppPreflightManager, store *store.MockStore) {
				mock.InOrder(
					// Mock GetAppPreflightOutput to return strict failures (cannot be bypassed)
					apm.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
						Fail: []types.PreflightsRecord{
							{
								Title:   "Strict preflight failure",
								Message: "This is a strict failure that cannot be bypassed",
								Strict:  true, // Strict failure - cannot be bypassed
							},
						},
					}, nil),
				)
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			appPreflightManager := &apppreflightmanager.MockAppPreflightManager{}
			appReleaseManager := &appreleasemanager.MockAppReleaseManager{}
			appInstallManager := &appinstallmanager.MockAppInstallManager{}
			sm := s.CreateInstallStateMachine(tt.currentState)

			appConfigManager := &appconfig.MockAppConfigManager{}
			appConfigManager.On("TemplateConfig", types.AppConfigValues{}, false, false).Return(types.AppConfig{}, nil)

			store := &store.MockStore{}

			controller, err := appcontroller.NewAppController(
				appcontroller.WithStateMachine(sm),
				appcontroller.WithAppConfigManager(appConfigManager),
				appcontroller.WithAppPreflightManager(appPreflightManager),
				appcontroller.WithAppReleaseManager(appReleaseManager),
				appcontroller.WithAppInstallManager(appInstallManager),
				appcontroller.WithStore(store),
				appcontroller.WithReleaseData(&release.ReleaseData{}),
			)
			require.NoError(t, err, "failed to create install controller")

			tt.setupMocks(appConfigManager, appInstallManager, appPreflightManager, store)
			err = controller.InstallApp(t.Context(), tt.ignoreAppPreflights)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Wait for the goroutine to complete and state to transition
			assert.Eventually(t, func() bool {
				return sm.CurrentState() == tt.expectedState
			}, 2*time.Second, 100*time.Millisecond, "state should be %s but is %s", tt.expectedState, sm.CurrentState())

			assert.Eventually(t, func() bool {
				return !sm.IsLockAcquired()
			}, 2*time.Second, 100*time.Millisecond, "state machine should not be locked")

			appConfigManager.AssertExpectations(s.T())
			appInstallManager.AssertExpectations(s.T())
		})
	}
}

func (s *AppControllerTestSuite) TestUpgradeApp() {
	tests := []struct {
		name                string
		ignoreAppPreflights bool
		currentState        statemachine.State
		expectedState       statemachine.State
		setupMocks          func(*appconfig.MockAppConfigManager, *appupgrademanager.MockAppUpgradeManager)
		expectedErr         bool
	}{
		{
			name:          "invalid state transition from succeeded state",
			currentState:  states.StateSucceeded,
			expectedState: states.StateSucceeded,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aum *appupgrademanager.MockAppUpgradeManager) {
				// No mocks needed for invalid state transition
			},
			expectedErr: true,
		},
		{
			name:          "invalid state transition from app upgrading state",
			currentState:  states.StateAppUpgrading,
			expectedState: states.StateAppUpgrading,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aum *appupgrademanager.MockAppUpgradeManager) {
				// No mocks needed for invalid state transition
			},
			expectedErr: true,
		},
		{
			name:          "invalid state transition from app upgrade failed state",
			currentState:  states.StateAppUpgradeFailed,
			expectedState: states.StateAppUpgradeFailed,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aum *appupgrademanager.MockAppUpgradeManager) {
				// No mocks needed for invalid state transition
			},
			expectedErr: true,
		},
		{
			name:          "invalid state transition from app configured state",
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateApplicationConfigured,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aum *appupgrademanager.MockAppUpgradeManager) {
				// No mocks needed for invalid state transition
			},
			expectedErr: true,
		},
		{
			name:          "successful app upgrade from app preflights succeeded state",
			currentState:  states.StateAppPreflightsSucceeded,
			expectedState: states.StateSucceeded,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aum *appupgrademanager.MockAppUpgradeManager) {
				mock.InOrder(
					acm.On("GetKotsadmConfigValues").Return(kotsv1beta1.ConfigValues{
						Spec: kotsv1beta1.ConfigValuesSpec{
							Values: map[string]kotsv1beta1.ConfigValue{
								"test-key": {Value: "test-value"},
							},
						},
					}, nil),
					aum.On("Upgrade", mock.Anything, mock.MatchedBy(func(cv kotsv1beta1.ConfigValues) bool {
						return cv.Spec.Values["test-key"].Value == "test-value"
					})).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "get config values error",
			currentState:  states.StateAppPreflightsSucceeded,
			expectedState: states.StateAppPreflightsSucceeded,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aum *appupgrademanager.MockAppUpgradeManager) {
				acm.On("GetKotsadmConfigValues").Return(kotsv1beta1.ConfigValues{}, errors.New("config values error"))
			},
			expectedErr: true,
		},
		{
			name:          "app upgrade manager error",
			currentState:  states.StateAppPreflightsSucceeded,
			expectedState: states.StateAppUpgradeFailed,
			setupMocks: func(acm *appconfig.MockAppConfigManager, aum *appupgrademanager.MockAppUpgradeManager) {
				mock.InOrder(
					acm.On("GetKotsadmConfigValues").Return(kotsv1beta1.ConfigValues{
						Spec: kotsv1beta1.ConfigValuesSpec{
							Values: map[string]kotsv1beta1.ConfigValue{
								"test-key": {Value: "test-value"},
							},
						},
					}, nil),
					aum.On("Upgrade", mock.Anything, mock.MatchedBy(func(cv kotsv1beta1.ConfigValues) bool {
						return cv.Spec.Values["test-key"].Value == "test-value"
					})).Return(errors.New("upgrade error")),
				)
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			appPreflightManager := &apppreflightmanager.MockAppPreflightManager{}
			appReleaseManager := &appreleasemanager.MockAppReleaseManager{}
			appInstallManager := &appinstallmanager.MockAppInstallManager{}
			appUpgradeManager := &appupgrademanager.MockAppUpgradeManager{}
			sm := s.CreateUpgradeStateMachine(tt.currentState)

			appConfigManager := &appconfig.MockAppConfigManager{}
			appConfigManager.On("TemplateConfig", types.AppConfigValues{}, false, false).Return(types.AppConfig{}, nil)

			controller, err := appcontroller.NewAppController(
				appcontroller.WithStateMachine(sm),
				appcontroller.WithAppConfigManager(appConfigManager),
				appcontroller.WithAppPreflightManager(appPreflightManager),
				appcontroller.WithAppReleaseManager(appReleaseManager),
				appcontroller.WithAppInstallManager(appInstallManager),
				appcontroller.WithAppUpgradeManager(appUpgradeManager),
				appcontroller.WithStore(&store.MockStore{}),
				appcontroller.WithReleaseData(&release.ReleaseData{}),
			)
			require.NoError(t, err, "failed to create app controller")

			tt.setupMocks(appConfigManager, appUpgradeManager)
			err = controller.UpgradeApp(t.Context(), tt.ignoreAppPreflights)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Wait for the goroutine to complete and state to transition
			assert.Eventually(t, func() bool {
				return sm.CurrentState() == tt.expectedState
			}, 2*time.Second, 100*time.Millisecond, "state should be %s but is %s", tt.expectedState, sm.CurrentState())

			assert.Eventually(t, func() bool {
				return !sm.IsLockAcquired()
			}, 2*time.Second, 100*time.Millisecond, "state machine should not be locked")

			appConfigManager.AssertExpectations(t)
			appUpgradeManager.AssertExpectations(t)
		})
	}
}

func (s *AppControllerTestSuite) TestGetAppUpgradeStatus() {
	expectedAppUpgrade := types.AppUpgrade{
		Status: types.Status{
			State:       types.StateRunning,
			Description: "Upgrading application",
			LastUpdated: time.Now(),
		},
		Logs: "Upgrade logs\n",
	}

	tests := []struct {
		name        string
		setupMocks  func(*appupgrademanager.MockAppUpgradeManager)
		expectedErr bool
	}{
		{
			name: "successful status retrieval",
			setupMocks: func(aum *appupgrademanager.MockAppUpgradeManager) {
				aum.On("GetStatus").Return(expectedAppUpgrade, nil)
			},
			expectedErr: false,
		},
		{
			name: "manager returns error",
			setupMocks: func(aum *appupgrademanager.MockAppUpgradeManager) {
				aum.On("GetStatus").Return(types.AppUpgrade{}, errors.New("status error"))
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		s.T().Run(tt.name, func(t *testing.T) {
			appPreflightManager := &apppreflightmanager.MockAppPreflightManager{}
			appReleaseManager := &appreleasemanager.MockAppReleaseManager{}
			appInstallManager := &appinstallmanager.MockAppInstallManager{}
			appUpgradeManager := &appupgrademanager.MockAppUpgradeManager{}
			sm := s.CreateUpgradeStateMachine(states.StateNew)

			appConfigManager := &appconfig.MockAppConfigManager{}
			appConfigManager.On("TemplateConfig", types.AppConfigValues{}, false, false).Return(types.AppConfig{}, nil)

			controller, err := appcontroller.NewAppController(
				appcontroller.WithStateMachine(sm),
				appcontroller.WithAppConfigManager(appConfigManager),
				appcontroller.WithAppPreflightManager(appPreflightManager),
				appcontroller.WithAppReleaseManager(appReleaseManager),
				appcontroller.WithAppInstallManager(appInstallManager),
				appcontroller.WithAppUpgradeManager(appUpgradeManager),
				appcontroller.WithStore(&store.MockStore{}),
				appcontroller.WithReleaseData(&release.ReleaseData{}),
			)
			require.NoError(t, err, "failed to create app controller")

			tt.setupMocks(appUpgradeManager)
			result, err := controller.GetAppUpgradeStatus(t.Context())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Equal(t, types.AppUpgrade{}, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, expectedAppUpgrade, result)
			}

			appUpgradeManager.AssertExpectations(t)
		})
	}
}
