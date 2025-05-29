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
	"github.com/replicatedhq/embedded-cluster/api/client"
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
			req := httptest.NewRequest(http.MethodPost, "/install/config", bytes.NewReader(configJSON))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+"TOKEN")
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
	req := httptest.NewRequest(http.MethodPost, "/install/config", bytes.NewReader(configJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+"TOKEN")
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
	assert.Contains(t, apiError.Error(), "Service CIDR is required when globalCidr is not set")
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
	req := httptest.NewRequest(http.MethodPost, "/install/config",
		bytes.NewReader([]byte(`{"dataDirectory": "/tmp/data", "adminConsolePort": "not-a-number"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+"TOKEN")
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
	req := httptest.NewRequest(http.MethodPost, "/install/config", bytes.NewReader(configJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+"TOKEN")
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
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
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
			installation.WithNetUtils(&mockNetUtils{ifaces: []string{"eth0", "eth1"}}),
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
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
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
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
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

var _ install.Controller = &mockInstallController{}

// Mock implementation of the install.Controller interface
type mockInstallController struct {
	setConfigError  error
	getError        error
	setStatusError  error
	readStatusError error
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

func (m *mockInstallController) ReadStatus(ctx context.Context) (*types.InstallationStatus, error) {
	return nil, m.readStatusError
}

// TestInstallWithAPIClient tests the install endpoints using the API client
func TestInstallWithAPIClient(t *testing.T) {
	password := "test-password"

	// Create a config manager
	installationManager := installation.NewInstallationManager()

	// Create an install controller with the config manager
	installController, err := install.NewInstallController(
		install.WithInstallationManager(installationManager),
	)
	require.NoError(t, err)

	// Set some initial config
	initialConfig := types.InstallationConfig{
		DataDirectory:           "/tmp/test-data-for-client",
		AdminConsolePort:        9080,
		LocalArtifactMirrorPort: 9081,
		GlobalCIDR:              "192.168.0.0/16",
		NetworkInterface:        "eth1",
	}
	err = installationManager.WriteConfig(initialConfig)
	require.NoError(t, err)

	// Create the API with controllers
	apiInstance, err := api.New(
		password,
		api.WithAuthController(&staticAuthController{"TOKEN"}),
		api.WithInstallController(installController),
		api.WithLogger(api.NewDiscardLogger()),
	)
	require.NoError(t, err)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router.PathPrefix("/api").Subrouter())

	// Create a test server using the router
	server := httptest.NewServer(router)
	defer server.Close()

	// Create client with the predefined token
	c := client.New(server.URL, client.WithToken("TOKEN"))
	require.NoError(t, err, "API client login should succeed")

	// Test GetInstall
	t.Run("GetInstall", func(t *testing.T) {
		install, err := c.GetInstall()
		require.NoError(t, err, "GetInstall should succeed")
		assert.NotNil(t, install, "Install should not be nil")

		// Verify values
		assert.Equal(t, "/tmp/test-data-for-client", install.Config.DataDirectory)
		assert.Equal(t, 9080, install.Config.AdminConsolePort)
		assert.Equal(t, 9081, install.Config.LocalArtifactMirrorPort)
		assert.Equal(t, "192.168.0.0/16", install.Config.GlobalCIDR)
		assert.Equal(t, "eth1", install.Config.NetworkInterface)
	})

	// Test SetInstallConfig
	t.Run("SetInstallConfig", func(t *testing.T) {
		// Create a valid config
		config := types.InstallationConfig{
			DataDirectory:           "/tmp/new-dir",
			AdminConsolePort:        8000,
			LocalArtifactMirrorPort: 8081,
			GlobalCIDR:              "10.0.0.0/16",
			NetworkInterface:        "eth0",
		}

		// Set the config using the client
		install, err := c.SetInstallConfig(config)
		require.NoError(t, err, "SetInstallConfig should succeed with valid config")
		assert.NotNil(t, install, "Install should not be nil")

		// Verify the config was set correctly
		assert.Equal(t, config.DataDirectory, install.Config.DataDirectory)
		assert.Equal(t, config.AdminConsolePort, install.Config.AdminConsolePort)
		assert.Equal(t, config.NetworkInterface, install.Config.NetworkInterface)

		// Get the config to verify it persisted
		install, err = c.GetInstall()
		require.NoError(t, err, "GetInstall should succeed after setting config")
		assert.Equal(t, config.DataDirectory, install.Config.DataDirectory)
		assert.Equal(t, config.AdminConsolePort, install.Config.AdminConsolePort)
		assert.Equal(t, config.NetworkInterface, install.Config.NetworkInterface)
	})

	// Test SetInstallConfig validation error
	t.Run("SetInstallConfig validation error", func(t *testing.T) {
		// Create an invalid config (port conflict)
		config := types.InstallationConfig{
			DataDirectory:           "/tmp/new-dir",
			AdminConsolePort:        8080,
			LocalArtifactMirrorPort: 8080, // Same as AdminConsolePort
			GlobalCIDR:              "10.0.0.0/16",
			NetworkInterface:        "eth0",
		}

		// Set the config using the client
		_, err := c.SetInstallConfig(config)
		require.Error(t, err, "SetInstallConfig should fail with invalid config")

		// Verify the error is of type APIError
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
		// Error message should contain the field and the validation error
		assert.Contains(t, apiErr.Error(), "Admin Console Port and Local Artifact Mirror Port cannot be equal")
	})

	// Test SetInstallStatus
	t.Run("SetInstallStatus", func(t *testing.T) {
		// Create a status
		status := types.InstallationStatus{
			State:       types.InstallationStateFailed,
			Description: "Installation failed",
		}

		// Set the status using the client
		install, err := c.SetInstallStatus(status)
		require.NoError(t, err, "SetInstallStatus should succeed")
		assert.NotNil(t, install, "Install should not be nil")
		assert.NotNil(t, install.Status, status, "Install status should match the one set")
	})
}
