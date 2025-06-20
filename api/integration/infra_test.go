package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/replicatedhq/embedded-cluster/api"
	"github.com/replicatedhq/embedded-cluster/api/controllers/install"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

// Test infra setup validation - focused on the new validation logic
func TestPostInstallSetupInfraValidation(t *testing.T) {
	tests := []struct {
		name           string
		request        types.InfraSetupRequest
		cliFlag        bool
		preflightState types.State
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "preflights pass - should succeed",
			request:        types.InfraSetupRequest{IgnorePreflightFailures: false},
			cliFlag:        false,
			preflightState: types.StateSucceeded,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "preflights fail with CLI flag - should succeed",
			request:        types.InfraSetupRequest{IgnorePreflightFailures: true},
			cliFlag:        true,
			preflightState: types.StateFailed,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "preflights fail without CLI flag - should fail",
			request:        types.InfraSetupRequest{IgnorePreflightFailures: false},
			cliFlag:        false,
			preflightState: types.StateFailed,
			expectedStatus: http.StatusBadRequest,
			expectedError:  install.ErrPreflightChecksFailed.Error(),
		},
		{
			name:           "preflights fail, request ignore but no CLI flag - should fail",
			request:        types.InfraSetupRequest{IgnorePreflightFailures: true},
			cliFlag:        false,
			preflightState: types.StateFailed,
			expectedStatus: http.StatusBadRequest,
			expectedError:  install.ErrPreflightChecksFailed.Error(),
		},
		{
			name:           "preflights not complete - should return 403",
			request:        types.InfraSetupRequest{IgnorePreflightFailures: false},
			cliFlag:        false,
			preflightState: types.StateRunning,
			expectedStatus: http.StatusForbidden,
			expectedError:  install.ErrPreflightChecksNotComplete.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock managers
			mockPreflightManager := &preflight.MockHostPreflightManager{}
			mockInfraManager := &infra.MockInfraManager{}
			mockInstallationManager := &installation.MockInstallationManager{}
			mockMetricsReporter := &metrics.MockReporter{}

			// Setup preflight manager mock
			mockPreflightManager.On("GetHostPreflightStatus", mock.Anything).Return(types.Status{State: tt.preflightState}, nil)

			if tt.preflightState == types.StateFailed {
				// Mock preflight output for failed cases
				preflightOutput := &types.HostPreflightsOutput{
					Fail: []types.HostPreflightsRecord{
						{Title: "Test Check", Message: "Test failed"},
					},
				}
				mockPreflightManager.On("GetHostPreflightOutput", mock.Anything).Return(preflightOutput, nil)

				// If we're ignoring failures and CLI flag allows it, expect metrics reporting and infra install
				if tt.request.IgnorePreflightFailures && tt.cliFlag {
					mockMetricsReporter.On("ReportPreflightsBypassed", mock.Anything, preflightOutput).Return(nil)
					mockInfraManager.On("Install", mock.Anything, mock.Anything).Return(nil)
					// Handler calls GetInfra after successful SetupInfra
					mockInfraManager.On("Get").Return(types.Infra{Status: types.Status{State: types.StateSucceeded}}, nil)
				}
			} else if tt.preflightState == types.StateSucceeded {
				// For successful preflights, expect infra install
				mockInfraManager.On("Install", mock.Anything, mock.Anything).Return(nil)
				// Handler calls GetInfra after successful SetupInfra
				mockInfraManager.On("Get").Return(types.Infra{Status: types.Status{State: types.StateSucceeded}}, nil)
			}

			// Create runtime config
			rc := runtimeconfig.New(nil)
			rc.SetDataDir(t.TempDir())

			// Create real controller with mocked managers
			realController, err := install.NewInstallController(
				install.WithRuntimeConfig(rc),
				install.WithHostPreflightManager(mockPreflightManager),
				install.WithInfraManager(mockInfraManager),
				install.WithInstallationManager(mockInstallationManager),
				install.WithMetricsReporter(mockMetricsReporter),
				install.WithAllowIgnoreHostPreflights(tt.cliFlag),
				install.WithLogger(logger.NewDiscardLogger()),
			)
			require.NoError(t, err)

			// Create API with real controller
			apiInstance, err := api.New(
				"password",
				api.WithInstallController(realController),
				api.WithAuthController(&staticAuthController{"TOKEN"}),
				api.WithAllowIgnoreHostPreflights(tt.cliFlag),
				api.WithLogger(logger.NewDiscardLogger()),
			)
			require.NoError(t, err)

			router := mux.NewRouter()
			apiInstance.RegisterRoutes(router)

			// Create request
			reqBody, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(http.MethodPost, "/install/infra/setup", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer TOKEN")
			rec := httptest.NewRecorder()

			// Execute request
			router.ServeHTTP(rec, req)

			// Assert response
			assert.Equal(t, tt.expectedStatus, rec.Code)

			if tt.expectedError != "" {
				var apiError types.APIError
				err := json.NewDecoder(rec.Body).Decode(&apiError)
				require.NoError(t, err)
				assert.Contains(t, apiError.Message, tt.expectedError)
			} else {
				var infraResp types.Infra
				err := json.NewDecoder(rec.Body).Decode(&infraResp)
				require.NoError(t, err)
				assert.NotNil(t, infraResp.Status)
			}

			// Verify all mocks were called as expected
			mockPreflightManager.AssertExpectations(t)
			mockInfraManager.AssertExpectations(t)
			mockInstallationManager.AssertExpectations(t)
			mockMetricsReporter.AssertExpectations(t)
		})
	}
}

// Test error handling
func TestPostInstallSetupInfraErrors(t *testing.T) {
	t.Run("Preflight status error", func(t *testing.T) {
		// Create mock managers
		mockPreflightManager := &preflight.MockHostPreflightManager{}
		mockInfraManager := &infra.MockInfraManager{}
		mockInstallationManager := &installation.MockInstallationManager{}
		mockMetricsReporter := &metrics.MockReporter{}

		// Setup mock to return error
		mockPreflightManager.On("GetHostPreflightStatus", mock.Anything).Return(types.Status{}, assert.AnError)

		// Create runtime config
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		// Create real controller with mocked managers
		realController, err := install.NewInstallController(
			install.WithRuntimeConfig(rc),
			install.WithHostPreflightManager(mockPreflightManager),
			install.WithInfraManager(mockInfraManager),
			install.WithInstallationManager(mockInstallationManager),
			install.WithMetricsReporter(mockMetricsReporter),
			install.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		apiInstance, err := api.New(
			"password",
			api.WithInstallController(realController),
			api.WithAuthController(&staticAuthController{"TOKEN"}),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		request := types.InfraSetupRequest{IgnorePreflightFailures: false}
		reqBody, _ := json.Marshal(request)
		req := httptest.NewRequest(http.MethodPost, "/install/infra/setup", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		var apiError types.APIError
		json.NewDecoder(rec.Body).Decode(&apiError)
		assert.Contains(t, apiError.Message, "get install host preflight status")

		// Verify mocks
		mockPreflightManager.AssertExpectations(t)
	})

	t.Run("Infra install error", func(t *testing.T) {
		// Create mock managers
		mockPreflightManager := &preflight.MockHostPreflightManager{}
		mockInfraManager := &infra.MockInfraManager{}
		mockInstallationManager := &installation.MockInstallationManager{}
		mockMetricsReporter := &metrics.MockReporter{}

		// Setup mocks - preflights pass but infra install fails
		mockPreflightManager.On("GetHostPreflightStatus", mock.Anything).Return(types.Status{State: types.StateSucceeded}, nil)
		mockInfraManager.On("Install", mock.Anything, mock.Anything).Return(assert.AnError)

		// Create runtime config
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		// Create real controller with mocked managers
		realController, err := install.NewInstallController(
			install.WithRuntimeConfig(rc),
			install.WithHostPreflightManager(mockPreflightManager),
			install.WithInfraManager(mockInfraManager),
			install.WithInstallationManager(mockInstallationManager),
			install.WithMetricsReporter(mockMetricsReporter),
			install.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		apiInstance, err := api.New(
			"password",
			api.WithInstallController(realController),
			api.WithAuthController(&staticAuthController{"TOKEN"}),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		request := types.InfraSetupRequest{IgnorePreflightFailures: false}
		reqBody, _ := json.Marshal(request)
		req := httptest.NewRequest(http.MethodPost, "/install/infra/setup", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)

		// Verify mocks
		mockPreflightManager.AssertExpectations(t)
		mockInfraManager.AssertExpectations(t)
	})

	t.Run("Authorization error", func(t *testing.T) {
		// Create mock managers (won't be called due to auth failure)
		mockPreflightManager := &preflight.MockHostPreflightManager{}
		mockInfraManager := &infra.MockInfraManager{}
		mockInstallationManager := &installation.MockInstallationManager{}
		mockMetricsReporter := &metrics.MockReporter{}

		// Create runtime config
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		// Create real controller with mocked managers
		realController, err := install.NewInstallController(
			install.WithRuntimeConfig(rc),
			install.WithHostPreflightManager(mockPreflightManager),
			install.WithInfraManager(mockInfraManager),
			install.WithInstallationManager(mockInstallationManager),
			install.WithMetricsReporter(mockMetricsReporter),
			install.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		apiInstance, err := api.New(
			"password",
			api.WithInstallController(realController),
			api.WithAuthController(&staticAuthController{"TOKEN"}),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		request := types.InfraSetupRequest{IgnorePreflightFailures: false}
		reqBody, _ := json.Marshal(request)
		req := httptest.NewRequest(http.MethodPost, "/install/infra/setup", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer INVALID_TOKEN")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		// No manager methods should be called due to auth failure
		mockPreflightManager.AssertExpectations(t)
		mockInfraManager.AssertExpectations(t)
		mockInstallationManager.AssertExpectations(t)
		mockMetricsReporter.AssertExpectations(t)
	})
}
