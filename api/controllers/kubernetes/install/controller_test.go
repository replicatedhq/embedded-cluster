package install

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/kubernetes/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/kubernetes/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
)

func TestGetInstallationConfig(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*installation.MockInstallationManager)
		expectedErr   bool
		expectedValue types.KubernetesInstallationConfig
	}{
		{
			name: "successful get",
			setupMock: func(m *installation.MockInstallationManager) {
				config := types.KubernetesInstallationConfig{
					AdminConsolePort: 9000,
					HTTPProxy:        "http://proxy.example.com:3128",
					HTTPSProxy:       "https://proxy.example.com:3128",
					NoProxy:          "localhost,127.0.0.1",
				}

				mock.InOrder(
					m.On("GetConfig").Return(config, nil),
					m.On("SetConfigDefaults", &config).Return(nil),
					m.On("ValidateConfig", config, 9001).Return(nil),
				)
			},
			expectedErr: false,
			expectedValue: types.KubernetesInstallationConfig{
				AdminConsolePort: 9000,
				HTTPProxy:        "http://proxy.example.com:3128",
				HTTPSProxy:       "https://proxy.example.com:3128",
				NoProxy:          "localhost,127.0.0.1",
			},
		},
		{
			name: "read config error",
			setupMock: func(m *installation.MockInstallationManager) {
				m.On("GetConfig").Return(types.KubernetesInstallationConfig{}, errors.New("read error"))
			},
			expectedErr:   true,
			expectedValue: types.KubernetesInstallationConfig{},
		},
		{
			name: "set defaults error",
			setupMock: func(m *installation.MockInstallationManager) {
				config := types.KubernetesInstallationConfig{}
				mock.InOrder(
					m.On("GetConfig").Return(config, nil),
					m.On("SetConfigDefaults", &config).Return(errors.New("defaults error")),
				)
			},
			expectedErr:   true,
			expectedValue: types.KubernetesInstallationConfig{},
		},
		{
			name: "validate error",
			setupMock: func(m *installation.MockInstallationManager) {
				config := types.KubernetesInstallationConfig{}
				mock.InOrder(
					m.On("GetConfig").Return(config, nil),
					m.On("SetConfigDefaults", &config).Return(nil),
					m.On("ValidateConfig", config, 9001).Return(errors.New("validation error")),
				)
			},
			expectedErr:   true,
			expectedValue: types.KubernetesInstallationConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockInstallation := &kubernetesinstallation.MockInstallation{}
			mockInstallation.On("ManagerPort").Return(9001)

			mockManager := &installation.MockInstallationManager{}
			tt.setupMock(mockManager)

			controller, err := NewInstallController(
				WithInstallation(mockInstallation),
				WithInstallationManager(mockManager),
			)
			require.NoError(t, err)

			result, err := controller.GetInstallationConfig(t.Context())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Equal(t, types.KubernetesInstallationConfig{}, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}

			mockManager.AssertExpectations(t)
			mockInstallation.AssertExpectations(t)
		})
	}
}

func TestConfigureInstallation(t *testing.T) {
	tests := []struct {
		name          string
		config        types.KubernetesInstallationConfig
		currentState  statemachine.State
		expectedState statemachine.State
		setupMock     func(*installation.MockInstallationManager, *kubernetesinstallation.MockInstallation, types.KubernetesInstallationConfig)
		expectedErr   bool
	}{
		{
			name: "successful configure installation",
			config: types.KubernetesInstallationConfig{
				AdminConsolePort: 9000,
				HTTPProxy:        "http://proxy.example.com:3128",
				HTTPSProxy:       "https://proxy.example.com:3128",
				NoProxy:          "localhost,127.0.0.1",
			},
			currentState:  StateNew,
			expectedState: StateInstallationConfigured,
			setupMock: func(m *installation.MockInstallationManager, ki *kubernetesinstallation.MockInstallation, config types.KubernetesInstallationConfig) {
				mock.InOrder(
					m.On("ValidateConfig", config, 9001).Return(nil),
					m.On("SetConfig", config).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "validate error",
			config:        types.KubernetesInstallationConfig{},
			currentState:  StateNew,
			expectedState: StateInstallationConfigurationFailed,
			setupMock: func(m *installation.MockInstallationManager, ki *kubernetesinstallation.MockInstallation, config types.KubernetesInstallationConfig) {
				m.On("ValidateConfig", config, 9001).Return(errors.New("validation error"))
			},
			expectedErr: false,
		},
		{
			name:          "set config error",
			config:        types.KubernetesInstallationConfig{},
			currentState:  StateNew,
			expectedState: StateInstallationConfigurationFailed,
			setupMock: func(m *installation.MockInstallationManager, ki *kubernetesinstallation.MockInstallation, config types.KubernetesInstallationConfig) {
				mock.InOrder(
					m.On("ValidateConfig", config, 9001).Return(nil),
					m.On("SetConfig", config).Return(errors.New("set config error")),
				)
			},
			expectedErr: false,
		},
		{
			name: "invalid state transition",
			config: types.KubernetesInstallationConfig{
				AdminConsolePort: 9000,
			},
			currentState:  StateInfrastructureInstalling,
			expectedState: StateInfrastructureInstalling,
			setupMock: func(m *installation.MockInstallationManager, ki *kubernetesinstallation.MockInstallation, config types.KubernetesInstallationConfig) {
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockInstallation := &kubernetesinstallation.MockInstallation{}
			mockInstallation.On("ManagerPort").Return(9001)

			sm := NewStateMachine(WithCurrentState(tt.currentState))

			mockManager := &installation.MockInstallationManager{}

			tt.setupMock(mockManager, mockInstallation, tt.config)

			controller, err := NewInstallController(
				WithInstallation(mockInstallation),
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
			assert.False(t, sm.IsLockAcquired(), "state machine should not be locked after configuration")

			mockManager.AssertExpectations(t)
			mockInstallation.AssertExpectations(t)
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
				m.On("GetStatus").Return(types.Status{}, errors.New("get status error"))
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
		setupMocks                      func(*installation.MockInstallationManager, *infra.MockInfraManager, *metrics.MockReporter, *kubernetesinstallation.MockInstallation)
		expectedErr                     error
	}{
		{
			name:                            "successful setup from installation configured",
			clientIgnoreHostPreflights:      false,
			serverAllowIgnoreHostPreflights: true,
			currentState:                    StateInstallationConfigured,
			expectedState:                   StateSucceeded,
			setupMocks: func(im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter, ki *kubernetesinstallation.MockInstallation) {
				mock.InOrder(
					fm.On("Install", mock.Anything, mock.Anything).Return(nil),
				)
			},
			expectedErr: nil,
		},
		{
			name:                            "install infra error",
			clientIgnoreHostPreflights:      false,
			serverAllowIgnoreHostPreflights: true,
			currentState:                    StateInstallationConfigured,
			expectedState:                   StateInfrastructureInstallFailed,
			setupMocks: func(im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter, ki *kubernetesinstallation.MockInstallation) {
				mock.InOrder(
					fm.On("Install", mock.Anything, mock.Anything).Return(errors.New("install error")),
				)
			},
			expectedErr: nil,
		},
		{
			name:                            "invalid state transition from installation configuration failed",
			clientIgnoreHostPreflights:      false,
			serverAllowIgnoreHostPreflights: true,
			currentState:                    StateInstallationConfigurationFailed,
			expectedState:                   StateInstallationConfigurationFailed,
			setupMocks: func(im *installation.MockInstallationManager, fm *infra.MockInfraManager, r *metrics.MockReporter, ki *kubernetesinstallation.MockInstallation) {
			},
			expectedErr: errors.New("invalid transition"), // Just check that an error occurs, don't care about exact message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewStateMachine(WithCurrentState(tt.currentState))

			mockInstallation := &kubernetesinstallation.MockInstallation{}
			mockInstallation.On("ManagerPort").Return(9001)

			mockInstallationManager := &installation.MockInstallationManager{}
			mockInfraManager := &infra.MockInfraManager{}
			mockMetricsReporter := &metrics.MockReporter{}
			tt.setupMocks(mockInstallationManager, mockInfraManager, mockMetricsReporter, mockInstallation)

			controller, err := NewInstallController(
				WithInstallation(mockInstallation),
				WithStateMachine(sm),
				WithInstallationManager(mockInstallationManager),
				WithInfraManager(mockInfraManager),
				WithMetricsReporter(mockMetricsReporter),
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
			assert.False(t, sm.IsLockAcquired(), "state machine should not be locked after running infra setup")

			mockInstallationManager.AssertExpectations(t)
			mockInfraManager.AssertExpectations(t)
			mockMetricsReporter.AssertExpectations(t)
			mockInstallation.AssertExpectations(t)
		})
	}
}

func TestGetInfra(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*infra.MockInfraManager)
		expectedErr   bool
		expectedValue types.KubernetesInfra
	}{
		{
			name: "successful get infra",
			setupMock: func(m *infra.MockInfraManager) {
				infra := types.KubernetesInfra{
					Components: []types.KubernetesInfraComponent{
						{
							Name: "SomeComponent",
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
			expectedValue: types.KubernetesInfra{
				Components: []types.KubernetesInfraComponent{
					{
						Name: "SomeComponent",
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
				m.On("Get").Return(types.KubernetesInfra{}, errors.New("get infra error"))
			},
			expectedErr:   true,
			expectedValue: types.KubernetesInfra{},
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
				assert.Equal(t, types.KubernetesInfra{}, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}

			mockManager.AssertExpectations(t)
		})
	}
}

func getTestReleaseData() *release.ReleaseData {
	return &release.ReleaseData{
		EmbeddedClusterConfig: &ecv1beta1.Config{},
		ChannelRelease:        &release.ChannelRelease{},
	}
}
