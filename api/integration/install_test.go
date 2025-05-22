package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	"github.com/replicatedhq/embedded-cluster/api/controllers/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/installation"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallPhaseSetConfig(t *testing.T) {
	// Create a memory store
	configStore := installation.NewConfigMemoryStore()

	// Create an install controller with the memory store
	installController, err := install.NewInstallController(
		install.WithConfigStore(configStore),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance, err := api.New(
		"password",
		api.WithInstallController(installController),
		api.WithLogger(api.NewDiscardLogger()),
	)
	require.NoError(t, err)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test scenarios
	testCases := []struct {
		name           string
		config         types.InstallationConfig
		expectedStatus int
		expectedError  bool
	}{
		{
			name: "Valid config",
			config: types.InstallationConfig{
				DataDirectory:    "/tmp/data",
				AdminConsolePort: 8000,
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
		},
		{
			name: "Invalid config - port conflict",
			config: types.InstallationConfig{
				DataDirectory:           "/tmp/data",
				AdminConsolePort:        8080,
				LocalArtifactMirrorPort: 8080, // Same as AdminConsolePort
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Serialize the config to JSON
			configJSON, err := json.Marshal(tc.config)
			require.NoError(t, err)

			// Create a request
			req := httptest.NewRequest(http.MethodPost, "/install/phase/set-config", bytes.NewReader(configJSON))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			// Serve the request
			router.ServeHTTP(rec, req)

			// Check the response
			assert.Equal(t, tc.expectedStatus, rec.Code)

			t.Logf("Response body: %s", rec.Body.String())

			// Parse the response body
			if tc.expectedError {
				var apiError types.APIError
				err = json.NewDecoder(rec.Body).Decode(&apiError)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedStatus, apiError.StatusCode)
				assert.NotEmpty(t, apiError.Message)
			} else {
				var install types.Install
				err = json.NewDecoder(rec.Body).Decode(&install)
				require.NoError(t, err)

				// Verify that the config was properly set
				assert.Equal(t, tc.config.DataDirectory, install.Config.DataDirectory)
				assert.Equal(t, tc.config.AdminConsolePort, install.Config.AdminConsolePort)
			}

			// Also verify that the config is in the store
			if !tc.expectedError {
				storedConfig, err := configStore.Read()
				require.NoError(t, err)
				assert.Equal(t, tc.config.DataDirectory, storedConfig.DataDirectory)
				assert.Equal(t, tc.config.AdminConsolePort, storedConfig.AdminConsolePort)
			}
		})
	}
}

// Test that config validation errors are properly returned
func TestInstallPhaseSetConfigValidation(t *testing.T) {
	// Create a memory store
	configStore := installation.NewConfigMemoryStore()

	// Create an install controller with the memory store
	installController, err := install.NewInstallController(
		install.WithConfigStore(configStore),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance, err := api.New(
		"password",
		api.WithInstallController(installController),
		api.WithLogger(api.NewDiscardLogger()),
	)
	require.NoError(t, err)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test a validation error case with mixed CIDR settings
	config := types.InstallationConfig{
		DataDirectory: "/tmp/data",
		PodCIDR:       "10.244.0.0/16", // Specify PodCIDR but not ServiceCIDR
	}

	// Serialize the config to JSON
	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	// Create a request
	req := httptest.NewRequest(http.MethodPost, "/install/phase/set-config", bytes.NewReader(configJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(rec, req)

	// Check the response
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	t.Logf("Response body: %s", rec.Body.String())

	// We expect a ValidationError with specific error about ServiceCIDR
	var apiError types.APIError
	err = json.NewDecoder(rec.Body).Decode(&apiError)
	require.NoError(t, err)
	assert.Contains(t, apiError.Error(), "serviceCidr")
}

// Test that the endpoint properly handles malformed JSON
func TestInstallPhaseSetConfigBadRequest(t *testing.T) {
	// Create a memory store and API
	configStore := installation.NewConfigMemoryStore()

	installController, err := install.NewInstallController(
		install.WithConfigStore(configStore),
	)
	require.NoError(t, err)

	apiInstance, err := api.New(
		"password",
		api.WithInstallController(installController),
		api.WithLogger(api.NewDiscardLogger()),
	)
	require.NoError(t, err)

	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Create a request with invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/install/phase/set-config",
		bytes.NewReader([]byte(`{"dataDirectory": "/tmp/data", "adminConsolePort": "not-a-number"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(rec, req)

	// Check the response
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	t.Logf("Response body: %s", rec.Body.String())
}

// Test that the server returns proper errors when the API controller fails
func TestInstallPhaseSetConfigControllerError(t *testing.T) {
	// Create a mock controller that returns an error
	mockController := &mockInstallController{
		setConfigError: assert.AnError,
	}

	// Create the API with the mock controller
	apiInstance, err := api.New(
		"password",
		api.WithInstallController(mockController),
		api.WithLogger(api.NewDiscardLogger()),
	)
	require.NoError(t, err)

	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Create a valid config request
	config := types.InstallationConfig{
		DataDirectory:    "/tmp/data",
		AdminConsolePort: 8000,
	}
	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	// Create a request
	req := httptest.NewRequest(http.MethodPost, "/install/phase/set-config", bytes.NewReader(configJSON))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(rec, req)

	// Check the response
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	t.Logf("Response body: %s", rec.Body.String())
}

// Mock implementation of the install.Controller interface
type mockInstallController struct {
	setConfigError error
}

func (m *mockInstallController) Get(ctx context.Context) (*types.Install, error) {
	return &types.Install{
		Config: types.InstallationConfig{},
	}, nil
}

func (m *mockInstallController) SetConfig(ctx context.Context, config *types.InstallationConfig) error {
	return m.setConfigError
}

func (m *mockInstallController) StartInstall(ctx context.Context) error {
	return nil
}
