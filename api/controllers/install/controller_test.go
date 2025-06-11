package install

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/preflight"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
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
			mockManager := &installation.MockInstallationManager{}
			tt.setupMock(mockManager)

			controller, err := NewInstallController(WithInstallationManager(mockManager))
			require.NoError(t, err)

			result, err := controller.GetInstallationConfig(context.Background())

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
				DataDirectory:           "/data/dir",
			},
			setupMock: func(m *installation.MockInstallationManager, config *types.InstallationConfig) {
				mock.InOrder(
					m.On("ValidateConfig", config).Return(nil),
					m.On("SetConfig", *config).Return(nil),
					m.On("ConfigureHost", context.Background()).Return(nil),
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &installation.MockInstallationManager{}

			// Create a copy of the config to avoid modifying the original
			configCopy := *tt.config

			tt.setupMock(mockManager, &configCopy)

			controller, err := NewInstallController(WithInstallationManager(mockManager))
			require.NoError(t, err)

			err = controller.ConfigureInstallation(context.Background(), tt.config)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockManager.AssertExpectations(t)
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

	expectedProxy := &ecv1beta1.ProxySpec{
		HTTPProxy:       "http://proxy.example.com",
		HTTPSProxy:      "https://proxy.example.com",
		ProvidedNoProxy: "provided-proxy.com",
		NoProxy:         "no-proxy.com",
	}

	tests := []struct {
		name        string
		setupMocks  func(*installation.MockInstallationManager, *preflight.MockHostPreflightManager)
		expectedErr bool
	}{
		{
			name: "successful run preflights",
			setupMocks: func(im *installation.MockInstallationManager, pm *preflight.MockHostPreflightManager) {
				mock.InOrder(
					im.On("GetConfig").Return(&types.InstallationConfig{DataDirectory: "/data/dir"}, nil),
					pm.On("PrepareHostPreflights", context.Background(), mock.Anything).Return(expectedHPF, expectedProxy, nil),
					pm.On("RunHostPreflights", context.Background(), mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec && expectedProxy == opts.Proxy
					})).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "prepare preflights error",
			setupMocks: func(im *installation.MockInstallationManager, pm *preflight.MockHostPreflightManager) {
				mock.InOrder(
					im.On("GetConfig").Return(&types.InstallationConfig{DataDirectory: "/data/dir"}, nil),
					pm.On("PrepareHostPreflights", context.Background(), mock.Anything).Return(nil, nil, errors.New("prepare error")),
				)
			},
			expectedErr: true,
		},
		{
			name: "run preflights error",
			setupMocks: func(im *installation.MockInstallationManager, pm *preflight.MockHostPreflightManager) {
				mock.InOrder(
					im.On("GetConfig").Return(&types.InstallationConfig{DataDirectory: "/data/dir"}, nil),
					pm.On("PrepareHostPreflights", context.Background(), mock.Anything).Return(expectedHPF, expectedProxy, nil),
					pm.On("RunHostPreflights", context.Background(), mock.MatchedBy(func(opts preflight.RunHostPreflightOptions) bool {
						return expectedHPF == opts.HostPreflightSpec && expectedProxy == opts.Proxy
					})).Return(errors.New("run preflights error")),
				)
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockInstallationManager := &installation.MockInstallationManager{}
			mockPreflightManager := &preflight.MockHostPreflightManager{}
			tt.setupMocks(mockInstallationManager, mockPreflightManager)

			controller, err := NewInstallController(
				WithInstallationManager(mockInstallationManager),
				WithHostPreflightManager(mockPreflightManager),
				WithReleaseData(getTestReleaseData()),
			)
			require.NoError(t, err)

			err = controller.RunHostPreflights(context.Background())

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockInstallationManager.AssertExpectations(t)
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
				m.On("GetHostPreflightStatus", context.Background()).Return(status, nil)
			},
			expectedErr: false,
			expectedValue: &types.Status{
				State: types.StateFailed,
			},
		},
		{
			name: "get status error",
			setupMock: func(m *preflight.MockHostPreflightManager) {
				m.On("GetHostPreflightStatus", context.Background()).Return(nil, errors.New("get status error"))
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

			result, err := controller.GetHostPreflightStatus(context.Background())

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
				m.On("GetHostPreflightOutput", context.Background()).Return(output, nil)
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
				m.On("GetHostPreflightOutput", context.Background()).Return(nil, errors.New("get output error"))
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

			result, err := controller.GetHostPreflightOutput(context.Background())

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
				m.On("GetHostPreflightTitles", context.Background()).Return(titles, nil)
			},
			expectedErr:   false,
			expectedValue: []string{"Check 1", "Check 2"},
		},
		{
			name: "get titles error",
			setupMock: func(m *preflight.MockHostPreflightManager) {
				m.On("GetHostPreflightTitles", context.Background()).Return(nil, errors.New("get titles error"))
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

			result, err := controller.GetHostPreflightTitles(context.Background())

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

			result, err := controller.GetInstallationStatus(context.Background())

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

func TestSetupNode(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  func(*preflight.MockHostPreflightManager, *metrics.MockReporter)
		expectedErr bool
	}{
		{
			name: "successful setup with passed preflights",
			setupMocks: func(m *preflight.MockHostPreflightManager, r *metrics.MockReporter) {
				preflightStatus := &types.Status{
					State: types.StateSucceeded,
				}
				m.On("GetHostPreflightStatus", context.Background()).Return(preflightStatus, nil)
			},
			expectedErr: false,
		},
		{
			name: "successful setup with failed preflights",
			setupMocks: func(m *preflight.MockHostPreflightManager, r *metrics.MockReporter) {
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
				mock.InOrder(
					m.On("GetHostPreflightStatus", context.Background()).Return(preflightStatus, nil),
					m.On("GetHostPreflightOutput", context.Background()).Return(preflightOutput, nil),
					r.On("ReportPreflightsFailed", context.Background(), preflightOutput).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name: "preflight status error",
			setupMocks: func(m *preflight.MockHostPreflightManager, r *metrics.MockReporter) {
				m.On("GetHostPreflightStatus", context.Background()).Return(nil, errors.New("get status error"))
			},
			expectedErr: true,
		},
		{
			name: "preflight not completed",
			setupMocks: func(m *preflight.MockHostPreflightManager, r *metrics.MockReporter) {
				preflightStatus := &types.Status{
					State: types.StateRunning,
				}
				m.On("GetHostPreflightStatus", context.Background()).Return(preflightStatus, nil)
			},
			expectedErr: true,
		},
		{
			name: "preflight output error",
			setupMocks: func(m *preflight.MockHostPreflightManager, r *metrics.MockReporter) {
				preflightStatus := &types.Status{
					State: types.StateFailed,
				}
				mock.InOrder(
					m.On("GetHostPreflightStatus", context.Background()).Return(preflightStatus, nil),
					m.On("GetHostPreflightOutput", context.Background()).Return(nil, errors.New("get output error")),
				)
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPreflightManager := &preflight.MockHostPreflightManager{}
			mockMetricsReporter := &metrics.MockReporter{}
			tt.setupMocks(mockPreflightManager, mockMetricsReporter)

			controller, err := NewInstallController(
				WithHostPreflightManager(mockPreflightManager),
				WithMetricsReporter(mockMetricsReporter),
			)
			require.NoError(t, err)

			err = controller.SetupNode(context.Background())

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockPreflightManager.AssertExpectations(t)
			mockMetricsReporter.AssertExpectations(t)
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
			mockInstallationManager := &installation.MockInstallationManager{}
			mockInstallationManager.On("GetStatus").Return(tt.install.Status, nil)

			controller := &InstallController{
				installationManager: mockInstallationManager,
			}

			result, err := controller.GetStatus(context.Background())

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller, err := NewInstallController()
			require.NoError(t, err)

			err = controller.SetStatus(context.Background(), tt.status)

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				status, err := controller.GetStatus(context.Background())
				require.NoError(t, err)
				assert.Equal(t, tt.status, *status)
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
