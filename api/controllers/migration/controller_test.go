package migration

import (
	"errors"
	"testing"
	"time"

	migrationmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/migration"
	migrationstore "github.com/replicatedhq/embedded-cluster/api/internal/store/migration"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestGetInstallationConfig(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*migrationmanager.MockManager)
		expectedErr   bool
		expectedValue func() types.LinuxInstallationConfigResponse
	}{
		{
			name: "successful read with values, defaults, and resolved configs",
			setupMock: func(m *migrationmanager.MockManager) {
				kurlConfig := types.LinuxInstallationConfig{
					AdminConsolePort: 8800,
					PodCIDR:          "10.32.0.0/20",
					ServiceCIDR:      "10.96.0.0/12",
				}

				defaults := types.LinuxInstallationConfig{
					AdminConsolePort:        30000,
					DataDirectory:           "/var/lib/embedded-cluster",
					LocalArtifactMirrorPort: 50000,
					GlobalCIDR:              "10.244.0.0/16",
				}

				resolvedConfig := types.LinuxInstallationConfig{
					AdminConsolePort:        8800,
					DataDirectory:           "/var/lib/embedded-cluster",
					LocalArtifactMirrorPort: 50000,
					PodCIDR:                 "10.32.0.0/20",
					ServiceCIDR:             "10.96.0.0/12",
					GlobalCIDR:              "10.244.0.0/16",
				}

				mock.InOrder(
					m.On("GetKurlConfig", mock.Anything).Return(kurlConfig, nil),
					m.On("GetECDefaults", mock.Anything).Return(defaults, nil),
					m.On("MergeConfigs", types.LinuxInstallationConfig{}, kurlConfig, defaults).Return(resolvedConfig),
				)
			},
			expectedErr: false,
			expectedValue: func() types.LinuxInstallationConfigResponse {
				return types.LinuxInstallationConfigResponse{
					Values: types.LinuxInstallationConfig{
						AdminConsolePort: 8800,
						PodCIDR:          "10.32.0.0/20",
						ServiceCIDR:      "10.96.0.0/12",
					},
					Defaults: types.LinuxInstallationConfig{
						AdminConsolePort:        30000,
						DataDirectory:           "/var/lib/embedded-cluster",
						LocalArtifactMirrorPort: 50000,
						GlobalCIDR:              "10.244.0.0/16",
					},
					Resolved: types.LinuxInstallationConfig{
						AdminConsolePort:        8800,
						DataDirectory:           "/var/lib/embedded-cluster",
						LocalArtifactMirrorPort: 50000,
						PodCIDR:                 "10.32.0.0/20",
						ServiceCIDR:             "10.96.0.0/12",
						GlobalCIDR:              "10.244.0.0/16",
					},
				}
			},
		},
		{
			name: "manager error on GetKurlConfig",
			setupMock: func(m *migrationmanager.MockManager) {
				m.On("GetKurlConfig", mock.Anything).Return(types.LinuxInstallationConfig{}, errors.New("kurl config error"))
			},
			expectedErr: true,
			expectedValue: func() types.LinuxInstallationConfigResponse {
				return types.LinuxInstallationConfigResponse{}
			},
		},
		{
			name: "manager error on GetECDefaults",
			setupMock: func(m *migrationmanager.MockManager) {
				kurlConfig := types.LinuxInstallationConfig{}
				mock.InOrder(
					m.On("GetKurlConfig", mock.Anything).Return(kurlConfig, nil),
					m.On("GetECDefaults", mock.Anything).Return(types.LinuxInstallationConfig{}, errors.New("defaults error")),
				)
			},
			expectedErr: true,
			expectedValue: func() types.LinuxInstallationConfigResponse {
				return types.LinuxInstallationConfigResponse{}
			},
		},
		{
			name: "verify proper config merging precedence (user > kURL > defaults)",
			setupMock: func(m *migrationmanager.MockManager) {
				kurlConfig := types.LinuxInstallationConfig{
					AdminConsolePort: 8800,
					DataDirectory:    "/opt/kurl",
				}

				defaults := types.LinuxInstallationConfig{
					AdminConsolePort:        30000,
					DataDirectory:           "/var/lib/embedded-cluster",
					LocalArtifactMirrorPort: 50000,
				}

				// Resolved should have kURL values override defaults (since no user config yet)
				resolvedConfig := types.LinuxInstallationConfig{
					AdminConsolePort:        8800,
					DataDirectory:           "/opt/kurl",
					LocalArtifactMirrorPort: 50000,
				}

				mock.InOrder(
					m.On("GetKurlConfig", mock.Anything).Return(kurlConfig, nil),
					m.On("GetECDefaults", mock.Anything).Return(defaults, nil),
					m.On("MergeConfigs", types.LinuxInstallationConfig{}, kurlConfig, defaults).Return(resolvedConfig),
				)
			},
			expectedErr: false,
			expectedValue: func() types.LinuxInstallationConfigResponse {
				return types.LinuxInstallationConfigResponse{
					Values: types.LinuxInstallationConfig{
						AdminConsolePort: 8800,
						DataDirectory:    "/opt/kurl",
					},
					Defaults: types.LinuxInstallationConfig{
						AdminConsolePort:        30000,
						DataDirectory:           "/var/lib/embedded-cluster",
						LocalArtifactMirrorPort: 50000,
					},
					Resolved: types.LinuxInstallationConfig{
						AdminConsolePort:        8800,
						DataDirectory:           "/opt/kurl",
						LocalArtifactMirrorPort: 50000,
					},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &migrationmanager.MockManager{}
			tt.setupMock(mockManager)

			controller, err := NewMigrationController(
				WithManager(mockManager),
			)
			require.NoError(t, err)

			result, err := controller.GetInstallationConfig(t.Context())

			if tt.expectedErr {
				assert.Error(t, err)
				assert.Equal(t, types.LinuxInstallationConfigResponse{}, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue(), result)
			}

			mockManager.AssertExpectations(t)
		})
	}
}

func TestStartMigration(t *testing.T) {
	tests := []struct {
		name           string
		transferMode   types.TransferMode
		config         types.LinuxInstallationConfig
		setupMock      func(*migrationmanager.MockManager, *migrationstore.MockStore)
		expectedErr    error
		validateResult func(t *testing.T, migrationID string, err error)
	}{
		{
			name:         "successful start with copy mode",
			transferMode: types.TransferModeCopy,
			config: types.LinuxInstallationConfig{
				AdminConsolePort: 9000,
				DataDirectory:    "/opt/ec",
			},
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				kurlConfig := types.LinuxInstallationConfig{
					PodCIDR:     "10.32.0.0/20",
					ServiceCIDR: "10.96.0.0/12",
				}
				defaults := types.LinuxInstallationConfig{
					AdminConsolePort: 30000,
					DataDirectory:    "/var/lib/embedded-cluster",
				}
				resolvedConfig := types.LinuxInstallationConfig{
					AdminConsolePort: 9000,
					DataDirectory:    "/opt/ec",
					PodCIDR:          "10.32.0.0/20",
					ServiceCIDR:      "10.96.0.0/12",
				}

				mock.InOrder(
					m.On("ValidateTransferMode", types.TransferModeCopy).Return(nil),
					s.On("GetMigrationID").Return("", types.ErrNoActiveMigration),
					m.On("GetKurlConfig", mock.Anything).Return(kurlConfig, nil),
					m.On("GetECDefaults", mock.Anything).Return(defaults, nil),
					m.On("MergeConfigs", mock.Anything, kurlConfig, defaults).Return(resolvedConfig),
					s.On("InitializeMigration", mock.AnythingOfType("string"), "copy", resolvedConfig).Return(nil),
					s.On("SetState", types.MigrationStateNotStarted).Return(nil),
					// For the background goroutine - skeleton implementation fails at first phase
					s.On("GetStatus").Return(types.MigrationStatusResponse{
						State: types.MigrationStateNotStarted,
						Phase: types.MigrationPhaseDiscovery,
					}, nil),
					s.On("SetState", types.MigrationStateInProgress).Return(nil),
					s.On("SetPhase", types.MigrationPhaseDiscovery).Return(nil),
					m.On("ExecutePhase", mock.Anything, types.MigrationPhaseDiscovery).Return(types.ErrMigrationPhaseNotImplemented),
					s.On("SetState", types.MigrationStateFailed).Return(nil),
					s.On("SetError", "migration phase execution not yet implemented").Return(nil),
				)
			},
			expectedErr: nil,
			validateResult: func(t *testing.T, migrationID string, err error) {
				assert.NoError(t, err)
				assert.NotEmpty(t, migrationID)
			},
		},
		{
			name:         "successful start with move mode",
			transferMode: types.TransferModeMove,
			config: types.LinuxInstallationConfig{
				AdminConsolePort: 9000,
			},
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				kurlConfig := types.LinuxInstallationConfig{}
				defaults := types.LinuxInstallationConfig{
					AdminConsolePort: 30000,
				}
				resolvedConfig := types.LinuxInstallationConfig{
					AdminConsolePort: 9000,
				}

				mock.InOrder(
					m.On("ValidateTransferMode", types.TransferModeMove).Return(nil),
					s.On("GetMigrationID").Return("", types.ErrNoActiveMigration),
					m.On("GetKurlConfig", mock.Anything).Return(kurlConfig, nil),
					m.On("GetECDefaults", mock.Anything).Return(defaults, nil),
					m.On("MergeConfigs", mock.Anything, kurlConfig, defaults).Return(resolvedConfig),
					s.On("InitializeMigration", mock.AnythingOfType("string"), "move", resolvedConfig).Return(nil),
					s.On("SetState", types.MigrationStateNotStarted).Return(nil),
					// For the background goroutine - skeleton implementation fails at first phase
					s.On("GetStatus").Return(types.MigrationStatusResponse{
						State: types.MigrationStateNotStarted,
						Phase: types.MigrationPhaseDiscovery,
					}, nil),
					s.On("SetState", types.MigrationStateInProgress).Return(nil),
					s.On("SetPhase", types.MigrationPhaseDiscovery).Return(nil),
					m.On("ExecutePhase", mock.Anything, types.MigrationPhaseDiscovery).Return(types.ErrMigrationPhaseNotImplemented),
					s.On("SetState", types.MigrationStateFailed).Return(nil),
					s.On("SetError", "migration phase execution not yet implemented").Return(nil),
				)
			},
			expectedErr: nil,
			validateResult: func(t *testing.T, migrationID string, err error) {
				assert.NoError(t, err)
				assert.NotEmpty(t, migrationID)
			},
		},
		{
			name:         "migration already started (409 error)",
			transferMode: types.TransferModeCopy,
			config:       types.LinuxInstallationConfig{},
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				mock.InOrder(
					m.On("ValidateTransferMode", types.TransferModeCopy).Return(nil),
					s.On("GetMigrationID").Return("existing-migration-id", nil),
				)
			},
			expectedErr: types.ErrMigrationAlreadyStarted,
			validateResult: func(t *testing.T, migrationID string, err error) {
				assert.Error(t, err)
				assert.Empty(t, migrationID)
				var apiErr *types.APIError
				require.True(t, errors.As(err, &apiErr))
				assert.Equal(t, 409, apiErr.StatusCode)
				assert.Contains(t, err.Error(), "migration already started")
			},
		},
		{
			name:         "invalid transfer mode (400 error)",
			transferMode: types.TransferMode("invalid"),
			config:       types.LinuxInstallationConfig{},
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				m.On("ValidateTransferMode", types.TransferMode("invalid")).Return(types.ErrInvalidTransferMode)
			},
			expectedErr: types.ErrInvalidTransferMode,
			validateResult: func(t *testing.T, migrationID string, err error) {
				assert.Error(t, err)
				assert.Empty(t, migrationID)
				var apiErr *types.APIError
				require.True(t, errors.As(err, &apiErr))
				assert.Equal(t, 400, apiErr.StatusCode)
			},
		},
		{
			name:         "store initialization error",
			transferMode: types.TransferModeCopy,
			config:       types.LinuxInstallationConfig{},
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				kurlConfig := types.LinuxInstallationConfig{}
				defaults := types.LinuxInstallationConfig{}
				resolvedConfig := types.LinuxInstallationConfig{}

				mock.InOrder(
					m.On("ValidateTransferMode", types.TransferModeCopy).Return(nil),
					s.On("GetMigrationID").Return("", types.ErrNoActiveMigration),
					m.On("GetKurlConfig", mock.Anything).Return(kurlConfig, nil),
					m.On("GetECDefaults", mock.Anything).Return(defaults, nil),
					m.On("MergeConfigs", mock.Anything, kurlConfig, defaults).Return(resolvedConfig),
					s.On("InitializeMigration", mock.AnythingOfType("string"), "copy", resolvedConfig).Return(errors.New("store error")),
				)
			},
			expectedErr: errors.New("store error"),
			validateResult: func(t *testing.T, migrationID string, err error) {
				assert.Error(t, err)
				assert.Empty(t, migrationID)
				assert.Contains(t, err.Error(), "initialize migration")
			},
		},
		{
			name:         "state set to NotStarted error",
			transferMode: types.TransferModeCopy,
			config:       types.LinuxInstallationConfig{},
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				kurlConfig := types.LinuxInstallationConfig{}
				defaults := types.LinuxInstallationConfig{}
				resolvedConfig := types.LinuxInstallationConfig{}

				mock.InOrder(
					m.On("ValidateTransferMode", types.TransferModeCopy).Return(nil),
					s.On("GetMigrationID").Return("", types.ErrNoActiveMigration),
					m.On("GetKurlConfig", mock.Anything).Return(kurlConfig, nil),
					m.On("GetECDefaults", mock.Anything).Return(defaults, nil),
					m.On("MergeConfigs", mock.Anything, kurlConfig, defaults).Return(resolvedConfig),
					s.On("InitializeMigration", mock.AnythingOfType("string"), "copy", resolvedConfig).Return(nil),
					s.On("SetState", types.MigrationStateNotStarted).Return(errors.New("set state error")),
				)
			},
			expectedErr: errors.New("set state error"),
			validateResult: func(t *testing.T, migrationID string, err error) {
				assert.Error(t, err)
				assert.Empty(t, migrationID)
				assert.Contains(t, err.Error(), "set initial state")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &migrationmanager.MockManager{}
			mockStore := &migrationstore.MockStore{}
			tt.setupMock(mockManager, mockStore)

			controller, err := NewMigrationController(
				WithManager(mockManager),
				WithStore(mockStore),
			)
			require.NoError(t, err)

			migrationID, err := controller.StartMigration(t.Context(), tt.transferMode, tt.config)

			if tt.expectedErr != nil {
				require.Error(t, err)
				if tt.validateResult != nil {
					tt.validateResult(t, migrationID, err)
				}
			} else {
				require.NoError(t, err)
				if tt.validateResult != nil {
					tt.validateResult(t, migrationID, err)
				}

				// Wait for the background goroutine to complete
				time.Sleep(100 * time.Millisecond)
			}

			mockManager.AssertExpectations(t)
			mockStore.AssertExpectations(t)
		})
	}
}

func TestGetMigrationStatus(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*migrationstore.MockStore)
		expectedErr   error
		expectedValue types.MigrationStatusResponse
	}{
		{
			name: "successful response with active migration",
			setupMock: func(s *migrationstore.MockStore) {
				status := types.MigrationStatusResponse{
					State:    types.MigrationStateInProgress,
					Phase:    types.MigrationPhaseDataTransfer,
					Message:  "Transferring data",
					Progress: 50,
					Error:    "",
				}
				s.On("GetStatus").Return(status, nil)
			},
			expectedErr: nil,
			expectedValue: types.MigrationStatusResponse{
				State:    types.MigrationStateInProgress,
				Phase:    types.MigrationPhaseDataTransfer,
				Message:  "Transferring data",
				Progress: 50,
				Error:    "",
			},
		},
		{
			name: "no active migration (404 error)",
			setupMock: func(s *migrationstore.MockStore) {
				s.On("GetStatus").Return(types.MigrationStatusResponse{}, types.ErrNoActiveMigration)
			},
			expectedErr:   types.ErrNoActiveMigration,
			expectedValue: types.MigrationStatusResponse{},
		},
		{
			name: "store error",
			setupMock: func(s *migrationstore.MockStore) {
				s.On("GetStatus").Return(types.MigrationStatusResponse{}, errors.New("store error"))
			},
			expectedErr:   errors.New("store error"),
			expectedValue: types.MigrationStatusResponse{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &migrationstore.MockStore{}
			tt.setupMock(mockStore)

			mockManager := &migrationmanager.MockManager{}
			controller, err := NewMigrationController(
				WithManager(mockManager),
				WithStore(mockStore),
			)
			require.NoError(t, err)

			result, err := controller.GetMigrationStatus(t.Context())

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, types.MigrationStatusResponse{}, result)

				// Verify it's a 404 error when appropriate
				if tt.expectedErr == types.ErrNoActiveMigration {
					var apiErr *types.APIError
					require.True(t, errors.As(err, &apiErr))
					assert.Equal(t, 404, apiErr.StatusCode)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}

			mockStore.AssertExpectations(t)
		})
	}
}

func TestRun(t *testing.T) {
	// These are skeleton tests since Run is a skeleton implementation
	// Full implementation will be added in PR 8
	tests := []struct {
		name        string
		setupMock   func(*migrationmanager.MockManager, *migrationstore.MockStore)
		expectedErr bool
	}{
		{
			name: "skeleton test - phase execution fails as expected",
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				mock.InOrder(
					// Initial status check
					s.On("GetStatus").Return(types.MigrationStatusResponse{
						State: types.MigrationStateNotStarted,
						Phase: types.MigrationPhaseDiscovery,
					}, nil).Once(),
					// Discovery phase - skeleton implementation fails here
					s.On("SetState", types.MigrationStateInProgress).Return(nil).Once(),
					s.On("SetPhase", types.MigrationPhaseDiscovery).Return(nil).Once(),
					// ExecutePhase is called and returns skeleton error
					m.On("ExecutePhase", mock.Anything, types.MigrationPhaseDiscovery).Return(types.ErrMigrationPhaseNotImplemented).Once(),
					// Error handling
					s.On("SetState", types.MigrationStateFailed).Return(nil).Once(),
					s.On("SetError", "migration phase execution not yet implemented").Return(nil).Once(),
				)
			},
			expectedErr: true,
		},
		{
			name: "skeleton test - error on GetStatus",
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				s.On("GetStatus").Return(types.MigrationStatusResponse{}, errors.New("get status error")).Once()
			},
			expectedErr: true,
		},
		{
			name: "skeleton test - error on SetState",
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				mock.InOrder(
					s.On("GetStatus").Return(types.MigrationStatusResponse{
						State: types.MigrationStateNotStarted,
						Phase: types.MigrationPhaseDiscovery,
					}, nil).Once(),
					s.On("SetState", types.MigrationStateInProgress).Return(errors.New("set state error")).Once(),
					s.On("SetState", types.MigrationStateFailed).Return(nil).Once(),
					s.On("SetError", "set state error").Return(nil).Once(),
				)
			},
			expectedErr: true,
		},
		{
			name: "skeleton test - error on SetPhase",
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				mock.InOrder(
					s.On("GetStatus").Return(types.MigrationStatusResponse{
						State: types.MigrationStateNotStarted,
						Phase: types.MigrationPhaseDiscovery,
					}, nil).Once(),
					s.On("SetState", types.MigrationStateInProgress).Return(nil).Once(),
					s.On("SetPhase", types.MigrationPhaseDiscovery).Return(errors.New("set phase error")).Once(),
					s.On("SetState", types.MigrationStateFailed).Return(nil).Once(),
					s.On("SetError", "set phase error").Return(nil).Once(),
				)
			},
			expectedErr: true,
		},
		{
			name: "skeleton test - resume from InProgress state",
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				mock.InOrder(
					s.On("GetStatus").Return(types.MigrationStatusResponse{
						State: types.MigrationStateInProgress,
						Phase: types.MigrationPhasePreparation,
					}, nil).Once(),
					// Should still go through all phases (skeleton doesn't implement resume logic yet)
					s.On("SetState", types.MigrationStateInProgress).Return(nil).Once(),
					s.On("SetPhase", types.MigrationPhaseDiscovery).Return(nil).Once(),
					// ExecutePhase fails in skeleton
					m.On("ExecutePhase", mock.Anything, types.MigrationPhaseDiscovery).Return(types.ErrMigrationPhaseNotImplemented).Once(),
					s.On("SetState", types.MigrationStateFailed).Return(nil).Once(),
					s.On("SetError", "migration phase execution not yet implemented").Return(nil).Once(),
				)
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &migrationmanager.MockManager{}
			mockStore := &migrationstore.MockStore{}
			tt.setupMock(mockManager, mockStore)

			controller, err := NewMigrationController(
				WithManager(mockManager),
				WithStore(mockStore),
			)
			require.NoError(t, err)

			err = controller.Run(t.Context())

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockStore.AssertExpectations(t)
		})
	}
}
