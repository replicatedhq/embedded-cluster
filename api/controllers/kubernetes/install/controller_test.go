package install

import (
	"errors"
	"testing"
	"time"

	appcontroller "github.com/replicatedhq/embedded-cluster/api/controllers/app"
	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/kubernetes/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/kubernetes/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetInstallationConfig(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*installation.MockInstallationManager)
		expectedErr   bool
		expectedValue types.KubernetesInstallationConfigResponse
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

				defaults := types.KubernetesInstallationConfig{
					AdminConsolePort: 9090,
				}

				resolvedConfig := types.KubernetesInstallationConfig{
					AdminConsolePort: 9000,
					HTTPProxy:        "http://proxy.example.com:3128",
					HTTPSProxy:       "https://proxy.example.com:3128",
					NoProxy:          "localhost,127.0.0.1",
				}

				mock.InOrder(
					m.On("GetConfigValues").Return(config, nil),
					m.On("GetDefaults").Return(defaults, nil),
					m.On("GetConfig").Return(resolvedConfig, nil),
				)
			},
			expectedErr: false,
			expectedValue: types.KubernetesInstallationConfigResponse{
				Values: types.KubernetesInstallationConfig{
					AdminConsolePort: 9000,
					HTTPProxy:        "http://proxy.example.com:3128",
					HTTPSProxy:       "https://proxy.example.com:3128",
					NoProxy:          "localhost,127.0.0.1",
				},
				Defaults: types.KubernetesInstallationConfig{
					AdminConsolePort: 9090,
				},
				Resolved: types.KubernetesInstallationConfig{
					AdminConsolePort: 9000,
					HTTPProxy:        "http://proxy.example.com:3128",
					HTTPSProxy:       "https://proxy.example.com:3128",
					NoProxy:          "localhost,127.0.0.1",
				},
			},
		},
		{
			name: "read config error",
			setupMock: func(m *installation.MockInstallationManager) {
				m.On("GetConfigValues").Return(types.KubernetesInstallationConfig{}, errors.New("read error"))
			},
			expectedErr:   true,
			expectedValue: types.KubernetesInstallationConfigResponse{},
		},
		{
			name: "set defaults error",
			setupMock: func(m *installation.MockInstallationManager) {
				config := types.KubernetesInstallationConfig{}
				mock.InOrder(
					m.On("GetConfigValues").Return(config, nil),
					m.On("GetDefaults").Return(types.KubernetesInstallationConfig{}, errors.New("defaults error")),
				)
			},
			expectedErr:   true,
			expectedValue: types.KubernetesInstallationConfigResponse{},
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
				WithReleaseData(getTestReleaseData(&kotsv1beta1.Config{})),
			)
			require.NoError(t, err)

			result, err := controller.GetInstallationConfig(t.Context())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Equal(t, types.KubernetesInstallationConfigResponse{}, result)
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
		setupMock     func(*installation.MockInstallationManager, *kubernetesinstallation.MockInstallation, types.KubernetesInstallationConfig, *store.MockStore, *metrics.MockReporter)
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
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateInstallationConfigured,

			setupMock: func(m *installation.MockInstallationManager, ki *kubernetesinstallation.MockInstallation, config types.KubernetesInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
				mock.InOrder(
					m.On("ConfigureInstallation", mock.Anything, ki, config).Return(nil),
				)
			},
			expectedErr: false,
		},
		{
			name:          "configure installation error",
			config:        types.KubernetesInstallationConfig{},
			currentState:  states.StateApplicationConfigured,
			expectedState: states.StateInstallationConfigurationFailed,
			setupMock: func(m *installation.MockInstallationManager, ki *kubernetesinstallation.MockInstallation, config types.KubernetesInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
				mock.InOrder(
					m.On("ConfigureInstallation", mock.Anything, ki, config).Return(errors.New("configure installation error")),
					st.KubernetesInstallationMockStore.On("GetStatus").Return(types.Status{Description: "configure installation error"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("configure installation error")),
				)
			},
			expectedErr: true,
		},
		{
			name:          "configure installation error from configured state",
			config:        types.KubernetesInstallationConfig{},
			currentState:  states.StateInstallationConfigured,
			expectedState: states.StateInstallationConfigurationFailed,
			setupMock: func(m *installation.MockInstallationManager, ki *kubernetesinstallation.MockInstallation, config types.KubernetesInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
				mock.InOrder(
					m.On("ConfigureInstallation", mock.Anything, ki, config).Return(errors.New("validation error")),
					st.KubernetesInstallationMockStore.On("GetStatus").Return(types.Status{Description: "validation error"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("validation error")),
				)
			},
			expectedErr: true,
		},
		{
			name:          "configure installation error on retry from failed configuration",
			config:        types.KubernetesInstallationConfig{},
			currentState:  states.StateInstallationConfigurationFailed,
			expectedState: states.StateInstallationConfigurationFailed,
			setupMock: func(m *installation.MockInstallationManager, ki *kubernetesinstallation.MockInstallation, config types.KubernetesInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
				mock.InOrder(
					m.On("ConfigureInstallation", mock.Anything, ki, config).Return(errors.New("validation error")),
					st.KubernetesInstallationMockStore.On("GetStatus").Return(types.Status{Description: "validation error"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("validation error")),
				)
			},
			expectedErr: true,
		},
		{
			name:          "invalid state transition",
			config:        types.KubernetesInstallationConfig{},
			currentState:  states.StateInfrastructureInstalling,
			expectedState: states.StateInfrastructureInstalling,
			setupMock: func(m *installation.MockInstallationManager, ki *kubernetesinstallation.MockInstallation, config types.KubernetesInstallationConfig, st *store.MockStore, mr *metrics.MockReporter) {
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockInstallation := &kubernetesinstallation.MockInstallation{}

			sm := NewStateMachine(WithCurrentState(tt.currentState))

			mockManager := &installation.MockInstallationManager{}
			mockMetricsReporter := &metrics.MockReporter{}
			mockStore := &store.MockStore{}

			tt.setupMock(mockManager, mockInstallation, tt.config, mockStore, mockMetricsReporter)

			controller, err := NewInstallController(
				WithInstallation(mockInstallation),
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
			mockStore.KubernetesInfraMockStore.AssertExpectations(t)
			mockStore.KubernetesInstallationMockStore.AssertExpectations(t)
			mockStore.AppConfigMockStore.AssertExpectations(t)
			mockInstallation.AssertExpectations(t)

			// Wait for the event handler goroutine to complete
			// TODO: find a better way to do this
			time.Sleep(1 * time.Second)
			mockMetricsReporter.AssertExpectations(t)
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
		name          string
		currentState  statemachine.State
		expectedState statemachine.State
		setupMocks    func(kubernetesinstallation.Installation, *installation.MockInstallationManager, *infra.MockInfraManager, *metrics.MockReporter, *store.MockStore, *appconfig.MockAppConfigManager)
		expectedErr   error
	}{
		{
			name:          "successful setup",
			currentState:  states.StateInstallationConfigured,
			expectedState: states.StateInfrastructureInstalled,
			setupMocks: func(ki kubernetesinstallation.Installation, im *installation.MockInstallationManager, fm *infra.MockInfraManager, mr *metrics.MockReporter, st *store.MockStore, am *appconfig.MockAppConfigManager) {
				mock.InOrder(
					fm.On("Install", mock.Anything, ki).Return(nil),
					// TODO: we are not yet reporting
				)
			},
			expectedErr: nil,
		},
		{
			name:          "install infra error",
			currentState:  states.StateInstallationConfigured,
			expectedState: states.StateInfrastructureInstallFailed,
			setupMocks: func(ki kubernetesinstallation.Installation, im *installation.MockInstallationManager, fm *infra.MockInfraManager, mr *metrics.MockReporter, st *store.MockStore, am *appconfig.MockAppConfigManager) {
				mock.InOrder(
					fm.On("Install", mock.Anything, ki).Return(errors.New("install error")),
					st.KubernetesInfraMockStore.On("GetStatus").Return(types.Status{Description: "install error"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("install error")),
				)
			},
			expectedErr: nil,
		},
		{
			name:          "install infra error without report if infra store fails",
			currentState:  states.StateInstallationConfigured,
			expectedState: states.StateInfrastructureInstallFailed,
			setupMocks: func(ki kubernetesinstallation.Installation, im *installation.MockInstallationManager, fm *infra.MockInfraManager, mr *metrics.MockReporter, st *store.MockStore, am *appconfig.MockAppConfigManager) {
				mock.InOrder(
					fm.On("Install", mock.Anything, ki).Return(errors.New("install error")),
					st.KubernetesInfraMockStore.On("GetStatus").Return(nil, assert.AnError),
				)
			},
			expectedErr: nil,
		},
		{
			name:          "install infra panic",
			currentState:  states.StateInstallationConfigured,
			expectedState: states.StateInfrastructureInstallFailed,
			setupMocks: func(ki kubernetesinstallation.Installation, im *installation.MockInstallationManager, fm *infra.MockInfraManager, mr *metrics.MockReporter, st *store.MockStore, am *appconfig.MockAppConfigManager) {
				mock.InOrder(
					fm.On("Install", mock.Anything, ki).Panic("this is a panic"),
					st.KubernetesInfraMockStore.On("GetStatus").Return(types.Status{Description: "this is a panic"}, nil),
					mr.On("ReportInstallationFailed", mock.Anything, errors.New("this is a panic")),
				)
			},
			expectedErr: nil,
		},
		{
			name:          "invalid state transition",
			currentState:  states.StateNew,
			expectedState: states.StateNew,
			setupMocks: func(ki kubernetesinstallation.Installation, im *installation.MockInstallationManager, fm *infra.MockInfraManager, mr *metrics.MockReporter, st *store.MockStore, am *appconfig.MockAppConfigManager) {
			},
			expectedErr: assert.AnError, // Just check that an error occurs, don't care about exact message
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sm := NewStateMachine(WithCurrentState(tt.currentState))

			ki := kubernetesinstallation.New(nil)
			ki.SetManagerPort(9001)

			mockInstallationManager := &installation.MockInstallationManager{}
			mockInfraManager := &infra.MockInfraManager{}
			mockMetricsReporter := &metrics.MockReporter{}
			mockStore := &store.MockStore{}
			mockAppConfigManager := &appconfig.MockAppConfigManager{}
			tt.setupMocks(ki, mockInstallationManager, mockInfraManager, mockMetricsReporter, mockStore, mockAppConfigManager)

			appController, err := appcontroller.NewAppController(
				appcontroller.WithStateMachine(sm),
				appcontroller.WithStore(mockStore),
				appcontroller.WithReleaseData(getTestReleaseData(&appConfig)),
				appcontroller.WithAppConfigManager(mockAppConfigManager),
			)
			require.NoError(t, err)

			controller, err := NewInstallController(
				WithInstallation(ki),
				WithStateMachine(sm),
				WithInstallationManager(mockInstallationManager),
				WithInfraManager(mockInfraManager),
				WithAppController(appController),
				WithMetricsReporter(mockMetricsReporter),
				WithReleaseData(getTestReleaseData(&appConfig)),
				WithStore(mockStore),
			)
			require.NoError(t, err)

			err = controller.SetupInfra(t.Context())

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

			// Wait for the goroutine to complete and state to transition
			assert.Eventually(t, func() bool {
				return sm.CurrentState() == tt.expectedState
			}, time.Second, 100*time.Millisecond, "state should be %s but is %s", tt.expectedState, sm.CurrentState())

			assert.Eventually(t, func() bool {
				return !sm.IsLockAcquired()
			}, time.Second, 100*time.Millisecond, "state machine should not be locked")

			mockInstallationManager.AssertExpectations(t)
			mockInfraManager.AssertExpectations(t)
			mockStore.KubernetesInfraMockStore.AssertExpectations(t)
			mockStore.KubernetesInstallationMockStore.AssertExpectations(t)

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
							Name: "Admin Console",
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
						Name: "Admin Console",
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
