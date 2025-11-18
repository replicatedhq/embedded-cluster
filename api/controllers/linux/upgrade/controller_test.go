package upgrade

import (
	"context"
	"errors"
	"strings"
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
		setupMocks    func(runtimeconfig.RuntimeConfig, *infra.MockInfraManager, *installation.MockInstallationManager, *store.MockStore)
		expectedErr   error
	}{
		{
			name:          "successful upgrade",
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateInfrastructureUpgraded,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, im *infra.MockInfraManager, instMgr *installation.MockInstallationManager, st *store.MockStore) {
				st.LinuxInfraMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
					return status.State == types.StateRunning
				})).Return(nil)
				instMgr.On("GetRegistrySettings", mock.Anything, rc).Return(nil, nil)
				im.On("Upgrade", mock.Anything, rc, mock.Anything).Return(nil)
				st.LinuxInfraMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
					return status.State == types.StateSucceeded
				})).Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:          "upgrade error",
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateInfrastructureUpgradeFailed,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, im *infra.MockInfraManager, instMgr *installation.MockInstallationManager, st *store.MockStore) {
				st.LinuxInfraMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
					return status.State == types.StateRunning
				})).Return(nil)
				instMgr.On("GetRegistrySettings", mock.Anything, rc).Return(nil, nil)
				im.On("Upgrade", mock.Anything, rc, mock.Anything).Return(errors.New("upgrade error"))
				st.LinuxInfraMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
					return status.State == types.StateFailed && status.Description == "upgrade infrastructure: upgrade error"
				})).Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:          "upgrade panic",
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateInfrastructureUpgradeFailed,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, im *infra.MockInfraManager, instMgr *installation.MockInstallationManager, st *store.MockStore) {
				st.LinuxInfraMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
					return status.State == types.StateRunning
				})).Return(nil)
				instMgr.On("GetRegistrySettings", mock.Anything, rc).Return(nil, nil)
				im.On("Upgrade", mock.Anything, rc, mock.Anything).Panic("this is a panic")
				st.LinuxInfraMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
					return status.State == types.StateFailed && strings.HasPrefix(status.Description, "panic: this is a panic")
				})).Return(nil)
			},
			expectedErr: nil,
		},
		{
			name:          "invalid state transition",
			currentState:  states.StateInfrastructureUpgrading,
			expectedState: states.StateInfrastructureUpgrading,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, im *infra.MockInfraManager, instMgr *installation.MockInstallationManager, st *store.MockStore) {
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
			mockInstallationManager := &installation.MockInstallationManager{}

			mockStore := &store.MockStore{}
			mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

			tt.setupMocks(rc, mockInfraManager, mockInstallationManager, mockStore)

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

			mockStore.AssertExpectations(t)
		})
	}
}

func TestProcessAirgap(t *testing.T) {
	tests := []struct {
		name          string
		currentState  statemachine.State
		expectedState statemachine.State
		setupMocks    func(*airgapmanager.MockAirgapManager, *installation.MockInstallationManager, *types.RegistrySettings, runtimeconfig.RuntimeConfig, *store.MockStore)
		expectedErr   bool
	}{
		{
			name:          "successful airgap processing",
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateAirgapProcessed,
			setupMocks: func(am *airgapmanager.MockAirgapManager, im *installation.MockInstallationManager, rs *types.RegistrySettings, rc runtimeconfig.RuntimeConfig, st *store.MockStore) {
				st.AirgapMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
					return status.State == types.StateRunning
				})).Return(nil)
				im.On("GetRegistrySettings", mock.Anything, rc).Return(rs, nil)
				am.On("Process", mock.Anything, rs).Return(nil)
				st.AirgapMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
					return status.State == types.StateSucceeded
				})).Return(nil)
			},
			expectedErr: false,
		},
		{
			name:          "airgap processing error",
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateAirgapProcessingFailed,
			setupMocks: func(am *airgapmanager.MockAirgapManager, im *installation.MockInstallationManager, rs *types.RegistrySettings, rc runtimeconfig.RuntimeConfig, st *store.MockStore) {
				st.AirgapMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
					return status.State == types.StateRunning
				})).Return(nil)
				im.On("GetRegistrySettings", mock.Anything, rc).Return(rs, nil)
				am.On("Process", mock.Anything, rs).Return(errors.New("processing error"))
				st.AirgapMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
					return status.State == types.StateFailed && status.Description == "process airgap: processing error"
				})).Return(nil)
			},
			expectedErr: false,
		},
		{
			name:          "airgap processing panic",
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateAirgapProcessingFailed,
			setupMocks: func(am *airgapmanager.MockAirgapManager, im *installation.MockInstallationManager, rs *types.RegistrySettings, rc runtimeconfig.RuntimeConfig, st *store.MockStore) {
				st.AirgapMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
					return status.State == types.StateRunning
				})).Return(nil)
				im.On("GetRegistrySettings", mock.Anything, rc).Return(rs, nil)
				am.On("Process", mock.Anything, rs).Panic("this is a panic")
				st.AirgapMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
					return status.State == types.StateFailed && strings.HasPrefix(status.Description, "panic: this is a panic")
				})).Return(nil)
			},
			expectedErr: false,
		},
		{
			name:          "invalid state transition",
			currentState:  states.StateInfrastructureUpgraded,
			expectedState: states.StateInfrastructureUpgraded,
			setupMocks: func(am *airgapmanager.MockAirgapManager, im *installation.MockInstallationManager, rs *types.RegistrySettings, rc runtimeconfig.RuntimeConfig, st *store.MockStore) {
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

			tt.setupMocks(mockAirgapManager, mockInstallationManager, expectedRegistrySettings, rc, mockStore)

			mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

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

			mockStore.AssertExpectations(t)
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
