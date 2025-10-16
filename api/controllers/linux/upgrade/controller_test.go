package upgrade

import (
	"context"
	"errors"
	"testing"
	"time"

	appcontroller "github.com/replicatedhq/embedded-cluster/api/controllers/app"
	airgapmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/airgap"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestUpgradeInfra(t *testing.T) {
	tests := []struct {
		name          string
		currentState  statemachine.State
		expectedState statemachine.State
		setupMocks    func(runtimeconfig.RuntimeConfig, *infra.MockInfraManager, *installation.MockInstallationManager)
		expectedErr   error
	}{
		{
			name:          "successful upgrade",
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateInfrastructureUpgraded,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, im *infra.MockInfraManager, instMgr *installation.MockInstallationManager) {
				instMgr.On("GetRegistrySettings", mock.Anything, rc).Return(nil, nil)
				im.On("Upgrade", mock.Anything, rc, mock.Anything).Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:          "upgrade error",
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateInfrastructureUpgradeFailed,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, im *infra.MockInfraManager, instMgr *installation.MockInstallationManager) {
				instMgr.On("GetRegistrySettings", mock.Anything, rc).Return(nil, nil)
				im.On("Upgrade", mock.Anything, rc, mock.Anything).Return(errors.New("upgrade error"))
			},
			expectedErr: nil,
		},
		{
			name:          "upgrade panic",
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateInfrastructureUpgradeFailed,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, im *infra.MockInfraManager, instMgr *installation.MockInstallationManager) {
				instMgr.On("GetRegistrySettings", mock.Anything, rc).Return(nil, nil)
				im.On("Upgrade", mock.Anything, rc, mock.Anything).Panic("this is a panic")
			},
			expectedErr: nil,
		},
		{
			name:          "invalid state transition",
			currentState:  states.StateInfrastructureUpgrading,
			expectedState: states.StateInfrastructureUpgrading,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, im *infra.MockInfraManager, instMgr *installation.MockInstallationManager) {
				// No mock setup needed for this test
			},
			expectedErr: assert.AnError, // Just check that an error occurs
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := runtimeconfig.New(nil)
			rc.SetDataDir(t.TempDir())
			rc.SetManagerPort(9001)

			mockInfraManager := &infra.MockInfraManager{}
			mockStore := &store.MockStore{}
			mockInstallationManager := &installation.MockInstallationManager{}

			tt.setupMocks(rc, mockInfraManager, mockInstallationManager)

			sm := NewStateMachine(
				WithCurrentState(tt.currentState),
				WithRequiresInfraUpgrade(true),
			)

			appController, err := appcontroller.NewAppController(
				appcontroller.WithStateMachine(sm),
				appcontroller.WithStore(mockStore),
				appcontroller.WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
				appcontroller.WithHelmClient(&helm.MockClient{}),
			)
			require.NoError(t, err)

			controller, err := NewUpgradeController(
				WithRuntimeConfig(rc),
				WithStateMachine(sm),
				WithInfraManager(mockInfraManager),
				WithInstallationManager(mockInstallationManager),
				WithAppController(appController),
				WithStore(mockStore),
				WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
				WithHelmClient(&helm.MockClient{}),
			)
			require.NoError(t, err)

			err = controller.UpgradeInfra(context.Background())

			if tt.expectedErr != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Wait for the goroutine to complete and state to transition
			assert.Eventually(t, func() bool {
				return sm.CurrentState() == tt.expectedState
			}, time.Second, 100*time.Millisecond, "state should be %s but is %s", tt.expectedState, sm.CurrentState())

			assert.Eventually(t, func() bool {
				return !sm.IsLockAcquired()
			}, time.Second, 100*time.Millisecond, "state machine should not be locked")

			mockInfraManager.AssertExpectations(t)
			mockInstallationManager.AssertExpectations(t)
		})
	}
}

func TestGetInfra(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*infra.MockInfraManager)
		expectedErr   bool
		expectedValue types.Infra
	}{
		{
			name: "successful get infra",
			setupMock: func(m *infra.MockInfraManager) {
				infraObj := types.Infra{
					Components: []types.InfraComponent{
						{
							Name: infra.K0sComponentName,
							Status: types.Status{
								State: types.StateRunning,
							},
						},
					},
					Status: types.Status{
						State: types.StateRunning,
					},
				}
				m.On("Get").Return(infraObj, nil)
			},
			expectedErr: false,
			expectedValue: types.Infra{
				Components: []types.InfraComponent{
					{
						Name: infra.K0sComponentName,
						Status: types.Status{
							State: types.StateRunning,
						},
					},
				},
				Status: types.Status{
					State: types.StateRunning,
				},
			},
		},
		{
			name: "get infra error",
			setupMock: func(m *infra.MockInfraManager) {
				m.On("Get").Return(types.Infra{}, errors.New("get infra error"))
			},
			expectedErr:   true,
			expectedValue: types.Infra{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockInfraManager := &infra.MockInfraManager{}
			mockStore := &store.MockStore{}

			tt.setupMock(mockInfraManager)

			controller, err := NewUpgradeController(
				WithInfraManager(mockInfraManager),
				WithStore(mockStore),
				WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
				WithHelmClient(&helm.MockClient{}),
			)
			require.NoError(t, err)

			result, err := controller.GetInfra(context.Background())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Equal(t, types.Infra{}, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}

			mockInfraManager.AssertExpectations(t)
		})
	}
}

func TestReportingHandlers(t *testing.T) {
	tests := []struct {
		name                 string
		currentState         statemachine.State
		targetState          statemachine.State
		requiresInfraUpgrade bool
		targetVersion        string
		initialVersion       string
		setupMocks           func(*metrics.MockReporter, *store.MockStore)
	}{
		{
			name:                 "report upgrade succeeded",
			currentState:         states.StateAppUpgrading,
			targetState:          states.StateSucceeded,
			requiresInfraUpgrade: false,
			targetVersion:        "1.0.0",
			initialVersion:       "0.9.0",
			setupMocks: func(mr *metrics.MockReporter, st *store.MockStore) {
				mr.On("ReportUpgradeSucceeded", mock.Anything, "1.0.0", "0.9.0")
			},
		},
		{
			name:                 "report infrastructure upgrade failed",
			currentState:         states.StateInfrastructureUpgrading,
			targetState:          states.StateInfrastructureUpgradeFailed,
			requiresInfraUpgrade: true,
			targetVersion:        "1.0.0",
			initialVersion:       "0.9.0",
			setupMocks: func(mr *metrics.MockReporter, st *store.MockStore) {
				st.LinuxInfraMockStore.On("GetStatus").Return(types.Status{
					State:       types.StateFailed,
					Description: "infrastructure upgrade failed",
				}, nil)
				mr.On("ReportUpgradeFailed", mock.Anything, mock.MatchedBy(func(err error) bool {
					return err.Error() == "infrastructure upgrade failed"
				}), "1.0.0", "0.9.0")
			},
		},
		{
			name:                 "report app upgrade failed",
			currentState:         states.StateAppUpgrading,
			targetState:          states.StateAppUpgradeFailed,
			requiresInfraUpgrade: false,
			targetVersion:        "1.0.0",
			initialVersion:       "0.9.0",
			setupMocks: func(mr *metrics.MockReporter, st *store.MockStore) {
				st.AppUpgradeMockStore.On("GetStatus").Return(types.Status{
					State:       types.StateFailed,
					Description: "app upgrade failed",
				}, nil)
				mr.On("ReportUpgradeFailed", mock.Anything, mock.MatchedBy(func(err error) bool {
					return err.Error() == "app upgrade failed"
				}), "1.0.0", "0.9.0")
			},
		},
		{
			name:                 "report app preflights succeeded",
			currentState:         states.StateAppPreflightsRunning,
			targetState:          states.StateAppPreflightsSucceeded,
			requiresInfraUpgrade: false,
			setupMocks: func(mr *metrics.MockReporter, st *store.MockStore) {
				mr.On("ReportAppPreflightsSucceeded", mock.Anything)
			},
		},
		{
			name:                 "report app preflights failed",
			currentState:         states.StateAppPreflightsRunning,
			targetState:          states.StateAppPreflightsFailed,
			requiresInfraUpgrade: false,
			setupMocks: func(mr *metrics.MockReporter, st *store.MockStore) {
				output := &types.PreflightsOutput{
					Fail: []types.PreflightsRecord{
						{
							Title:   "Test Check",
							Message: "Test check failed",
							Strict:  true,
						},
					},
				}
				st.AppPreflightMockStore.On("GetOutput").Return(output, nil)
				mr.On("ReportAppPreflightsFailed", mock.Anything, output)
			},
		},
		{
			name:                 "report app preflights bypassed",
			currentState:         states.StateAppPreflightsFailed,
			targetState:          states.StateAppPreflightsFailedBypassed,
			requiresInfraUpgrade: false,
			setupMocks: func(mr *metrics.MockReporter, st *store.MockStore) {
				output := &types.PreflightsOutput{
					Fail: []types.PreflightsRecord{
						{
							Title:   "Non-strict Check",
							Message: "Test check failed but can be bypassed",
							Strict:  false,
						},
					},
				}
				st.AppPreflightMockStore.On("GetOutput").Return(output, nil)
				mr.On("ReportAppPreflightsBypassed", mock.Anything, output)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMetricsReporter := &metrics.MockReporter{}
			mockStore := &store.MockStore{}
			mockInfraManager := &infra.MockInfraManager{}

			tt.setupMocks(mockMetricsReporter, mockStore)

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
			)
			require.NoError(t, err)

			// Trigger the state transition
			lock, err := sm.AcquireLock()
			require.NoError(t, err)
			defer lock.Release()

			err = sm.Transition(lock, tt.targetState)
			require.NoError(t, err)

			// Wait for the event handler goroutine to complete
			time.Sleep(1 * time.Second)

			// Verify that the metrics reporter was called as expected
			mockMetricsReporter.AssertExpectations(t)
			mockStore.LinuxInfraMockStore.AssertExpectations(t)
			mockStore.AppUpgradeMockStore.AssertExpectations(t)
			mockStore.AppPreflightMockStore.AssertExpectations(t)

			// Avoid unused variable error
			_ = controller
		})
	}
}

func TestProcessAirgap(t *testing.T) {
	tests := []struct {
		name          string
		currentState  statemachine.State
		expectedState statemachine.State
		setupMocks    func(*airgapmanager.MockAirgapManager, *installation.MockInstallationManager, *types.RegistrySettings, runtimeconfig.RuntimeConfig)
		expectedErr   bool
	}{
		{
			name:          "successful airgap processing",
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateAirgapProcessed,
			setupMocks: func(am *airgapmanager.MockAirgapManager, im *installation.MockInstallationManager, rs *types.RegistrySettings, rc runtimeconfig.RuntimeConfig) {
				im.On("GetRegistrySettings", mock.Anything, rc).Return(rs, nil)
				am.On("Process", mock.Anything, rs).Return(nil)
			},
			expectedErr: false,
		},
		{
			name:          "airgap processing error",
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateAirgapProcessingFailed,
			setupMocks: func(am *airgapmanager.MockAirgapManager, im *installation.MockInstallationManager, rs *types.RegistrySettings, rc runtimeconfig.RuntimeConfig) {
				im.On("GetRegistrySettings", mock.Anything, rc).Return(rs, nil)
				am.On("Process", mock.Anything, rs).Return(errors.New("processing error"))
			},
			expectedErr: false,
		},
		{
			name:          "airgap processing panic",
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateAirgapProcessingFailed,
			setupMocks: func(am *airgapmanager.MockAirgapManager, im *installation.MockInstallationManager, rs *types.RegistrySettings, rc runtimeconfig.RuntimeConfig) {
				im.On("GetRegistrySettings", mock.Anything, rc).Return(rs, nil)
				am.On("Process", mock.Anything, rs).Panic("this is a panic")
			},
			expectedErr: false,
		},
		{
			name:          "invalid state transition",
			currentState:  states.StateInfrastructureUpgraded,
			expectedState: states.StateInfrastructureUpgraded,
			setupMocks: func(am *airgapmanager.MockAirgapManager, im *installation.MockInstallationManager, rs *types.RegistrySettings, rc runtimeconfig.RuntimeConfig) {
				// No mock setup needed for invalid state transition
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := runtimeconfig.New(nil)
			rc.SetDataDir(t.TempDir())
			rc.SetManagerPort(9001)

			sm := NewStateMachine(
				WithCurrentState(tt.currentState),
				WithIsAirgap(true),
			)

			mockAirgapManager := &airgapmanager.MockAirgapManager{}
			mockStore := &store.MockStore{}
			mockInstallationManager := &installation.MockInstallationManager{}

			// Setup expected registry settings
			expectedRegistrySettings := &types.RegistrySettings{
				Host:             "registry.local:5000",
				HasLocalRegistry: true,
			}

			tt.setupMocks(mockAirgapManager, mockInstallationManager, expectedRegistrySettings, rc)

			controller, err := NewUpgradeController(
				WithRuntimeConfig(rc),
				WithStateMachine(sm),
				WithAirgapManager(mockAirgapManager),
				WithStore(mockStore),
				WithInstallationManager(mockInstallationManager),
				WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
				WithHelmClient(&helm.MockClient{}),
			)
			require.NoError(t, err)

			err = controller.ProcessAirgap(context.Background())

			if tt.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Wait for the goroutine to complete and state to transition
			assert.Eventually(t, func() bool {
				return sm.CurrentState() == tt.expectedState
			}, time.Second, 100*time.Millisecond, "state should be %s but is %s", tt.expectedState, sm.CurrentState())

			assert.Eventually(t, func() bool {
				return !sm.IsLockAcquired()
			}, time.Second, 100*time.Millisecond, "state machine should not be locked")

			mockAirgapManager.AssertExpectations(t)
			mockInstallationManager.AssertExpectations(t)
		})
	}
}

func TestGetAirgapStatus(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*airgapmanager.MockAirgapManager)
		expectedErr   bool
		expectedValue types.Airgap
	}{
		{
			name: "successful get status",
			setupMock: func(m *airgapmanager.MockAirgapManager) {
				airgapStatus := types.Airgap{
					Status: types.Status{
						State:       types.StateSucceeded,
						Description: "Airgap processing completed",
					},
				}
				m.On("GetStatus").Return(airgapStatus, nil)
			},
			expectedErr: false,
			expectedValue: types.Airgap{
				Status: types.Status{
					State:       types.StateSucceeded,
					Description: "Airgap processing completed",
				},
			},
		},
		{
			name: "get status error",
			setupMock: func(m *airgapmanager.MockAirgapManager) {
				m.On("GetStatus").Return(nil, errors.New("get status error"))
			},
			expectedErr:   true,
			expectedValue: types.Airgap{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &airgapmanager.MockAirgapManager{}
			tt.setupMock(mockManager)

			controller, err := NewUpgradeController(
				WithAirgapManager(mockManager),
				WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
				WithHelmClient(&helm.MockClient{}),
			)
			require.NoError(t, err)

			result, err := controller.GetAirgapStatus(context.Background())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Equal(t, types.Airgap{}, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}

			mockManager.AssertExpectations(t)
		})
	}
}

func getTestReleaseData(appConfig *kotsv1beta1.Config) *release.ReleaseData {
	return &release.ReleaseData{
		EmbeddedClusterConfig: &ecv1beta1.Config{},
		ChannelRelease:        &release.ChannelRelease{},
		AppConfig:             appConfig,
	}
}
