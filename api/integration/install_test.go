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

func TestSetInstallConfig(t *testing.T) {
	manager := installation.NewInstallationManager()

	// Create an install controller with the config manager
	installController, err := install.NewInstallController(
		install.WithInstallationManager(manager),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance, err := api.New(
		"password",
		api.WithInstallController(installController),
		api.WithAuthController(&staticAuthController{"TOKEN"}),
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
				DataDirectory:           "/tmp/data",
				AdminConsolePort:        8000,
				LocalArtifactMirrorPort: 8081,
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
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
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
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
			req.Header.Set("Authorization", "TOKEN")
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
				storedConfig, err := manager.ReadConfig()
				require.NoError(t, err)
				assert.Equal(t, tc.config.DataDirectory, storedConfig.DataDirectory)
				assert.Equal(t, tc.config.AdminConsolePort, storedConfig.AdminConsolePort)
			}
		})
	}
}

// Test that config validation errors are properly returned
func TestSetInstallConfigValidation(t *testing.T) {
	// Create a memory store
	manager := installation.NewInstallationManager()

	// Create an install controller with the config manager
	installController, err := install.NewInstallController(
		install.WithInstallationManager(manager),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance, err := api.New(
		"password",
		api.WithInstallController(installController),
		api.WithAuthController(&staticAuthController{"TOKEN"}),
		api.WithLogger(api.NewDiscardLogger()),
	)
	require.NoError(t, err)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test a validation error case with mixed CIDR settings
	config := types.InstallationConfig{
		DataDirectory:           "/tmp/data",
		AdminConsolePort:        8000,
		LocalArtifactMirrorPort: 8081,
		PodCIDR:                 "10.244.0.0/16", // Specify PodCIDR but not ServiceCIDR
		NetworkInterface:        "eth0",
	}

	// Serialize the config to JSON
	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	// Create a request
	req := httptest.NewRequest(http.MethodPost, "/install/phase/set-config", bytes.NewReader(configJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "TOKEN")
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
func TestSetInstallConfigBadRequest(t *testing.T) {
	// Create a memory store and API
	manager := installation.NewInstallationManager()

	// Create an install controller with the config manager
	installController, err := install.NewInstallController(
		install.WithInstallationManager(manager),
	)
	require.NoError(t, err)

	apiInstance, err := api.New(
		"password",
		api.WithInstallController(installController),
		api.WithAuthController(&staticAuthController{"TOKEN"}),
		api.WithLogger(api.NewDiscardLogger()),
	)
	require.NoError(t, err)

	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Create a request with invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/install/phase/set-config",
		bytes.NewReader([]byte(`{"dataDirectory": "/tmp/data", "adminConsolePort": "not-a-number"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "TOKEN")
	rec := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(rec, req)

	// Check the response
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	t.Logf("Response body: %s", rec.Body.String())
}

// Test that the server returns proper errors when the API controller fails
func TestSetInstallConfigControllerError(t *testing.T) {
	// Create a mock controller that returns an error
	mockController := &mockInstallController{
		setConfigError: assert.AnError,
	}

	// Create the API with the mock controller
	apiInstance, err := api.New(
		"password",
		api.WithInstallController(mockController),
		api.WithAuthController(&staticAuthController{"TOKEN"}),
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
	req.Header.Set("Authorization", "TOKEN")
	rec := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(rec, req)

	// Check the response
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	t.Logf("Response body: %s", rec.Body.String())
}

// Test the getInstall endpoint returns installation data correctly
func TestGetInstall(t *testing.T) {
	// Create a config manager
	installationManager := installation.NewInstallationManager()

	// Create an install controller with the config manager
	installController, err := install.NewInstallController(
		install.WithInstallationManager(installationManager),
	)
	require.NoError(t, err)

	// Set some initial config
	initialConfig := types.InstallationConfig{
		DataDirectory:           "/tmp/test-data",
		AdminConsolePort:        8080,
		LocalArtifactMirrorPort: 8081,
		GlobalCIDR:              "10.0.0.0/16",
		NetworkInterface:        "eth0",
	}
	err = installationManager.WriteConfig(initialConfig)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance, err := api.New(
		"password",
		api.WithInstallController(installController),
		api.WithAuthController(&staticAuthController{"TOKEN"}),
		api.WithLogger(api.NewDiscardLogger()),
	)
	require.NoError(t, err)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test successful get
	t.Run("Success", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/install", nil)
		req.Header.Set("Authorization", "TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var install types.Install
		err = json.NewDecoder(rec.Body).Decode(&install)
		require.NoError(t, err)

		// Verify the installation data matches what we expect
		assert.Equal(t, initialConfig.DataDirectory, install.Config.DataDirectory)
		assert.Equal(t, initialConfig.AdminConsolePort, install.Config.AdminConsolePort)
		assert.Equal(t, initialConfig.LocalArtifactMirrorPort, install.Config.LocalArtifactMirrorPort)
		assert.Equal(t, initialConfig.GlobalCIDR, install.Config.GlobalCIDR)
		assert.Equal(t, initialConfig.NetworkInterface, install.Config.NetworkInterface)
	})

	// Test get with default/empty configuration
	t.Run("Default configuration", func(t *testing.T) {
		// Create a fresh config manager without writing anything
		emptyInstallationManager := installation.NewInstallationManager(
			installation.WithNetUtils(&mockNetUtils{iface: "eth0"}),
		)

		// Create an install controller with the empty config manager
		emptyInstallController, err := install.NewInstallController(
			install.WithInstallationManager(emptyInstallationManager),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		emptyAPI, err := api.New(
			"password",
			api.WithInstallController(emptyInstallController),
			api.WithAuthController(&staticAuthController{"TOKEN"}),
			api.WithLogger(api.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		emptyRouter := mux.NewRouter()
		emptyAPI.RegisterRoutes(emptyRouter)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/install", nil)
		req.Header.Set("Authorization", "TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		emptyRouter.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var install types.Install
		err = json.NewDecoder(rec.Body).Decode(&install)
		require.NoError(t, err)

		// Verify the installation data contains defaults or empty values
		assert.Equal(t, "/var/lib/embedded-cluster", install.Config.DataDirectory)
		assert.Equal(t, 30000, install.Config.AdminConsolePort)
		assert.Equal(t, 50000, install.Config.LocalArtifactMirrorPort)
		assert.Equal(t, "10.244.0.0/16", install.Config.GlobalCIDR)
		assert.Equal(t, "eth0", install.Config.NetworkInterface)
	})

	// Test error handling
	t.Run("Controller error", func(t *testing.T) {
		// Create a mock controller that returns an error
		mockController := &mockInstallController{
			getError: assert.AnError,
		}

		// Create the API with the mock controller
		apiInstance, err := api.New(
			"password",
			api.WithInstallController(mockController),
			api.WithAuthController(&staticAuthController{"TOKEN"}),
			api.WithLogger(api.NewDiscardLogger()),
		)
		require.NoError(t, err)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/install", nil)
		req.Header.Set("Authorization", "TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusInternalServerError, rec.Code)

		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, apiError.StatusCode)
		assert.NotEmpty(t, apiError.Message)
	})
}

// Mock implementation of the install.Controller interface
type mockInstallController struct {
	setConfigError error
	getError       error
	setStatusError error
}

func (m *mockInstallController) Get(ctx context.Context) (*types.Install, error) {
	if m.getError != nil {
		return nil, m.getError
	}
	return &types.Install{
		Config: types.InstallationConfig{},
	}, nil
}

func (m *mockInstallController) SetConfig(ctx context.Context, config *types.InstallationConfig) error {
	return m.setConfigError
}

func (m *mockInstallController) SetStatus(ctx context.Context, status *types.InstallationStatus) error {
	return m.setStatusError
}
