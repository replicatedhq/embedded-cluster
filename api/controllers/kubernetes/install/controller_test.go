package install

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/kubernetes/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/kubernetesinstallation"
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
			ki := kubernetesinstallation.New(nil)
			ki.SetManagerPort(9001)

			mockManager := &installation.MockInstallationManager{}
			tt.setupMock(mockManager)

			controller, err := NewInstallController(
				WithInstallation(ki),
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
					m.On("ConfigureInstallation", mock.Anything, ki, config).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "configure installation error",
			config:        types.KubernetesInstallationConfig{},
			currentState:  StateNew,
			expectedState: StateInstallationConfigurationFailed,
			setupMock: func(m *installation.MockInstallationManager, ki *kubernetesinstallation.MockInstallation, config types.KubernetesInstallationConfig) {
				m.On("ConfigureInstallation", mock.Anything, ki, config).Return(errors.New("validation error"))
			},
			expectedErr: true,
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
