package kurlmigration

import (
	"errors"
	"testing"
	"time"

	migrationmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/kurlmigration"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	migrationstore "github.com/replicatedhq/embedded-cluster/api/internal/store/kurlmigration"
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
					PodCIDR:     "10.32.0.0/20",
					ServiceCIDR: "10.96.0.0/12",
				}

				defaults := types.LinuxInstallationConfig{
					DataDirectory: "/var/lib/embedded-cluster",
					GlobalCIDR:    "10.244.0.0/16",
				}

				resolvedConfig := types.LinuxInstallationConfig{
					DataDirectory: "/var/lib/embedded-cluster",
					PodCIDR:       "10.32.0.0/20",
					ServiceCIDR:   "10.96.0.0/12",
					GlobalCIDR:    "10.244.0.0/16",
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
						PodCIDR:     "10.32.0.0/20",
						ServiceCIDR: "10.96.0.0/12",
					},
					Defaults: types.LinuxInstallationConfig{
						DataDirectory: "/var/lib/embedded-cluster",
						GlobalCIDR:    "10.244.0.0/16",
					},
					Resolved: types.LinuxInstallationConfig{
						DataDirectory: "/var/lib/embedded-cluster",
						PodCIDR:       "10.32.0.0/20",
						ServiceCIDR:   "10.96.0.0/12",
						GlobalCIDR:    "10.244.0.0/16",
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
					DataDirectory: "/opt/kurl",
				}

				defaults := types.LinuxInstallationConfig{
					DataDirectory: "/var/lib/embedded-cluster",
				}

				// Resolved should have kURL values override defaults (since no user config yet)
				resolvedConfig := types.LinuxInstallationConfig{
					DataDirectory: "/opt/kurl",
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
						DataDirectory: "/opt/kurl",
					},
					Defaults: types.LinuxInstallationConfig{
						DataDirectory: "/var/lib/embedded-cluster",
					},
					Resolved: types.LinuxInstallationConfig{
						DataDirectory: "/opt/kurl",
					},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &migrationmanager.MockManager{}
			tt.setupMock(mockManager)

			controller, err := NewKURLMigrationController(
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
				DataDirectory: "/opt/ec",
			},
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				kurlConfig := types.LinuxInstallationConfig{
					PodCIDR:     "10.32.0.0/20",
					ServiceCIDR: "10.96.0.0/12",
				}
				defaults := types.LinuxInstallationConfig{
					DataDirectory: "/var/lib/embedded-cluster",
				}
				resolvedConfig := types.LinuxInstallationConfig{
					DataDirectory: "/opt/ec",
					PodCIDR:       "10.32.0.0/20",
					ServiceCIDR:   "10.96.0.0/12",
				}

				mock.InOrder(
					m.On("ValidateTransferMode", types.TransferModeCopy).Return(nil),
					s.On("GetMigrationID").Return("", types.ErrNoActiveKURLMigration),
					m.On("GetKurlConfig", mock.Anything).Return(kurlConfig, nil),
					m.On("GetECDefaults", mock.Anything).Return(defaults, nil),
					m.On("MergeConfigs", mock.Anything, kurlConfig, defaults).Return(resolvedConfig),
					s.On("SetUserConfig", mock.Anything).Return(nil),
					s.On("InitializeMigration", mock.AnythingOfType("string"), "copy", resolvedConfig).Return(nil),
					s.On("SetState", types.KURLMigrationStateNotStarted).Return(nil),
					// For the background goroutine - skeleton implementation fails at first phase
					s.On("GetStatus").Return(types.KURLMigrationStatusResponse{
						State: types.KURLMigrationStateNotStarted,
						Phase: types.KURLMigrationPhaseDiscovery,
					}, nil),
					s.On("SetState", types.KURLMigrationStateInProgress).Return(nil),
					s.On("SetPhase", types.KURLMigrationPhaseDiscovery).Return(nil),
					m.On("ExecutePhase", mock.Anything, types.KURLMigrationPhaseDiscovery).Return(types.ErrKURLMigrationPhaseNotImplemented),
					s.On("SetState", types.KURLMigrationStateFailed).Return(nil),
					s.On("SetError", "execute phase Discovery: kURL migration phase execution not yet implemented").Return(nil),
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
				DataDirectory: "/opt/ec",
			},
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				kurlConfig := types.LinuxInstallationConfig{}
				defaults := types.LinuxInstallationConfig{
					DataDirectory: "/var/lib/embedded-cluster",
				}
				resolvedConfig := types.LinuxInstallationConfig{
					DataDirectory: "/opt/ec",
				}

				mock.InOrder(
					m.On("ValidateTransferMode", types.TransferModeMove).Return(nil),
					s.On("GetMigrationID").Return("", types.ErrNoActiveKURLMigration),
					m.On("GetKurlConfig", mock.Anything).Return(kurlConfig, nil),
					m.On("GetECDefaults", mock.Anything).Return(defaults, nil),
					m.On("MergeConfigs", mock.Anything, kurlConfig, defaults).Return(resolvedConfig),
					s.On("SetUserConfig", mock.Anything).Return(nil),
					s.On("InitializeMigration", mock.AnythingOfType("string"), "move", resolvedConfig).Return(nil),
					s.On("SetState", types.KURLMigrationStateNotStarted).Return(nil),
					// For the background goroutine - skeleton implementation fails at first phase
					s.On("GetStatus").Return(types.KURLMigrationStatusResponse{
						State: types.KURLMigrationStateNotStarted,
						Phase: types.KURLMigrationPhaseDiscovery,
					}, nil),
					s.On("SetState", types.KURLMigrationStateInProgress).Return(nil),
					s.On("SetPhase", types.KURLMigrationPhaseDiscovery).Return(nil),
					m.On("ExecutePhase", mock.Anything, types.KURLMigrationPhaseDiscovery).Return(types.ErrKURLMigrationPhaseNotImplemented),
					s.On("SetState", types.KURLMigrationStateFailed).Return(nil),
					s.On("SetError", "execute phase Discovery: kURL migration phase execution not yet implemented").Return(nil),
				)
			},
			expectedErr: nil,
			validateResult: func(t *testing.T, migrationID string, err error) {
				assert.NoError(t, err)
				assert.NotEmpty(t, migrationID)
			},
		},
		{
			name:         "kURL migration already started (409 error)",
			transferMode: types.TransferModeCopy,
			config:       types.LinuxInstallationConfig{},
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				mock.InOrder(
					m.On("ValidateTransferMode", types.TransferModeCopy).Return(nil),
					s.On("GetMigrationID").Return("existing-migration-id", nil),
				)
			},
			expectedErr: types.ErrKURLMigrationAlreadyStarted,
			validateResult: func(t *testing.T, migrationID string, err error) {
				assert.Error(t, err)
				assert.Empty(t, migrationID)
				var apiErr *types.APIError
				require.True(t, errors.As(err, &apiErr))
				assert.Equal(t, 409, apiErr.StatusCode)
				assert.Contains(t, err.Error(), "kURL migration already started")
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
					s.On("GetMigrationID").Return("", types.ErrNoActiveKURLMigration),
					m.On("GetKurlConfig", mock.Anything).Return(kurlConfig, nil),
					m.On("GetECDefaults", mock.Anything).Return(defaults, nil),
					m.On("MergeConfigs", mock.Anything, kurlConfig, defaults).Return(resolvedConfig),
					s.On("SetUserConfig", mock.Anything).Return(nil),
					s.On("InitializeMigration", mock.AnythingOfType("string"), "copy", resolvedConfig).Return(errors.New("store error")),
				)
			},
			expectedErr: errors.New("store error"),
			validateResult: func(t *testing.T, migrationID string, err error) {
				assert.Error(t, err)
				assert.Empty(t, migrationID)
				assert.Contains(t, err.Error(), "initialize kURL migration")
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
					s.On("GetMigrationID").Return("", types.ErrNoActiveKURLMigration),
					m.On("GetKurlConfig", mock.Anything).Return(kurlConfig, nil),
					m.On("GetECDefaults", mock.Anything).Return(defaults, nil),
					m.On("MergeConfigs", mock.Anything, kurlConfig, defaults).Return(resolvedConfig),
					s.On("SetUserConfig", mock.Anything).Return(nil),
					s.On("InitializeMigration", mock.AnythingOfType("string"), "copy", resolvedConfig).Return(nil),
					s.On("SetState", types.KURLMigrationStateNotStarted).Return(errors.New("set state error")),
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
			// Create unified store with empty migration store
			mockStore := &store.MockStore{}
			// Set up expectations on the unified store's migration store field
			tt.setupMock(mockManager, &mockStore.KURLMigrationMockStore)

			controller, err := NewKURLMigrationController(
				WithManager(mockManager),
				WithStore(mockStore),
			)
			require.NoError(t, err)

			migrationID, err := controller.StartKURLMigration(t.Context(), tt.transferMode, tt.config)

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
				// Includes 100ms delay in Run() to prevent race condition + time for execution
				time.Sleep(250 * time.Millisecond)
			}

			mockManager.AssertExpectations(t)
			mockStore.KURLMigrationMockStore.AssertExpectations(t)
		})
	}
}

func TestGetMigrationStatus(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*migrationstore.MockStore)
		expectedErr   error
		expectedValue types.KURLMigrationStatusResponse
	}{
		{
			name: "successful response with active migration",
			setupMock: func(s *migrationstore.MockStore) {
				status := types.KURLMigrationStatusResponse{
					State:    types.KURLMigrationStateInProgress,
					Phase:    types.KURLMigrationPhaseDataTransfer,
					Message:  "Transferring data",
					Progress: 50,
					Error:    "",
				}
				s.On("GetStatus").Return(status, nil)
			},
			expectedErr: nil,
			expectedValue: types.KURLMigrationStatusResponse{
				State:    types.KURLMigrationStateInProgress,
				Phase:    types.KURLMigrationPhaseDataTransfer,
				Message:  "Transferring data",
				Progress: 50,
				Error:    "",
			},
		},
		{
			name: "no active migration (404 error)",
			setupMock: func(s *migrationstore.MockStore) {
				s.On("GetStatus").Return(types.KURLMigrationStatusResponse{}, types.ErrNoActiveKURLMigration)
			},
			expectedErr:   types.ErrNoActiveKURLMigration,
			expectedValue: types.KURLMigrationStatusResponse{},
		},
		{
			name: "store error",
			setupMock: func(s *migrationstore.MockStore) {
				s.On("GetStatus").Return(types.KURLMigrationStatusResponse{}, errors.New("store error"))
			},
			expectedErr:   errors.New("store error"),
			expectedValue: types.KURLMigrationStatusResponse{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create unified store with empty migration store
			mockStore := &store.MockStore{}
			// Set up expectations on the unified store's migration store field
			tt.setupMock(&mockStore.KURLMigrationMockStore)

			mockManager := &migrationmanager.MockManager{}
			controller, err := NewKURLMigrationController(
				WithManager(mockManager),
				WithStore(mockStore),
			)
			require.NoError(t, err)

			result, err := controller.GetKURLMigrationStatus(t.Context())

			if tt.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, types.KURLMigrationStatusResponse{}, result)

				// Verify it's a 404 error when appropriate
				if tt.expectedErr == types.ErrNoActiveKURLMigration {
					var apiErr *types.APIError
					require.True(t, errors.As(err, &apiErr))
					assert.Equal(t, 404, apiErr.StatusCode)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, result)
			}

			mockStore.KURLMigrationMockStore.AssertExpectations(t)
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
					s.On("GetStatus").Return(types.KURLMigrationStatusResponse{
						State: types.KURLMigrationStateNotStarted,
						Phase: types.KURLMigrationPhaseDiscovery,
					}, nil).Once(),
					// Discovery phase - skeleton implementation fails here
					s.On("SetState", types.KURLMigrationStateInProgress).Return(nil).Once(),
					s.On("SetPhase", types.KURLMigrationPhaseDiscovery).Return(nil).Once(),
					// ExecutePhase is called and returns skeleton error
					m.On("ExecutePhase", mock.Anything, types.KURLMigrationPhaseDiscovery).Return(types.ErrKURLMigrationPhaseNotImplemented).Once(),
					// Error handling
					s.On("SetState", types.KURLMigrationStateFailed).Return(nil).Once(),
					s.On("SetError", "execute phase Discovery: kURL migration phase execution not yet implemented").Return(nil).Once(),
				)
			},
			expectedErr: true,
		},
		{
			name: "skeleton test - error on GetStatus",
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				s.On("GetStatus").Return(types.KURLMigrationStatusResponse{}, errors.New("get status error")).Once()
				// Defer will try to set state to Failed and set error message
				s.On("SetState", types.KURLMigrationStateFailed).Return(nil).Once()
				s.On("SetError", "get status: get status error").Return(nil).Once()
			},
			expectedErr: true,
		},
		{
			name: "skeleton test - error on SetState",
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				mock.InOrder(
					s.On("GetStatus").Return(types.KURLMigrationStatusResponse{
						State: types.KURLMigrationStateNotStarted,
						Phase: types.KURLMigrationPhaseDiscovery,
					}, nil).Once(),
					s.On("SetState", types.KURLMigrationStateInProgress).Return(errors.New("set state error")).Once(),
					s.On("SetState", types.KURLMigrationStateFailed).Return(nil).Once(),
					s.On("SetError", "set state: set state error").Return(nil).Once(),
				)
			},
			expectedErr: true,
		},
		{
			name: "skeleton test - error on SetPhase",
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				mock.InOrder(
					s.On("GetStatus").Return(types.KURLMigrationStatusResponse{
						State: types.KURLMigrationStateNotStarted,
						Phase: types.KURLMigrationPhaseDiscovery,
					}, nil).Once(),
					s.On("SetState", types.KURLMigrationStateInProgress).Return(nil).Once(),
					s.On("SetPhase", types.KURLMigrationPhaseDiscovery).Return(errors.New("set phase error")).Once(),
					s.On("SetState", types.KURLMigrationStateFailed).Return(nil).Once(),
					s.On("SetError", "set phase: set phase error").Return(nil).Once(),
				)
			},
			expectedErr: true,
		},
		{
			name: "skeleton test - resume from InProgress state",
			setupMock: func(m *migrationmanager.MockManager, s *migrationstore.MockStore) {
				mock.InOrder(
					s.On("GetStatus").Return(types.KURLMigrationStatusResponse{
						State: types.KURLMigrationStateInProgress,
						Phase: types.KURLMigrationPhasePreparation,
					}, nil).Once(),
					// Should still go through all phases (skeleton doesn't implement resume logic yet)
					s.On("SetState", types.KURLMigrationStateInProgress).Return(nil).Once(),
					s.On("SetPhase", types.KURLMigrationPhaseDiscovery).Return(nil).Once(),
					// ExecutePhase fails in skeleton
					m.On("ExecutePhase", mock.Anything, types.KURLMigrationPhaseDiscovery).Return(types.ErrKURLMigrationPhaseNotImplemented).Once(),
					s.On("SetState", types.KURLMigrationStateFailed).Return(nil).Once(),
					s.On("SetError", "execute phase Discovery: kURL migration phase execution not yet implemented").Return(nil).Once(),
				)
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &migrationmanager.MockManager{}
			// Create unified store with empty migration store
			mockStore := &store.MockStore{}
			// Set up expectations on the unified store's migration store field
			tt.setupMock(mockManager, &mockStore.KURLMigrationMockStore)

			controller, err := NewKURLMigrationController(
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

			mockStore.KURLMigrationMockStore.AssertExpectations(t)
		})
	}
}
