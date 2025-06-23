package install

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/preflight"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func TestGetInstallationConfig(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*installation.MockInstallationManager)
		expectedErr   bool
		expectedValue types.InstallationConfig
	}{
		{
			name: "successful get",
			setupMock: func(m *installation.MockInstallationManager) {
				config := types.InstallationConfig{
					AdminConsolePort: 9000,
					GlobalCIDR:       "10.0.0.1/16",
				}

				mock.InOrder(
					m.On("GetConfig").Return(config, nil),
					m.On("SetConfigDefaults", &config).Return(nil),
					m.On("ValidateConfig", config, 9001).Return(nil),
				)
			},
			expectedErr: false,
			expectedValue: types.InstallationConfig{
				AdminConsolePort: 9000,
				GlobalCIDR:       "10.0.0.1/16",
			},
		},
		{
			name: "read config error",
			setupMock: func(m *installation.MockInstallationManager) {
				m.On("GetConfig").Return(nil, errors.New("read error"))
			},
			expectedErr:   true,
			expectedValue: types.InstallationConfig{},
		},
		{
			name: "set defaults error",
			setupMock: func(m *installation.MockInstallationManager) {
				config := types.InstallationConfig{}
				mock.InOrder(
					m.On("GetConfig").Return(config, nil),
					m.On("SetConfigDefaults", &config).Return(errors.New("defaults error")),
				)
			},
			expectedErr:   true,
			expectedValue: types.InstallationConfig{},
		},
		{
			name: "validate error",
			setupMock: func(m *installation.MockInstallationManager) {
				config := types.InstallationConfig{}
				mock.InOrder(
					m.On("GetConfig").Return(config, nil),
					m.On("SetConfigDefaults", &config).Return(nil),
					m.On("ValidateConfig", config, 9001).Return(errors.New("validation error")),
				)
			},
			expectedErr:   true,
			expectedValue: types.InstallationConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
			rc.SetDataDir(t.TempDir())
			rc.SetManagerPort(9001)

			mockManager := &installation.MockInstallationManager{}
			tt.setupMock(mockManager)

			controller, err := NewInstallController(
				WithRuntimeConfig(rc),
				WithInstallationManager(mockManager),
			)
			require.NoError(t, err)

			result, err := controller.GetInstallationConfig(t.Context())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Equal(t, types.InstallationConfig{}, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}

			mockManager.AssertExpectations(t)
		})
	}
}

func TestConfigureInstallation(t *testing.T) {
	tests := []struct {
		name          string
		config        types.InstallationConfig
		currentState  statemachine.State
		expectedState statemachine.State
		setupMock     func(*installation.MockInstallationManager, runtimeconfig.RuntimeConfig, types.InstallationConfig)
		expectedErr   bool
	}{
		{
			name: "successful configure installation",
			config: types.InstallationConfig{
				LocalArtifactMirrorPort: 9000,
				DataDirectory:           t.TempDir(),
			},
			currentState:  StateNew,
			expectedState: StateHostConfigured,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.InstallationConfig) {
				mock.InOrder(
					m.On("ValidateConfig", config, 9001).Return(nil),
					m.On("SetConfig", config).Return(nil),
					m.On("ConfigureHost", mock.Anything, rc).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "validate error",
			config:        types.InstallationConfig{},
			currentState:  StateNew,
			expectedState: StateNew,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.InstallationConfig) {
				m.On("ValidateConfig", config, 9001).Return(errors.New("validation error"))
			},
			expectedErr: true,
		},
		{
			name:          "set config error",
			config:        types.InstallationConfig{},
			currentState:  StateNew,
			expectedState: StateNew,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.InstallationConfig) {
				mock.InOrder(
					m.On("ValidateConfig", config, 9001).Return(nil),
					m.On("SetConfig", config).Return(errors.New("set config error")),
				)
			},
			expectedErr: true,
		},
		{
			name: "configure host error",
			config: types.InstallationConfig{
				LocalArtifactMirrorPort: 9000,
				DataDirectory:           t.TempDir(),
			},
			currentState:  StateNew,
			expectedState: StateInstallationConfigured,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.InstallationConfig) {
				mock.InOrder(
					m.On("ValidateConfig", config, 9001).Return(nil),
					m.On("SetConfig", config).Return(nil),
					m.On("ConfigureHost", mock.Anything, rc).Return(errors.New("configure host error")),
				)
			},
			expectedErr: false,
		},
		{
			name: "with global CIDR",
			config: types.InstallationConfig{
				GlobalCIDR:    "10.0.0.0/16",
				DataDirectory: t.TempDir(),
			},
			currentState:  StateNew,
			expectedState: StateHostConfigured,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.InstallationConfig) {
				// Create a copy with expected CIDR values after computation
				configWithCIDRs := config
				configWithCIDRs.PodCIDR = "10.0.0.0/17"
				configWithCIDRs.ServiceCIDR = "10.0.128.0/17"

				mock.InOrder(
					m.On("ValidateConfig", config, 9001).Return(nil),
					m.On("SetConfig", configWithCIDRs).Return(nil),
					m.On("ConfigureHost", mock.Anything, rc).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "invalid state transition",
			config: types.InstallationConfig{
				LocalArtifactMirrorPort: 9000,
				DataDirectory:           t.TempDir(),
			},
			currentState:  StateInfrastructureInstalling,
			expectedState: StateInfrastructureInstalling,
			setupMock: func(m *installation.MockInstallationManager, rc runtimeconfig.RuntimeConfig, config types.InstallationConfig) {
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

			tt.setupMock(mockManager, rc, tt.config)

			controller, err := NewInstallController(
				WithRuntimeConfig(rc),
				WithStateMachine(sm),
				WithInstallationManager(mockManager),
			)
			require.NoError(t, err)

			err = controller.ConfigureInstallation(t.Context(), tt.config)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				assert.NotEqual(t, tt.currentState, sm.CurrentState(), "state should have changed and should not be %s", tt.currentState)
			}

			assert.Eventually(t, func() bool {
				return sm.CurrentState() == tt.expectedState
			}, time.Second, 100*time.Millisecond, "state should be %s but is %s", tt.expectedState, sm.CurrentState())

			mockManager.AssertExpectations(t)
		})
	}
}

// TestIntegrationComputeCIDRs tests the CIDR computation with real networking utility
func TestIntegrationComputeCIDRs(t *testing.T) {
	tests := []struct {
		name        string
		globalCIDR  string
		expectedPod string
		expectedSvc string
		expectedErr bool
	}{
		{
			name:        "valid cidr 10.0.0.0/16",
			globalCIDR:  "10.0.0.0/16",
			expectedPod: "10.0.0.0/17",
			expectedSvc: "10.0.128.0/17",
			expectedErr: false,
		},
		{
			name:        "valid cidr 192.168.0.0/16",
			globalCIDR:  "192.168.0.0/16",
			expectedPod: "192.168.0.0/17",
			expectedSvc: "192.168.128.0/17",
			expectedErr: false,
		},
		{
			name:        "no global cidr",
			globalCIDR:  "",
			expectedPod: "", // Should remain unchanged
			expectedSvc: "", // Should remain unchanged
			expectedErr: false,
		},
		{
			name:        "invalid cidr",
			globalCIDR:  "not-a-cidr",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller, err := NewInstallController()
			require.NoError(t, err)

			config := types.InstallationConfig{
				GlobalCIDR: tt.globalCIDR,
			}

			err = controller.computeCIDRs(&config)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPod, config.PodCIDR)
				assert.Equal(t, tt.expectedSvc, config.ServiceCIDR)
			}
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
		setupMocks    func(*preflight.MockHostPreflightManager, runtimeconfig.RuntimeConfig)
		expectedErr   bool
	}{
		{
			name:          "successful run preflights",
			currentState:  StateHostConfigured,
			expectedState: StatePreflightsSucceeded,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", mock.Anything, rc, mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "prepare preflights error",
			currentState:  StateHostConfigured,
			expectedState: StateHostConfigured,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), rc, mock.Anything).Return(nil, errors.New("prepare error")),
				)
			},
			expectedErr: true,
		},
		{
			name:          "run preflights error",
			currentState:  StateHostConfigured,
			expectedState: StatePreflightsFailed,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig) {
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
			name:          "invalid state transition",
			currentState:  StateInfrastructureInstalling,
			expectedState: StateInfrastructureInstalling,
			setupMocks: func(pm *preflight.MockHostPreflightManager, rc runtimeconfig.RuntimeConfig) {
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
			tt.setupMocks(mockPreflightManager, rc)

			controller, err := NewInstallController(
				WithRuntimeConfig(rc),
				WithStateMachine(sm),
				WithHostPreflightManager(mockPreflightManager),
				WithReleaseData(getTestReleaseData()),
			)
			require.NoError(t, err)

			err = controller.RunHostPreflights(t.Context(), RunHostPreflightsOptions{})

			if tt.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				assert.NotEqual(t, sm.CurrentState(), tt.currentState, "state should have changed and should not be %s", tt.currentState)
			}

			assert.Eventually(t, func() bool {
				return sm.CurrentState() == tt.expectedState
			}, time.Second, 100*time.Millisecond, "state should be %s but is %s", tt.expectedState, sm.CurrentState())

			mockPreflightManager.AssertExpectations(t)
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

			controller, err := NewInstallController(WithHostPreflightManager(mockManager))
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
		expectedValue *types.HostPreflightsOutput
	}{
		{
			name: "successful get output",
			setupMock: func(m *preflight.MockHostPreflightManager) {
				output := &types.HostPreflightsOutput{
					Pass: []types.HostPreflightsRecord{
						{
							Title:   "Test Check",
							Message: "Test check passed",
						},
					},
				}
				m.On("GetHostPreflightOutput", t.Context()).Return(output, nil)
			},
			expectedErr: false,
			expectedValue: &types.HostPreflightsOutput{
				Pass: []types.HostPreflightsRecord{
					{
						Title:   "Test Check",
						Message: "Test check passed",
					},
				},
			},
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

			controller, err := NewInstallController(WithHostPreflightManager(mockManager))
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

			controller, err := NewInstallController(WithHostPreflightManager(mockManager))
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

			controller, err := NewInstallController(WithInstallationManager(mockManager))
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
	tests := []struct {
		name                            string
		clientIgnoreHostPreflights      bool // From HTTP request
		serverAllowIgnoreHostPreflights bool // From CLI flag
		currentState                    statemachine.State
		expectedState                   statemachine.State
		setupMocks                      func(runtimeconfig.RuntimeConfig, *preflight.MockHostPreflightManager, *installation.MockInstallationManager, *infra.MockInfraManager, *metrics.MockReporter)
		expectedErr                     error
	}{
		{
			name:                            "successful setup with passed preflights",
			clientIgnoreHostPreflights:      false,
			serverAllowIgnoreHostPreflights: true,
			currentState:                    StatePreflightsSucceeded,
			expectedState:                   StateSucceeded,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter) {
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
			currentState:                    StatePreflightsFailed,
			expectedState:                   StateSucceeded,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter) {
				preflightOutput := &types.HostPreflightsOutput{
					Fail: []types.HostPreflightsRecord{
						{
							Title:   "Test Check",
							Message: "Test check failed",
						},
					},
				}
				mock.InOrder(
					pm.On("GetHostPreflightOutput", t.Context()).Return(preflightOutput, nil),
					r.On("ReportPreflightsBypassed", t.Context(), preflightOutput).Return(nil),
					fm.On("Install", mock.Anything, rc).Return(nil),
				)
			},
			expectedErr: nil,
		},
		{
			name:                            "failed setup with failed preflights - not ignored",
			clientIgnoreHostPreflights:      false,
			serverAllowIgnoreHostPreflights: true,
			currentState:                    StatePreflightsFailed,
			expectedState:                   StatePreflightsFailed,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter) {
			},
			expectedErr: types.NewBadRequestError(ErrPreflightChecksFailed),
		},
		{
			name:                            "preflight output error",
			clientIgnoreHostPreflights:      true,
			serverAllowIgnoreHostPreflights: true,
			currentState:                    StatePreflightsFailed,
			expectedState:                   StatePreflightsFailed,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter) {
				mock.InOrder(
					pm.On("GetHostPreflightOutput", t.Context()).Return(nil, errors.New("get output error")),
				)
			},
			expectedErr: errors.New("any error"), // Just check that an error occurs, don't care about exact message
		},
		{
			name:                            "install infra error",
			clientIgnoreHostPreflights:      false,
			serverAllowIgnoreHostPreflights: true,
			currentState:                    StatePreflightsSucceeded,
			expectedState:                   StateFailed,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter) {
				mock.InOrder(
					fm.On("Install", mock.Anything, rc).Return(errors.New("install error")),
				)
			},
			expectedErr: nil,
		},
		{
			name:                            "invalid state transition",
			clientIgnoreHostPreflights:      false,
			serverAllowIgnoreHostPreflights: true,
			currentState:                    StateInstallationConfigured,
			expectedState:                   StateInstallationConfigured,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter) {
			},
			expectedErr: errors.New("invalid transition"), // Just check that an error occurs, don't care about exact message
		},
		{
			name:                            "failed preflights with ignore flag but CLI flag disabled",
			clientIgnoreHostPreflights:      true,
			serverAllowIgnoreHostPreflights: false,
			currentState:                    StatePreflightsFailed,
			expectedState:                   StatePreflightsFailed,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter) {
			},
			expectedErr: types.NewBadRequestError(ErrPreflightChecksFailed),
		},
		{
			name:                            "failed preflights without ignore flag and CLI flag disabled",
			clientIgnoreHostPreflights:      false,
			serverAllowIgnoreHostPreflights: false,
			currentState:                    StatePreflightsFailed,
			expectedState:                   StatePreflightsFailed,
			setupMocks: func(rc runtimeconfig.RuntimeConfig, pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter) {
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
			tt.setupMocks(rc, mockPreflightManager, mockInstallationManager, mockInfraManager, mockMetricsReporter)

			controller, err := NewInstallController(
				WithRuntimeConfig(rc),
				WithStateMachine(sm),
				WithHostPreflightManager(mockPreflightManager),
				WithInstallationManager(mockInstallationManager),
				WithInfraManager(mockInfraManager),
				WithMetricsReporter(mockMetricsReporter),
				WithAllowIgnoreHostPreflights(tt.serverAllowIgnoreHostPreflights),
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

				assert.NotEqual(t, sm.CurrentState(), tt.currentState, "state should have changed and should not be %s", tt.currentState)
			}

			assert.Eventually(t, func() bool {
				return sm.CurrentState() == tt.expectedState
			}, time.Second, 100*time.Millisecond, "state should be %s but is %s", tt.expectedState, sm.CurrentState())

			mockPreflightManager.AssertExpectations(t)
			mockInstallationManager.AssertExpectations(t)
			mockInfraManager.AssertExpectations(t)
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

			controller, err := NewInstallController(WithInfraManager(mockManager))
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

func TestGetStatus(t *testing.T) {
	tests := []struct {
		name          string
		install       types.Install
		expectedValue types.Status
	}{
		{
			name: "successful get status",
			install: types.Install{
				Status: types.Status{
					State: types.StateFailed,
				},
			},
			expectedValue: types.Status{
				State: types.StateFailed,
			},
		},
		{
			name:          "empty status",
			install:       types.Install{},
			expectedValue: types.Status{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := &InstallController{
				install: tt.install,
			}

			result, err := controller.GetStatus(t.Context())

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedValue, result)
		})
	}
}

func TestSetStatus(t *testing.T) {
	tests := []struct {
		name        string
		status      types.Status
		expectedErr bool
	}{
		{
			name: "successful set status",
			status: types.Status{
				State: types.StateFailed,
			},
			expectedErr: false,
		},
		{
			name:        "nil status",
			status:      types.Status{},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller, err := NewInstallController()
			require.NoError(t, err)

			err = controller.SetStatus(t.Context(), tt.status)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.status, controller.install.Status)
			}
		})
	}
}

func getTestReleaseData() *release.ReleaseData {
	return &release.ReleaseData{
		EmbeddedClusterConfig: &ecv1beta1.Config{},
		ChannelRelease:        &release.ChannelRelease{},
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
