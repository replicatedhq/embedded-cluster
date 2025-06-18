package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	"github.com/replicatedhq/embedded-cluster/api/controllers/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Simple mock controller for infra setup validation tests
type mockInfraController struct {
	preflightStatus           *types.Status
	preflightError            error
	setupError                error
	allowIgnoreHostPreflights bool
}

func (m *mockInfraController) GetInstallationConfig(ctx context.Context) (*types.InstallationConfig, error) {
	return &types.InstallationConfig{}, nil
}

func (m *mockInfraController) ConfigureInstallation(ctx context.Context, config *types.InstallationConfig) error {
	return nil
}

func (m *mockInfraController) GetInstallationStatus(ctx context.Context) (*types.Status, error) {
	return &types.Status{}, nil
}

func (m *mockInfraController) RunHostPreflights(ctx context.Context, opts install.RunHostPreflightsOptions) error {
	return nil
}

func (m *mockInfraController) GetHostPreflightStatus(ctx context.Context) (*types.Status, error) {
	if m.preflightError != nil {
		return nil, m.preflightError
	}
	return m.preflightStatus, nil
}

func (m *mockInfraController) GetHostPreflightOutput(ctx context.Context) (*types.HostPreflightsOutput, error) {
	// Return appropriate output based on preflight status
	if m.preflightStatus != nil && m.preflightStatus.State == types.StateFailed {
		return &types.HostPreflightsOutput{
			Fail: []types.HostPreflightsRecord{
				{Title: "Mock Failure", Message: "Mock preflight failure for testing"},
			},
		}, nil
	}
	return &types.HostPreflightsOutput{
		Pass: []types.HostPreflightsRecord{
			{Title: "Mock Success", Message: "Mock preflight success for testing"},
		},
	}, nil
}

func (m *mockInfraController) GetHostPreflightTitles(ctx context.Context) ([]string, error) {
	return []string{}, nil
}

func (m *mockInfraController) SetupInfra(ctx context.Context, ignorePreflightFailures bool) (bool, error) {
	if m.setupError != nil {
		return false, m.setupError
	}

	// Check for preflight error first (this simulates GetHostPreflightStatus failing)
	if m.preflightError != nil {
		return false, fmt.Errorf("get install host preflight status: %w", m.preflightError)
	}

	// Simulate the validation logic that was moved to SetupInfra
	if m.preflightStatus != nil && m.preflightStatus.State == types.StateFailed {
		// Check if we can proceed despite failures
		if !ignorePreflightFailures || !m.allowIgnoreHostPreflights {
			return false, fmt.Errorf("Preflight checks failed")
		}

		// We're proceeding despite failures
		return true, nil
	}

	// Preflights passed
	return false, nil
}

func (m *mockInfraController) GetInfra(ctx context.Context) (*types.Infra, error) {
	return &types.Infra{Status: &types.Status{State: types.StateRunning}}, nil
}

func (m *mockInfraController) SetStatus(ctx context.Context, status *types.Status) error {
	return nil
}

func (m *mockInfraController) GetStatus(ctx context.Context) (*types.Status, error) {
	return &types.Status{}, nil
}

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
			mockController := &mockInfraController{
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
				var infraResp types.InfraSetupResponse
				err := json.NewDecoder(rec.Body).Decode(&infraResp)
				require.NoError(t, err)
				assert.NotNil(t, infraResp.Status)
				assert.Equal(t, tt.request.IgnorePreflightFailures, infraResp.PreflightsIgnored)
			}
		})
	}
}

// Test error handling
func TestPostInstallSetupInfraErrors(t *testing.T) {
	t.Run("Preflight status error", func(t *testing.T) {
		mockController := &mockInfraController{
			preflightError: assert.AnError,
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
		mockController := &mockInfraController{
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
