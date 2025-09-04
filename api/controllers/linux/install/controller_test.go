package install

import (
	"errors"
	"testing"
	"time"

	appcontroller "github.com/replicatedhq/embedded-cluster/api/controllers/app/install"
	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	apppreflightmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/preflight"
	appreleasemanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/release"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/preflight"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var failedPreflightOutput = &types.PreflightsOutput{
	Fail: []types.PreflightsRecord{
		{
			Title:   "Test Check",
			Message: "Test check failed",
		},
	},
}

var successfulPreflightOutput = &types.PreflightsOutput{
	Pass: []types.PreflightsRecord{
		{
			Title:   "Test Check",
			Message: "Test check passed",
		},
	},
}

var warnPreflightOutput = &types.PreflightsOutput{
	Warn: []types.PreflightsRecord{
		{
			Title:   "Test Check",
			Message: "Test check warning",
		},
	},
}

func TestGetInstallationConfig(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*installation.MockInstallationManager, runtimeconfig.RuntimeConfig)
		expectedErr   bool
		expectedValue func(string) types.LinuxInstallationConfigResponse
	}{
		{
			name: "successful read with values and defaults",
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig) {
				values := types.LinuxInstallationConfig{
					AdminConsolePort: 9000,
					GlobalCIDR:       "10.0.0.1/16",
				}

				defaults := types.LinuxInstallationConfig{
					AdminConsolePort:        30000,
					DataDirectory:           rc.EmbeddedClusterHomeDirectory(),
					LocalArtifactMirrorPort: 50000,
					GlobalCIDR:              "10.244.0.0/16",
				}

				resolvedConfig := types.LinuxInstallationConfig{
					AdminConsolePort:        9000,
					DataDirectory:           rc.EmbeddedClusterHomeDirectory(),
					LocalArtifactMirrorPort: 50000,
					GlobalCIDR:              "10.0.0.1/16",
				}

				mock.InOrder(
					m.On("GetConfigValues").Return(values, nil),
					m.On("GetDefaults", rc).Return(defaults, nil),
					m.On("GetConfig", rc).Return(resolvedConfig, nil),
				)
			},
			expectedErr: false,
			expectedValue: func(tempDir string) types.LinuxInstallationConfigResponse {
				return types.LinuxInstallationConfigResponse{
					Values: types.LinuxInstallationConfig{
						AdminConsolePort: 9000,
						GlobalCIDR:       "10.0.0.1/16",
					},
					Defaults: types.LinuxInstallationConfig{
						AdminConsolePort:        30000,
						DataDirectory:           tempDir,
						LocalArtifactMirrorPort: 50000,
						GlobalCIDR:              "10.244.0.0/16",
					},
					Resolved: types.LinuxInstallationConfig{
						AdminConsolePort:        9000,
						DataDirectory:           tempDir,
						LocalArtifactMirrorPort: 50000,
						GlobalCIDR:              "10.0.0.1/16",
					},
				}
			},
		},
		{
			name: "read config error",
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig) {
				m.On("GetConfigValues").Return(types.LinuxInstallationConfig{}, errors.New("read error"))
			},
			expectedErr: true,
			expectedValue: func(tempDir string) types.LinuxInstallationConfigResponse {
				return types.LinuxInstallationConfigResponse{}
			},
		},
		{
			name: "get defaults error",
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig) {
				values := types.LinuxInstallationConfig{}
				mock.InOrder(
					m.On("GetConfigValues").Return(values, nil),
					m.On("GetDefaults", rc).Return(types.LinuxInstallationConfig{}, errors.New("defaults error")),
				)
			},
			expectedErr: true,
			expectedValue: func(tempDir string) types.LinuxInstallationConfigResponse {
				return types.LinuxInstallationConfigResponse{}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
			rc.SetDataDir(tempDir)
			rc.SetManagerPort(9001)

			mockManager := &installation.MockInstallationManager{}
			tt.setupMock(mockManager, rc)

			controller, err := NewInstallController(
				WithRuntimeConfig(rc),
				WithInstallationManager(mockManager),
				WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
			)
			require.NoError(t, err)

			result, err := controller.GetInstallationConfig(t.Context())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Equal(t, types.LinuxInstallationConfigResponse{}, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue(rc.EmbeddedClusterHomeDirectory()), result)
			}

			mockManager.AssertExpectations(t)
		})
	}
}

func TestConfigureInstallation(t *testing.T) {
	tests := []struct {
		name          string
		config        types.LinuxInstallationConfig
		currentState  statemachine.State
		expectedState statemachine.State
		setupMock     func(*installation.MockInstallationManager, runtimeconfig.RuntimeConfig, types.LinuxInstallationConfig, *store.MockStore, *metrics.MockReporter)
		expectedErr   bool
	}{
		{
			name: "successful configure installation",
			config: types.LinuxInstallationConfig{
				LocalArtifactMirrorPort: 9000,
				DataDirectory:           t.TempDir(),
			},
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateHostConfigured,

			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.LinuxInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
				mock.InOrder(
					m.On("SetConfigValues", config).Return(nil),
					m.On("GetConfig", rc).Return(config, nil),
					m.On("ValidateConfig", config, 9001).Return(nil),
					m.On("ConfigureHost", mock.Anything, rc).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "validatation error",
			config:        types.LinuxInstallationConfig{},
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateInstallationConfigurationFailed,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.LinuxInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
				mock.InOrder(
					m.On("SetConfigValues", config).Return(nil),
					m.On("GetConfig", rc).Return(config, nil),
					m.On("ValidateConfig", config, 9001).Return(errors.New("validation error")),
					// Status is set in the store by the controller when configuring the installation
					st.LinuxInstallationMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateFailed && status.Description == "validate: validation error"
					})).Return(nil),
					st.LinuxInstallationMockStore.On("GetStatus").Return(types.Status{Description: "validate: validation error"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("validate: validation error")),
				)
			},
			expectedErr: true,
		},
		{
			name:          "validation error on retry from host already configured",
			config:        types.LinuxInstallationConfig{},
			currentState:  states.StateHostConfigured,
			expectedState: states.StateInstallationConfigurationFailed,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.LinuxInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
				mock.InOrder(
					m.On("SetConfigValues", config).Return(nil),
					m.On("GetConfig", rc).Return(config, nil),
					m.On("ValidateConfig", config, 9001).Return(errors.New("validation error")),
					// Status is set in the store by the controller when configuring the installation
					st.LinuxInstallationMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateFailed && status.Description == "validate: validation error"
					})).Return(nil),
					st.LinuxInstallationMockStore.On("GetStatus").Return(types.Status{Description: "validate: validation error"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("validate: validation error")),
				)
			},
			expectedErr: true,
		},
		{
			name:          "validation error on retry from host that failed to configure",
			config:        types.LinuxInstallationConfig{},
			currentState:  states.StateHostConfigurationFailed,
			expectedState: states.StateInstallationConfigurationFailed,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.LinuxInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
				mock.InOrder(
					m.On("SetConfigValues", config).Return(nil),
					m.On("GetConfig", rc).Return(config, nil),
					m.On("ValidateConfig", config, 9001).Return(errors.New("validation error")),
					// Status is set in the store by the controller when configuring the installation
					st.LinuxInstallationMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateFailed && status.Description == "validate: validation error"
					})).Return(nil),
					st.LinuxInstallationMockStore.On("GetStatus").Return(types.Status{Description: "validate: validation error"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("validate: validation error")),
				)
			},
			expectedErr: true,
		},
		{
			name:          "set config error",
			config:        types.LinuxInstallationConfig{},
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateInstallationConfigurationFailed,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.LinuxInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
				mock.InOrder(
					m.On("SetConfigValues", config).Return(errors.New("set config error")),
					// Status is set in the store by the controller when configuring the installation
					st.LinuxInstallationMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateFailed && status.Description == "write: set config error"
					})).Return(nil),
					st.LinuxInstallationMockStore.On("GetStatus").Return(types.Status{Description: "write: set config error"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("write: set config error")),
				)
			},
			expectedErr: true,
		},
		{
			name:          "set config error on retry from host already configured",
			config:        types.LinuxInstallationConfig{},
			currentState:  states.StateHostConfigured,
			expectedState: states.StateInstallationConfigurationFailed,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.LinuxInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
				mock.InOrder(
					m.On("SetConfigValues", config).Return(errors.New("set config error")),
					// Status is set in the store by the controller when configuring the installation
					st.LinuxInstallationMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateFailed && status.Description == "write: set config error"
					})).Return(nil),
					st.LinuxInstallationMockStore.On("GetStatus").Return(types.Status{Description: "write: set config error"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("write: set config error")),
				)
			},
			expectedErr: true,
		},
		{
			name:          "set config error on retry from host that failed to configure",
			config:        types.LinuxInstallationConfig{},
			currentState:  states.StateHostConfigurationFailed,
			expectedState: states.StateInstallationConfigurationFailed,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.LinuxInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
				mock.InOrder(
					m.On("SetConfigValues", config).Return(errors.New("set config error")),
					// Status is set in the store by the controller when configuring the installation
					st.LinuxInstallationMockStore.On("SetStatus", mock.MatchedBy(func(status types.Status) bool {
						return status.State == types.StateFailed && status.Description == "write: set config error"
					})).Return(nil),
					st.LinuxInstallationMockStore.On("GetStatus").Return(types.Status{Description: "write: set config error"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("write: set config error")),
				)
			},
			expectedErr: true,
		},
		{
			name: "configure host error",
			config: types.LinuxInstallationConfig{
				LocalArtifactMirrorPort: 9000,
				DataDirectory:           t.TempDir(),
			},
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateHostConfigurationFailed,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.LinuxInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
				mock.InOrder(
					m.On("SetConfigValues", config).Return(nil),
					m.On("GetConfig", rc).Return(config, nil),
					m.On("ValidateConfig", config, 9001).Return(nil),
					m.On("ConfigureHost", mock.Anything, rc).Return(errors.New("configure host error")),
					st.LinuxInstallationMockStore.On("GetStatus").Return(types.Status{Description: "configure host error"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("configure host error")),
				)
			},
			expectedErr: false,
		},
		{
			name: "configure host error on retry from host already configured",
			config: types.LinuxInstallationConfig{
				LocalArtifactMirrorPort: 9000,
				DataDirectory:           t.TempDir(),
			},
			currentState:  states.StateHostConfigured,
			expectedState: states.StateHostConfigurationFailed,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.LinuxInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
				mock.InOrder(
					m.On("SetConfigValues", config).Return(nil),
					m.On("GetConfig", rc).Return(config, nil),
					m.On("ValidateConfig", config, 9001).Return(nil),
					m.On("ConfigureHost", mock.Anything, rc).Return(errors.New("configure host error")),
					st.LinuxInstallationMockStore.On("GetStatus").Return(types.Status{Description: "configure host error"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("configure host error")),
				)
			},
			expectedErr: false,
		},
		{
			name: "configure host error on retry from host that failed to configure",
			config: types.LinuxInstallationConfig{
				LocalArtifactMirrorPort: 9000,
				DataDirectory:           t.TempDir(),
			},
			currentState:  states.StateHostConfigurationFailed,
			expectedState: states.StateHostConfigurationFailed,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.LinuxInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
				mock.InOrder(
					m.On("SetConfigValues", config).Return(nil),
					m.On("GetConfig", rc).Return(config, nil),
					m.On("ValidateConfig", config, 9001).Return(nil),
					m.On("ConfigureHost", mock.Anything, rc).Return(errors.New("configure host error")),
					st.LinuxInstallationMockStore.On("GetStatus").Return(types.Status{Description: "configure host error"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("configure host error")),
				)
			},
			expectedErr: false,
		},
		{
			name: "invalid state transition",
			config: types.LinuxInstallationConfig{
				LocalArtifactMirrorPort: 9000,
				DataDirectory:           t.TempDir(),
			},
			currentState:  states.StateInfrastructureInstalling,
			expectedState: states.StateInfrastructureInstalling,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.LinuxInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
			rc.SetDataDir(t.TempDir())
			rc.SetManagerPort(9001)

			sm := NewStateMachine(WithCurrentState(tt.currentState))

			mockManager := &installation.MockInstallationManager{}
			mockMetricsReporter := &metrics.MockReporter{}
			mockStore := &store.MockStore{}

			tt.setupMock(mockManager, rc, tt.config, mockStore, mockMetricsReporter)

			controller, err := NewInstallController(
				WithRuntimeConfig(rc),
				WithStateMachine(sm),
				WithInstallationManager(mockManager),
				WithStore(mockStore),
				WithMetricsReporter(mockMetricsReporter),
				WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
			)
			require.NoError(t, err)

			err = controller.ConfigureInstallation(t.Context(), tt.config)
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

			mockManager.AssertExpectations(t)
			mockStore.LinuxInfraMockStore.AssertExpectations(t)
			mockStore.LinuxInstallationMockStore.AssertExpectations(t)
			mockStore.LinuxPreflightMockStore.AssertExpectations(t)
			mockStore.AppConfigMockStore.AssertExpectations(t)

			// Wait for the event handler goroutine to complete
			// TODO: find a better way to do this
			time.Sleep(1 * time.Second)
			mockMetricsReporter.AssertExpectations(t)
		})
	}
}

func TestRunHostPreflights(t *testing.T) {
	expectedHPF := &troubleshootv1beta2.HostPreflightSpec{
		Collectors: []*troubleshootv1beta2.HostCollect{
			{
				Time: &troubleshootv1beta2.HostTime{},
			},
		},
	}

	tests := []struct {
		name          string
		currentState  statemachine.State
		expectedState statemachine.State
		setupMocks    func(*preflight.MockHostPreflightManager, runtimeconfig.RuntimeConfig, *metrics.MockReporter, *store.MockStore)
		expectedErr   bool
	}{
		{
			name:          "successful run preflights without preflight errors",
			currentState:  states.StateHostConfigured,
			expectedState: states.StateHostPreflightsSucceeded,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", mock.Anything, rc, mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(nil),
					pm.On("GetHostPreflightOutput", mock.Anything).Return(successfulPreflightOutput, nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "successful run preflights from preflights execution failed state without preflight errors",
			currentState:  states.StateHostPreflightsExecutionFailed,
			expectedState: states.StateHostPreflightsSucceeded,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", mock.Anything, rc, mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(nil),
					pm.On("GetHostPreflightOutput", mock.Anything).Return(successfulPreflightOutput, nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "successful run preflights from preflights failed state without preflight errors",
			currentState:  states.StateHostPreflightsFailed,
			expectedState: states.StateHostPreflightsSucceeded,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", mock.Anything, rc, mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(nil),
					pm.On("GetHostPreflightOutput", mock.Anything).Return(successfulPreflightOutput, nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "successful run preflights from preflights failure bypassed state without preflight errors",
			currentState:  states.StateHostPreflightsFailedBypassed,
			expectedState: states.StateHostPreflightsSucceeded,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", mock.Anything, rc, mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(nil),
					pm.On("GetHostPreflightOutput", mock.Anything).Return(successfulPreflightOutput, nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "successful run preflights with preflight errors",
			currentState:  states.StateHostPreflightsFailedBypassed,
			expectedState: states.StateHostPreflightsFailed,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", mock.Anything, rc, mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(nil),
					pm.On("GetHostPreflightOutput", mock.Anything).Return(failedPreflightOutput, nil),
					st.LinuxPreflightMockStore.On("GetOutput").Return(failedPreflightOutput, nil),
					mr.On("ReportHostPreflightsFailed", mock.Anything, failedPreflightOutput).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "successful run preflights with preflight errors and failure to get output for reporting",
			currentState:  states.StateHostPreflightsFailedBypassed,
			expectedState: states.StateHostPreflightsFailed,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", mock.Anything, rc, mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(nil),
					pm.On("GetHostPreflightOutput", mock.Anything).Return(failedPreflightOutput, nil),
					st.LinuxPreflightMockStore.On("GetOutput").Return(nil, assert.AnError),
				)
			},
			expectedErr: false,
		},
		{
			name:          "successful run preflights from preflights execution failed state with preflight errors",
			currentState:  states.StateHostPreflightsExecutionFailed,
			expectedState: states.StateHostPreflightsFailed,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", mock.Anything, rc, mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(nil),
					pm.On("GetHostPreflightOutput", mock.Anything).Return(failedPreflightOutput, nil),
					st.LinuxPreflightMockStore.On("GetOutput").Return(failedPreflightOutput, nil),
					mr.On("ReportHostPreflightsFailed", mock.Anything, failedPreflightOutput).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "successful run preflights from preflights failed state with preflight errors",
			currentState:  states.StateHostPreflightsFailed,
			expectedState: states.StateHostPreflightsFailed,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", mock.Anything, rc, mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(nil),
					pm.On("GetHostPreflightOutput", mock.Anything).Return(failedPreflightOutput, nil),
					st.LinuxPreflightMockStore.On("GetOutput").Return(failedPreflightOutput, nil),
					mr.On("ReportHostPreflightsFailed", mock.Anything, failedPreflightOutput).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "successful run preflights from preflights failure bypassed state with preflight errors",
			currentState:  states.StateHostPreflightsFailedBypassed,
			expectedState: states.StateHostPreflightsFailed,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", mock.Anything, rc, mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(nil),
					pm.On("GetHostPreflightOutput", mock.Anything).Return(failedPreflightOutput, nil),
					st.LinuxPreflightMockStore.On("GetOutput").Return(failedPreflightOutput, nil),
					mr.On("ReportHostPreflightsFailed", mock.Anything, failedPreflightOutput).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "successful run preflights with get preflight output error",
			currentState:  states.StateHostConfigured,
			expectedState: states.StateHostPreflightsExecutionFailed,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", mock.Anything, rc, mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(nil),
					pm.On("GetHostPreflightOutput", mock.Anything).Return(nil, assert.AnError),
				)
			},
			expectedErr: false,
		},
		{
			name:          "successful run preflights with nil preflight output",
			currentState:  states.StateHostConfigured,
			expectedState: states.StateHostPreflightsSucceeded,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", mock.Anything, rc, mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(nil),
					pm.On("GetHostPreflightOutput", mock.Anything).Return(nil, nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "successful run preflights with preflight warnings",
			currentState:  states.StateHostConfigured,
			expectedState: states.StateHostPreflightsSucceeded,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", mock.Anything, rc, mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(nil),
					pm.On("GetHostPreflightOutput", mock.Anything).Return(warnPreflightOutput, nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "prepare preflights error",
			currentState:  states.StateHostConfigured,
			expectedState: states.StateHostConfigured,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(nil, errors.New("prepare error")),
				)
			},
			expectedErr: true,
		},
		{
			name:          "run preflights error",
			currentState:  states.StateHostConfigured,
			expectedState: states.StateHostPreflightsExecutionFailed,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", mock.Anything, rc, mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(errors.New("run preflights error")),
				)
			},
			expectedErr: false,
		},
		{
			name:          "run preflights panic",
			currentState:  states.StateHostConfigured,
			expectedState: states.StateHostPreflightsExecutionFailed,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", mock.Anything, rc, mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Panic("this is a panic"),
				)
			},
			expectedErr: false,
		},
		{
			name:          "invalid state transition",
			currentState:  states.StateInfrastructureInstalling,
			expectedState: states.StateInfrastructureInstalling,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig, mr *metrics.MockReporter, st *store.MockStore) {
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := runtimeconfig.New(nil)
			rc.SetDataDir(t.TempDir())
			rc.SetProxySpec(&ecv1beta1.ProxySpec{
				HTTPProxy:       "http://proxy.example.com",
				HTTPSProxy:      "https://proxy.example.com",
				ProvidedNoProxy: "provided-proxy.com",
				NoProxy:         "no-proxy.com",
			})

			sm := NewStateMachine(WithCurrentState(tt.currentState))

			mockPreflightManager := &preflight.MockHostPreflightManager{}
			mockReporter := &metrics.MockReporter{}
			mockStore := &store.MockStore{}

			tt.setupMocks(mockPreflightManager, rc, mockReporter, mockStore)

			controller, err := NewInstallController(
				WithRuntimeConfig(rc),
				WithStateMachine(sm),
				WithHostPreflightManager(mockPreflightManager),
				WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
				WithMetricsReporter(mockReporter),
				WithStore(mockStore),
			)
			require.NoError(t, err)

			err = controller.RunHostPreflights(t.Context(), RunHostPreflightsOptions{})

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

			mockPreflightManager.AssertExpectations(t)
			mockReporter.AssertExpectations(t)
			mockStore.LinuxInfraMockStore.AssertExpectations(t)
			mockStore.LinuxInstallationMockStore.AssertExpectations(t)
			mockStore.LinuxPreflightMockStore.AssertExpectations(t)
			mockStore.AppConfigMockStore.AssertExpectations(t)
		})
	}
}

func TestGetHostPreflightStatus(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*preflight.MockHostPreflightManager)
		expectedErr   bool
		expectedValue types.Status
	}{
		{
			name: "successful get status",
			setupMock: func(m *preflight.MockHostPreflightManager) {
				status := types.Status{
					State: types.StateFailed,
				}
				m.On("GetHostPreflightStatus", t.Context()).Return(status, nil)
			},
			expectedErr: false,
			expectedValue: types.Status{
				State: types.StateFailed,
			},
		},
		{
			name: "get status error",
			setupMock: func(m *preflight.MockHostPreflightManager) {
				m.On("GetHostPreflightStatus", t.Context()).Return(nil, errors.New("get status error"))
			},
			expectedErr:   true,
			expectedValue: types.Status{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &preflight.MockHostPreflightManager{}
			tt.setupMock(mockManager)

			controller, err := NewInstallController(
				WithHostPreflightManager(mockManager),
				WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
			)
			require.NoError(t, err)

			result, err := controller.GetHostPreflightStatus(t.Context())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Equal(t, types.Status{}, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}

			mockManager.AssertExpectations(t)
		})
	}
}

func TestGetHostPreflightOutput(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*preflight.MockHostPreflightManager)
		expectedErr   bool
		expectedValue *types.PreflightsOutput
	}{
		{
			name: "successful get output",
			setupMock: func(m *preflight.MockHostPreflightManager) {
				output := successfulPreflightOutput
				m.On("GetHostPreflightOutput", t.Context()).Return(output, nil)
			},
			expectedErr:   false,
			expectedValue: successfulPreflightOutput,
		},
		{
			name: "get output error",
			setupMock: func(m *preflight.MockHostPreflightManager) {
				m.On("GetHostPreflightOutput", t.Context()).Return(nil, errors.New("get output error"))
			},
			expectedErr:   true,
			expectedValue: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &preflight.MockHostPreflightManager{}
			tt.setupMock(mockManager)

			controller, err := NewInstallController(
				WithHostPreflightManager(mockManager),
				WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
			)
			require.NoError(t, err)

			result, err := controller.GetHostPreflightOutput(t.Context())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}

			mockManager.AssertExpectations(t)
		})
	}
}

func TestGetHostPreflightTitles(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*preflight.MockHostPreflightManager)
		expectedErr   bool
		expectedValue []string
	}{
		{
			name: "successful get titles",
			setupMock: func(m *preflight.MockHostPreflightManager) {
				titles := []string{"Check 1", "Check 2"}
				m.On("GetHostPreflightTitles", t.Context()).Return(titles, nil)
			},
			expectedErr:   false,
			expectedValue: []string{"Check 1", "Check 2"},
		},
		{
			name: "get titles error",
			setupMock: func(m *preflight.MockHostPreflightManager) {
				m.On("GetHostPreflightTitles", t.Context()).Return(nil, errors.New("get titles error"))
			},
			expectedErr:   true,
			expectedValue: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &preflight.MockHostPreflightManager{}
			tt.setupMock(mockManager)

			controller, err := NewInstallController(
				WithHostPreflightManager(mockManager),
				WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
			)
			require.NoError(t, err)

			result, err := controller.GetHostPreflightTitles(t.Context())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}

			mockManager.AssertExpectations(t)
		})
	}
}

func TestGetInstallationStatus(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*installation.MockInstallationManager)
		expectedErr   bool
		expectedValue types.Status
	}{
		{
			name: "successful get status",
			setupMock: func(m *installation.MockInstallationManager) {
				status := types.Status{
					State: types.StateRunning,
				}
				m.On("GetStatus").Return(status, nil)
			},
			expectedErr: false,
			expectedValue: types.Status{
				State: types.StateRunning,
			},
		},
		{
			name: "get status error",
			setupMock: func(m *installation.MockInstallationManager) {
				m.On("GetStatus").Return(nil, errors.New("get status error"))
			},
			expectedErr:   true,
			expectedValue: types.Status{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &installation.MockInstallationManager{}
			tt.setupMock(mockManager)

			controller, err := NewInstallController(
				WithInstallationManager(mockManager),
				WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
			)
			require.NoError(t, err)

			result, err := controller.GetInstallationStatus(t.Context())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Equal(t, types.Status{}, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}

			mockManager.AssertExpectations(t)
		})
	}
}

func TestSetupInfra(t *testing.T) {
	// Create an app config
	appConfig := kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "test-group",
					Title: "Test Group",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "test-item",
							Type:    "text",
							Title:   "Test Item",
							Default: multitype.FromString("default"),
							Value:   multitype.FromString("value"),
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name                            string
		clientIgnoreHostPreflights      bool // From HTTP request
		serverAllowIgnoreHostPreflights bool // From CLI flag
		currentState                    statemachine.State
		expectedState                   statemachine.State
		setupMocks                      func(runtimeconfig.RuntimeConfig, *preflight.MockHostPreflightManager, *installation.MockInstallationManager, *infra.MockInfraManager, *appconfig.MockAppConfigManager, *metrics.MockReporter, *store.MockStore)
		expectedErr                     error
	}{
		{
			name:                            "successful setup with passed preflights",
			clientIgnoreHostPreflights:      false,
			serverAllowIgnoreHostPreflights: true,
			currentState:                    states.StateHostPreflightsSucceeded,
			expectedState:                   states.StateInfrastructureInstalled,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, am *appconfig.MockAppConfigManager, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					fm.On("Install", mock.Anything, rc).Return(nil),
				)
			},
			expectedErr: nil,
		},
		{
			name:                            "successful setup with failed preflights - ignored with CLI flag",
			clientIgnoreHostPreflights:      true,
			serverAllowIgnoreHostPreflights: true,
			currentState:                    states.StateHostPreflightsFailed,
			expectedState:                   states.StateInfrastructureInstalled,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, am *appconfig.MockAppConfigManager, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					st.LinuxPreflightMockStore.On("GetOutput").Return(failedPreflightOutput, nil),
					mr.On("ReportHostPreflightsBypassed", mock.Anything, failedPreflightOutput),
					fm.On("Install", mock.Anything, rc).Return(nil),
				)
			},
			expectedErr: nil,
		},
		{
			name:                            "failed setup with failed preflights - not ignored",
			clientIgnoreHostPreflights:      false,
			serverAllowIgnoreHostPreflights: true,
			currentState:                    states.StateHostPreflightsFailed,
			expectedState:                   states.StateHostPreflightsFailed,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, am *appconfig.MockAppConfigManager, mr *metrics.MockReporter, st *store.MockStore) {
			},
			expectedErr: types.NewBadRequestError(ErrPreflightChecksFailed),
		},
		{
			name:                            "install infra error",
			clientIgnoreHostPreflights:      false,
			serverAllowIgnoreHostPreflights: true,
			currentState:                    states.StateHostPreflightsSucceeded,
			expectedState:                   states.StateInfrastructureInstallFailed,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, am *appconfig.MockAppConfigManager, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					fm.On("Install", mock.Anything, rc).Return(errors.New("install error")),
					st.LinuxInfraMockStore.On("GetStatus").Return(types.Status{Description: "install error"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("install error")),
				)
			},
			expectedErr: nil,
		},
		{
			name:                            "install infra error without report if infra store fails",
			clientIgnoreHostPreflights:      false,
			serverAllowIgnoreHostPreflights: true,
			currentState:                    states.StateHostPreflightsSucceeded,
			expectedState:                   states.StateInfrastructureInstallFailed,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, am *appconfig.MockAppConfigManager, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					fm.On("Install", mock.Anything, rc).Return(errors.New("install error")),
					st.LinuxInfraMockStore.On("GetStatus").Return(nil, assert.AnError),
				)
			},
			expectedErr: nil,
		},
		{
			name:                            "install infra panic",
			clientIgnoreHostPreflights:      false,
			serverAllowIgnoreHostPreflights: true,
			currentState:                    states.StateHostPreflightsSucceeded,
			expectedState:                   states.StateInfrastructureInstallFailed,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, am *appconfig.MockAppConfigManager, mr *metrics.MockReporter, st *store.MockStore) {
				mock.InOrder(
					fm.On("Install", mock.Anything, rc).Panic("this is a panic"),
					st.LinuxInfraMockStore.On("GetStatus").Return(types.Status{Description: "this is a panic"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("this is a panic")),
				)
			},
			expectedErr: nil,
		},
		{
			name:                            "invalid state transition",
			clientIgnoreHostPreflights:      false,
			serverAllowIgnoreHostPreflights: true,
			currentState:                    states.StateInstallationConfigured,
			expectedState:                   states.StateInstallationConfigured,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, am *appconfig.MockAppConfigManager, mr *metrics.MockReporter, st *store.MockStore) {
			},
			expectedErr: assert.AnError, // Just check that an error occurs, don't care about exact message
		},
		{
			name:                            "failed preflights with ignore flag but CLI flag disabled",
			clientIgnoreHostPreflights:      true,
			serverAllowIgnoreHostPreflights: false,
			currentState:                    states.StateHostPreflightsFailed,
			expectedState:                   states.StateHostPreflightsFailed,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, am *appconfig.MockAppConfigManager, mr *metrics.MockReporter, st *store.MockStore) {
			},
			expectedErr: types.NewBadRequestError(ErrPreflightChecksFailed),
		},
		{
			name:                            "failed preflights without ignore flag and CLI flag disabled",
			clientIgnoreHostPreflights:      false,
			serverAllowIgnoreHostPreflights: false,
			currentState:                    states.StateHostPreflightsFailed,
			expectedState:                   states.StateHostPreflightsFailed,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, am *appconfig.MockAppConfigManager, mr *metrics.MockReporter, st *store.MockStore) {
			},
			expectedErr: types.NewBadRequestError(ErrPreflightChecksFailed),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewStateMachine(WithCurrentState(tt.currentState))

			rc := runtimeconfig.New(nil)
			rc.SetDataDir(t.TempDir())
			rc.SetManagerPort(9001)

			mockPreflightManager := &preflight.MockHostPreflightManager{}
			mockInstallationManager := &installation.MockInstallationManager{}
			mockInfraManager := &infra.MockInfraManager{}
			mockMetricsReporter := &metrics.MockReporter{}
			mockStore := &store.MockStore{}
			mockAppConfigManager := &appconfig.MockAppConfigManager{}
			mockAppPreflightManager := &apppreflightmanager.MockAppPreflightManager{}
			mockAppReleaseManager := &appreleasemanager.MockAppReleaseManager{}

			tt.setupMocks(rc, mockPreflightManager, mockInstallationManager, mockInfraManager, mockAppConfigManager, mockMetricsReporter, mockStore)

			appInstallController, err := appcontroller.NewInstallController(
				appcontroller.WithStateMachine(sm),
				appcontroller.WithStore(mockStore),
				appcontroller.WithReleaseData(getTestReleaseData(&appConfig)),
				appcontroller.WithLicense([]byte("spec:\n  licenseID: test-license\n")),
				appcontroller.WithAppConfigManager(mockAppConfigManager),
				appcontroller.WithAppPreflightManager(mockAppPreflightManager),
				appcontroller.WithAppReleaseManager(mockAppReleaseManager),
			)
			require.NoError(t, err)

			controller, err := NewInstallController(
				WithRuntimeConfig(rc),
				WithStateMachine(sm),
				WithHostPreflightManager(mockPreflightManager),
				WithInstallationManager(mockInstallationManager),
				WithInfraManager(mockInfraManager),
				WithAppInstallController(appInstallController),
				WithAllowIgnoreHostPreflights(tt.serverAllowIgnoreHostPreflights),
				WithMetricsReporter(mockMetricsReporter),
				WithReleaseData(getTestReleaseData(&appConfig)),
				WithLicense([]byte("spec:\n  licenseID: test-license\n")),
				WithStore(mockStore),
			)
			require.NoError(t, err)

			err = controller.SetupInfra(t.Context(), tt.clientIgnoreHostPreflights)

			if tt.expectedErr != nil {
				require.Error(t, err)

				// Check for specific error types
				var expectedAPIErr *types.APIError
				if errors.As(tt.expectedErr, &expectedAPIErr) {
					// For API errors, check the exact type and status code
					var actualAPIErr *types.APIError
					require.True(t, errors.As(err, &actualAPIErr), "expected error to be of type *types.APIError, got %T", err)
					assert.Equal(t, expectedAPIErr.StatusCode, actualAPIErr.StatusCode, "status codes should match")
					assert.Contains(t, actualAPIErr.Error(), expectedAPIErr.Unwrap().Error(), "error messages should contain expected content")
				}
			} else {
				require.NoError(t, err)
			}

			assert.Eventually(t, func() bool {
				return sm.CurrentState() == tt.expectedState
			}, time.Second, 100*time.Millisecond, "state should be %s", tt.expectedState)

			assert.Eventually(t, func() bool {
				return !sm.IsLockAcquired()
			}, time.Second, 100*time.Millisecond, "state machine should not be locked after running infra setup")

			mockPreflightManager.AssertExpectations(t)
			mockInstallationManager.AssertExpectations(t)
			mockInfraManager.AssertExpectations(t)
			mockStore.LinuxInfraMockStore.AssertExpectations(t)
			mockStore.LinuxInstallationMockStore.AssertExpectations(t)
			mockStore.LinuxPreflightMockStore.AssertExpectations(t)
			mockStore.AppConfigMockStore.AssertExpectations(t)

			// Wait for the event handler goroutine to complete
			// TODO: find a better way to do this
			time.Sleep(1 * time.Second)
			mockMetricsReporter.AssertExpectations(t)
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
				infra := types.Infra{
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
				m.On("Get").Return(infra, nil)
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
				m.On("Get").Return(nil, errors.New("get infra error"))
			},
			expectedErr:   true,
			expectedValue: types.Infra{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &infra.MockInfraManager{}
			tt.setupMock(mockManager)

			controller, err := NewInstallController(
				WithInfraManager(mockManager),
				WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
			)
			require.NoError(t, err)

			result, err := controller.GetInfra(t.Context())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Equal(t, types.Infra{}, result)
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

type testEnvSetter struct {
	env map[string]string
}

func (e *testEnvSetter) Setenv(key string, val string) error {
	if e.env == nil {
		e.env = make(map[string]string)
	}
	e.env[key] = val
	return nil
}
