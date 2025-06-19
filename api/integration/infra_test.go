package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			expectedError:  "Preflight checks failed",
		},
		{
			name:           "preflights fail, request ignore but no CLI flag - should fail",
			request:        types.InfraSetupRequest{IgnorePreflightFailures: true},
			cliFlag:        false,
			preflightState: types.StateFailed,
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Preflight checks failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock controller
			mockController := &mockInstallController{
				preflightStatus:           &types.Status{State: tt.preflightState},
				allowIgnoreHostPreflights: tt.cliFlag,
			}

			// Create API with CLI flag
			apiInstance, err := api.New(
				"password",
				api.WithInstallController(mockController),
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
		})
	}
}

// Test error handling
func TestPostInstallSetupInfraErrors(t *testing.T) {
	t.Run("Preflight status error", func(t *testing.T) {
		mockController := &mockInstallController{
			getHostPreflightStatusError: assert.AnError,
		}

		apiInstance, err := api.New(
			"password",
			api.WithInstallController(mockController),
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
	})

	t.Run("Authorization error", func(t *testing.T) {
		mockController := &mockInstallController{
			preflightStatus: &types.Status{State: types.StateSucceeded},
		}

		apiInstance, err := api.New(
			"password",
			api.WithInstallController(mockController),
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
	})
}
