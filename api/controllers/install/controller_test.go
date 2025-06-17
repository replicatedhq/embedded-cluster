package install

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/preflight"
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
		expectedValue *types.InstallationConfig
	}{
		{
			name: "successful get",
			setupMock: func(m *installation.MockInstallationManager) {
				config := &types.InstallationConfig{
					AdminConsolePort: 9000,
					GlobalCIDR:       "10.0.0.1/16",
				}

				mock.InOrder(
					m.On("GetConfig").Return(config, nil),
					m.On("SetConfigDefaults", config).Return(nil),
					m.On("ValidateConfig", config).Return(nil),
				)
			},
			expectedErr: false,
			expectedValue: &types.InstallationConfig{
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
			expectedValue: nil,
		},
		{
			name: "set defaults error",
			setupMock: func(m *installation.MockInstallationManager) {
				config := &types.InstallationConfig{}
				mock.InOrder(
					m.On("GetConfig").Return(config, nil),
					m.On("SetConfigDefaults", config).Return(errors.New("defaults error")),
				)
			},
			expectedErr:   true,
			expectedValue: nil,
		},
		{
			name: "validate error",
			setupMock: func(m *installation.MockInstallationManager) {
				config := &types.InstallationConfig{}
				mock.InOrder(
					m.On("GetConfig").Return(config, nil),
					m.On("SetConfigDefaults", config).Return(nil),
					m.On("ValidateConfig", config).Return(errors.New("validation error")),
				)
			},
			expectedErr:   true,
			expectedValue: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
			rc.SetDataDir(t.TempDir())

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
				assert.Nil(t, result)
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
		name        string
		config      *types.InstallationConfig
		setupMock   func(*installation.MockInstallationManager, *types.InstallationConfig)
		expectedErr bool
	}{
		{
			name: "successful configure installation",
			config: &types.InstallationConfig{
				LocalArtifactMirrorPort: 9000,
				DataDirectory:           t.TempDir(),
			},
			setupMock: func(m *installation.MockInstallationManager, config *types.InstallationConfig) {
				mock.InOrder(
					m.On("ValidateConfig", config).Return(nil),
					m.On("SetConfig", *config).Return(nil),
					m.On("ConfigureHost", t.Context()).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:   "validate error",
			config: &types.InstallationConfig{},
			setupMock: func(m *installation.MockInstallationManager, config *types.InstallationConfig) {
				m.On("ValidateConfig", config).Return(errors.New("validation error"))
			},
			expectedErr: true,
		},
		{
			name:   "set config error",
			config: &types.InstallationConfig{},
			setupMock: func(m *installation.MockInstallationManager, config *types.InstallationConfig) {
				mock.InOrder(
					m.On("ValidateConfig", config).Return(nil),
					m.On("SetConfig", *config).Return(errors.New("set config error")),
				)
			},
			expectedErr: true,
		},
		{
			name: "with global CIDR",
			config: &types.InstallationConfig{
				GlobalCIDR:    "10.0.0.0/16",
				DataDirectory: t.TempDir(),
			},
			setupMock: func(m *installation.MockInstallationManager, config *types.InstallationConfig) {
				// Create a copy with expected CIDR values after computation
				configWithCIDRs := *config
				configWithCIDRs.PodCIDR = "10.0.0.0/17"
				configWithCIDRs.ServiceCIDR = "10.0.128.0/17"

				mock.InOrder(
					m.On("ValidateConfig", config).Return(nil),
					m.On("SetConfig", configWithCIDRs).Return(nil),
					m.On("ConfigureHost", t.Context()).Return(nil),
				)
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
			rc.SetDataDir(t.TempDir())

			mockManager := &installation.MockInstallationManager{}

			// Create a copy of the config to avoid modifying the original
			configCopy := *tt.config

			tt.setupMock(mockManager, &configCopy)

			controller, err := NewInstallController(
				WithRuntimeConfig(rc),
				WithInstallationManager(mockManager),
			)
			require.NoError(t, err)

			err = controller.ConfigureInstallation(t.Context(), tt.config)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

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

			config := &types.InstallationConfig{
				GlobalCIDR: tt.globalCIDR,
			}

			err = controller.computeCIDRs(config)

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
		name        string
		setupMocks  func(*preflight.MockHostPreflightManager)
		expectedErr bool
	}{
		{
			name: "successful run preflights",
			setupMocks: func(pm *preflight.MockHostPreflightManager) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", t.Context(), mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "prepare preflights error",
			setupMocks: func(pm *preflight.MockHostPreflightManager) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), mock.Anything).Return(nil, errors.New("prepare error")),
				)
			},
			expectedErr: true,
		},
		{
			name: "run preflights error",
			setupMocks: func(pm *preflight.MockHostPreflightManager) {
				mock.InOrder(
					pm.On("PrepareHostPreflights", t.Context(), mock.Anything).Return(expectedHPF, nil),
					pm.On("RunHostPreflights", t.Context(), mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec
					})).Return(errors.New("run preflights error")),
				)
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPreflightManager := &preflight.MockHostPreflightManager{}
			tt.setupMocks(mockPreflightManager)

			rc := runtimeconfig.New(nil)
			rc.SetDataDir(t.TempDir())
			rc.SetProxySpec(&ecv1beta1.ProxySpec{
				HTTPProxy:       "http://proxy.example.com",
				HTTPSProxy:      "https://proxy.example.com",
				ProvidedNoProxy: "provided-proxy.com",
				NoProxy:         "no-proxy.com",
			})

			controller, err := NewInstallController(
				WithRuntimeConfig(rc),
				WithHostPreflightManager(mockPreflightManager),
				WithReleaseData(getTestReleaseData()),
			)
			require.NoError(t, err)

			err = controller.RunHostPreflights(t.Context(), RunHostPreflightsOptions{})

			if tt.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			mockPreflightManager.AssertExpectations(t)
		})
	}
}

func TestGetHostPreflightStatus(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*preflight.MockHostPreflightManager)
		expectedErr   bool
		expectedValue *types.Status
	}{
		{
			name: "successful get status",
			setupMock: func(m *preflight.MockHostPreflightManager) {
				status := &types.Status{
					State: types.StateFailed,
				}
				m.On("GetHostPreflightStatus", t.Context()).Return(status, nil)
			},
			expectedErr: false,
			expectedValue: &types.Status{
				State: types.StateFailed,
			},
		},
		{
			name: "get status error",
			setupMock: func(m *preflight.MockHostPreflightManager) {
				m.On("GetHostPreflightStatus", t.Context()).Return(nil, errors.New("get status error"))
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

			result, err := controller.GetHostPreflightStatus(t.Context())

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
		expectedValue *types.Status
	}{
		{
			name: "successful get status",
			setupMock: func(m *installation.MockInstallationManager) {
				status := &types.Status{
					State: types.StateRunning,
				}
				m.On("GetStatus").Return(status, nil)
			},
			expectedErr: false,
			expectedValue: &types.Status{
				State: types.StateRunning,
			},
		},
		{
			name: "get status error",
			setupMock: func(m *installation.MockInstallationManager) {
				m.On("GetStatus").Return(nil, errors.New("get status error"))
			},
			expectedErr:   true,
			expectedValue: nil,
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
				assert.Nil(t, result)
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
		name        string
		setupMocks  func(*preflight.MockHostPreflightManager, *installation.MockInstallationManager, *infra.MockInfraManager, *metrics.MockReporter)
		expectedErr bool
	}{
		{
			name: "successful setup with passed preflights",
			setupMocks: func(pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter) {
				preflightStatus := &types.Status{
					State: types.StateSucceeded,
				}
				config := &types.InstallationConfig{
					AdminConsolePort: 8000,
				}
				mock.InOrder(
					pm.On("GetHostPreflightStatus", t.Context()).Return(preflightStatus, nil),
					im.On("GetConfig").Return(config, nil),
					fm.On("Install", t.Context(), config).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "successful setup with failed preflights",
			setupMocks: func(pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter) {
				preflightStatus := &types.Status{
					State: types.StateFailed,
				}
				preflightOutput := &types.HostPreflightsOutput{
					Fail: []types.HostPreflightsRecord{
						{
							Title:   "Test Check",
							Message: "Test check failed",
						},
					},
				}
				config := &types.InstallationConfig{
					AdminConsolePort: 8000,
				}
				mock.InOrder(
					pm.On("GetHostPreflightStatus", t.Context()).Return(preflightStatus, nil),
					pm.On("GetHostPreflightOutput", t.Context()).Return(preflightOutput, nil),
					r.On("ReportPreflightsFailed", t.Context(), preflightOutput).Return(nil),
					im.On("GetConfig").Return(config, nil),
					fm.On("Install", t.Context(), config).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "preflight status error",
			setupMocks: func(pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter) {
				pm.On("GetHostPreflightStatus", t.Context()).Return(nil, errors.New("get preflight status error"))
			},
			expectedErr: true,
		},
		{
			name: "preflight not completed",
			setupMocks: func(pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter) {
				preflightStatus := &types.Status{
					State: types.StateRunning,
				}
				pm.On("GetHostPreflightStatus", t.Context()).Return(preflightStatus, nil)
			},
			expectedErr: true,
		},
		{
			name: "preflight output error",
			setupMocks: func(pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter) {
				preflightStatus := &types.Status{
					State: types.StateFailed,
				}
				mock.InOrder(
					pm.On("GetHostPreflightStatus", t.Context()).Return(preflightStatus, nil),
					pm.On("GetHostPreflightOutput", t.Context()).Return(nil, errors.New("get output error")),
				)
			},
			expectedErr: true,
		},
		{
			name: "get config error",
			setupMocks: func(pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter) {
				preflightStatus := &types.Status{
					State: types.StateSucceeded,
				}
				mock.InOrder(
					pm.On("GetHostPreflightStatus", t.Context()).Return(preflightStatus, nil),
					im.On("GetConfig").Return(nil, errors.New("get config error")),
				)
			},
			expectedErr: true,
		},
		{
			name: "install infra error",
			setupMocks: func(pm *preflight.MockHostPreflightManager, im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter) {
				preflightStatus := &types.Status{
					State: types.StateSucceeded,
				}
				config := &types.InstallationConfig{
					AdminConsolePort: 8000,
				}
				mock.InOrder(
					pm.On("GetHostPreflightStatus", t.Context()).Return(preflightStatus, nil),
					im.On("GetConfig").Return(config, nil),
					fm.On("Install", t.Context(), config).Return(errors.New("install error")),
				)
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPreflightManager := &preflight.MockHostPreflightManager{}
			mockInstallationManager := &installation.MockInstallationManager{}
			mockInfraManager := &infra.MockInfraManager{}
			mockMetricsReporter := &metrics.MockReporter{}
			tt.setupMocks(mockPreflightManager, mockInstallationManager, mockInfraManager, mockMetricsReporter)

			controller, err := NewInstallController(
				WithHostPreflightManager(mockPreflightManager),
				WithInstallationManager(mockInstallationManager),
				WithInfraManager(mockInfraManager),
				WithMetricsReporter(mockMetricsReporter),
			)
			require.NoError(t, err)

			err = controller.SetupInfra(t.Context())

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

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
		expectedValue *types.Infra
	}{
		{
			name: "successful get infra",
			setupMock: func(m *infra.MockInfraManager) {
				infra := &types.Infra{
					Components: []types.InfraComponent{
						{
							Name: infra.K0sComponentName,
							Status: &types.Status{
								State: types.StateRunning,
							},
						},
					},
					Status: &types.Status{
						State: types.StateRunning,
					},
				}
				m.On("Get").Return(infra, nil)
			},
			expectedErr: false,
			expectedValue: &types.Infra{
				Components: []types.InfraComponent{
					{
						Name: infra.K0sComponentName,
						Status: &types.Status{
							State: types.StateRunning,
						},
					},
				},
				Status: &types.Status{
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
			expectedValue: nil,
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
				assert.Nil(t, result)
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
		install       *types.Install
		expectedValue *types.Status
	}{
		{
			name: "successful get status",
			install: &types.Install{
				Status: &types.Status{
					State: types.StateFailed,
				},
			},
			expectedValue: &types.Status{
				State: types.StateFailed,
			},
		},
		{
			name:          "nil status",
			install:       &types.Install{},
			expectedValue: nil,
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
		status      *types.Status
		expectedErr bool
	}{
		{
			name: "successful set status",
			status: &types.Status{
				State: types.StateFailed,
			},
			expectedErr: false,
		},
		{
			name:        "nil status",
			status:      nil,
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

func WithInfraManager(infraManager infra.InfraManager) InstallControllerOption {
	return func(c *InstallController) {
		c.infraManager = infraManager
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

func TestInstallControllerIgnoreHostPreflights(t *testing.T) {
	tests := []struct {
		name                     string
		ignoreHostPreflights     bool
		setupMock                func(*installation.MockInstallationManager)
		expectedIgnorePreflights bool
	}{
		{
			name:                 "ignore host preflights enabled",
			ignoreHostPreflights: true,
			setupMock: func(m *installation.MockInstallationManager) {
				// Config returned by GetConfig() does NOT have IgnoreHostPreflights set
				// This is realistic - the controller will set it
				config := &types.InstallationConfig{
					AdminConsolePort: 8800,
					GlobalCIDR:       "10.0.0.0/16",
				}

				mock.InOrder(
					m.On("GetConfig").Return(config, nil),
					m.On("SetConfigDefaults", config).Return(nil),
					m.On("ValidateConfig", config).Return(nil),
				)
			},
			expectedIgnorePreflights: true,
		},
		{
			name:                 "ignore host preflights disabled",
			ignoreHostPreflights: false,
			setupMock: func(m *installation.MockInstallationManager) {
				// Config returned by GetConfig() does NOT have IgnoreHostPreflights set
				config := &types.InstallationConfig{
					AdminConsolePort: 8800,
					GlobalCIDR:       "10.0.0.0/16",
				}

				mock.InOrder(
					m.On("GetConfig").Return(config, nil),
					m.On("SetConfigDefaults", config).Return(nil),
					m.On("ValidateConfig", config).Return(nil),
				)
			},
			expectedIgnorePreflights: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
			rc.SetDataDir(t.TempDir())

			mockManager := &installation.MockInstallationManager{}
			tt.setupMock(mockManager)

			// Create controller with the ignoreHostPreflights flag
			installController, err := NewInstallController(
				WithRuntimeConfig(rc),
				WithInstallationManager(mockManager),
				WithIgnoreHostPreflights(tt.ignoreHostPreflights),
			)
			require.NoError(t, err)

			// Call GetInstallationConfig() - this should set the flag on the config
			resultConfig, err := installController.GetInstallationConfig(t.Context())
			require.NoError(t, err)

			// Verify the returned config has the flag set correctly
			assert.Equal(t, tt.expectedIgnorePreflights, resultConfig.IgnoreHostPreflights)
			mockManager.AssertExpectations(t)
		})
	}
}
