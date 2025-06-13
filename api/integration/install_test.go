package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	"github.com/replicatedhq/embedded-cluster/api/client"
	"github.com/replicatedhq/embedded-cluster/api/controllers/install"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/installation"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock implementation of the install.Controller interface
type mockInstallController struct {
	configureInstallationError  error
	getInstallationConfigError  error
	runHostPreflightsError      error
	getHostPreflightStatusError error
	getHostPreflightOutputError error
	getHostPreflightTitlesError error
	setupInfraError             error
	getInfraError               error
	setStatusError              error
	readStatusError             error
}

func (m *mockInstallController) GetInstallationConfig(ctx context.Context) (*types.InstallationConfig, error) {
	if m.getInstallationConfigError != nil {
		return nil, m.getInstallationConfigError
	}
	return &types.InstallationConfig{}, nil
}

func (m *mockInstallController) ConfigureInstallation(ctx context.Context, config *types.InstallationConfig) error {
	return m.configureInstallationError
}

func (m *mockInstallController) GetInstallationStatus(ctx context.Context) (*types.Status, error) {
	if m.readStatusError != nil {
		return nil, m.readStatusError
	}
	return &types.Status{}, nil
}

func (m *mockInstallController) RunHostPreflights(ctx context.Context) error {
	return m.runHostPreflightsError
}

func (m *mockInstallController) GetHostPreflightStatus(ctx context.Context) (*types.Status, error) {
	if m.getHostPreflightStatusError != nil {
		return nil, m.getHostPreflightStatusError
	}
	return &types.Status{}, nil
}

func (m *mockInstallController) GetHostPreflightOutput(ctx context.Context) (*types.HostPreflightsOutput, error) {
	if m.getHostPreflightOutputError != nil {
		return nil, m.getHostPreflightOutputError
	}
	return &types.HostPreflightsOutput{}, nil
}

func (m *mockInstallController) GetHostPreflightTitles(ctx context.Context) ([]string, error) {
	if m.getHostPreflightTitlesError != nil {
		return nil, m.getHostPreflightTitlesError
	}
	return []string{}, nil
}

func (m *mockInstallController) SetupInfra(ctx context.Context) error {
	return m.setupInfraError
}

func (m *mockInstallController) GetInfra(ctx context.Context) (*types.Infra, error) {
	if m.getInfraError != nil {
		return nil, m.getInfraError
	}
	return &types.Infra{}, nil
}

func (m *mockInstallController) SetStatus(ctx context.Context, status *types.Status) error {
	return m.setStatusError
}

func (m *mockInstallController) GetStatus(ctx context.Context) (*types.Status, error) {
	return nil, m.readStatusError
}

func TestConfigureInstallation(t *testing.T) {
	// Test scenarios
	testCases := []struct {
		name           string
		mockHostUtils  *hostutils.MockHostUtils
		token          string
		config         types.InstallationConfig
		expectedStatus int
		expectedError  bool
	}{
		{
			name: "Valid config",
			mockHostUtils: func() *hostutils.MockHostUtils {
				mockHostUtils := &hostutils.MockHostUtils{}
				mockHostUtils.On("ConfigureHost", mock.Anything, mock.Anything).Return(nil).Once()
				return mockHostUtils
			}(),
			token: "TOKEN",
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
			name:          "Invalid config - port conflict",
			mockHostUtils: &hostutils.MockHostUtils{},
			token:         "TOKEN",
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
		{
			name:           "Unauthorized",
			mockHostUtils:  &hostutils.MockHostUtils{},
			token:          "NOT_A_TOKEN",
			config:         types.InstallationConfig{},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a runtime config
			rc := runtimeconfig.New(nil)
			rc.SetDataDir(t.TempDir())

			// Create an install controller with the config manager
			installController, err := install.NewInstallController(
				install.WithHostUtils(tc.mockHostUtils),
				install.WithRuntimeConfig(rc),
			)
			require.NoError(t, err)

			// Create the API with the install controller
			apiInstance, err := api.New(
				"password",
				api.WithInstallController(installController),
				api.WithAuthController(&staticAuthController{"TOKEN"}),
				api.WithLogger(logger.NewDiscardLogger()),
			)
			require.NoError(t, err)

			// Create a router and register the API routes
			router := mux.NewRouter()
			apiInstance.RegisterRoutes(router)

			// Serialize the config to JSON
			configJSON, err := json.Marshal(tc.config)
			require.NoError(t, err)

			// Create a request
			req := httptest.NewRequest(http.MethodPost, "/install/installation/configure", bytes.NewReader(configJSON))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tc.token)
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
				var status types.Status
				err = json.NewDecoder(rec.Body).Decode(&status)
				require.NoError(t, err)

				// Verify that the status was properly set
				// The status is set to succeeded in a goroutine, so we need to wait for it
				assert.Eventually(t, func() bool {
					status, err := installController.GetInstallationStatus(t.Context())
					require.NoError(t, err)
					return status.State == types.StateSucceeded && status.Description == "Installation configured"
				}, 1*time.Second, 100*time.Millisecond, "status should eventually be succeeded")
			}

			if !tc.expectedError {
				// Verify that the config is in the store
				storedConfig, err := installController.GetInstallationConfig(t.Context())
				require.NoError(t, err)
				assert.Equal(t, tc.config.DataDirectory, storedConfig.DataDirectory)
				assert.Equal(t, tc.config.AdminConsolePort, storedConfig.AdminConsolePort)

				// Verify that the runtime config is updated
				assert.Equal(t, tc.config.DataDirectory, rc.EmbeddedClusterHomeDirectory())
				assert.Equal(t, tc.config.AdminConsolePort, rc.AdminConsolePort())
				assert.Equal(t, tc.config.LocalArtifactMirrorPort, rc.LocalArtifactMirrorPort())
			}

			// Verify host confiuration was performed for successful tests
			tc.mockHostUtils.AssertExpectations(t)
		})
	}
}

// Test that config validation errors are properly returned
func TestConfigureInstallationValidation(t *testing.T) {
	// Create an install controller with the config manager
	installController, err := install.NewInstallController()
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance, err := api.New(
		"password",
		api.WithInstallController(installController),
		api.WithAuthController(&staticAuthController{"TOKEN"}),
		api.WithLogger(logger.NewDiscardLogger()),
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
	req := httptest.NewRequest(http.MethodPost, "/install/installation/configure", bytes.NewReader(configJSON))
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
	// Also verify the field name is correct
	assert.Equal(t, "serviceCidr", apiError.Errors[0].Field)
}

// Test that the endpoint properly handles malformed JSON
func TestConfigureInstallationBadRequest(t *testing.T) {
	// Create an install controller with the config manager
	installController, err := install.NewInstallController()
	require.NoError(t, err)

	apiInstance, err := api.New(
		"password",
		api.WithInstallController(installController),
		api.WithAuthController(&staticAuthController{"TOKEN"}),
		api.WithLogger(logger.NewDiscardLogger()),
	)
	require.NoError(t, err)

	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Create a request with invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/install/installation/configure",
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
func TestConfigureInstallationControllerError(t *testing.T) {
	// Create a mock controller that returns an error
	mockController := &mockInstallController{
		configureInstallationError: assert.AnError,
	}

	// Create the API with the mock controller
	apiInstance, err := api.New(
		"password",
		api.WithInstallController(mockController),
		api.WithAuthController(&staticAuthController{"TOKEN"}),
		api.WithLogger(logger.NewDiscardLogger()),
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
	req := httptest.NewRequest(http.MethodPost, "/install/installation/configure", bytes.NewReader(configJSON))
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
func TestGetInstallationConfig(t *testing.T) {
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
	err = installationManager.SetConfig(initialConfig)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance, err := api.New(
		"password",
		api.WithInstallController(installController),
		api.WithAuthController(&staticAuthController{"TOKEN"}),
		api.WithLogger(logger.NewDiscardLogger()),
	)
	require.NoError(t, err)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test successful get
	t.Run("Success", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/install/installation/config", nil)
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var config types.InstallationConfig
		err = json.NewDecoder(rec.Body).Decode(&config)
		require.NoError(t, err)

		// Verify the installation data matches what we expect
		assert.Equal(t, initialConfig.DataDirectory, config.DataDirectory)
		assert.Equal(t, initialConfig.AdminConsolePort, config.AdminConsolePort)
		assert.Equal(t, initialConfig.LocalArtifactMirrorPort, config.LocalArtifactMirrorPort)
		assert.Equal(t, initialConfig.GlobalCIDR, config.GlobalCIDR)
		assert.Equal(t, initialConfig.NetworkInterface, config.NetworkInterface)
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
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		emptyRouter := mux.NewRouter()
		emptyAPI.RegisterRoutes(emptyRouter)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/install/installation/config", nil)
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		emptyRouter.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var config types.InstallationConfig
		err = json.NewDecoder(rec.Body).Decode(&config)
		require.NoError(t, err)

		// Verify the installation data contains defaults or empty values
		assert.Equal(t, "/var/lib/embedded-cluster", config.DataDirectory)
		assert.Equal(t, 30000, config.AdminConsolePort)
		assert.Equal(t, 50000, config.LocalArtifactMirrorPort)
		assert.Equal(t, "10.244.0.0/16", config.GlobalCIDR)
		assert.Equal(t, "eth0", config.NetworkInterface)
	})

	// Test authorization
	t.Run("Authorization error", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/install/installation/config", nil)
		req.Header.Set("Authorization", "Bearer "+"NOT_A_TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, apiError.StatusCode)
	})

	// Test error handling
	t.Run("Controller error", func(t *testing.T) {
		// Create a mock controller that returns an error
		mockController := &mockInstallController{
			getInstallationConfigError: assert.AnError,
		}

		// Create the API with the mock controller
		apiInstance, err := api.New(
			"password",
			api.WithInstallController(mockController),
			api.WithAuthController(&staticAuthController{"TOKEN"}),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/install/installation/config", nil)
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

// Test the getInstallStatus endpoint returns install status correctly
func TestGetInstallStatus(t *testing.T) {
	// Create an install controller with the config manager
	installController, err := install.NewInstallController()
	require.NoError(t, err)

	// Set some initial status
	initialStatus := types.Status{
		State:       types.StatePending,
		Description: "Installation in progress",
	}
	err = installController.SetStatus(t.Context(), &initialStatus)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance, err := api.New(
		"password",
		api.WithInstallController(installController),
		api.WithAuthController(&staticAuthController{"TOKEN"}),
		api.WithLogger(logger.NewDiscardLogger()),
	)
	require.NoError(t, err)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test successful get
	t.Run("Success", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/install/status", nil)
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var status types.Status
		err = json.NewDecoder(rec.Body).Decode(&status)
		require.NoError(t, err)

		// Verify the status matches what we expect
		assert.Equal(t, initialStatus.State, status.State)
		assert.Equal(t, initialStatus.Description, status.Description)
	})

	// Test authorization
	t.Run("Authorization error", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/install/status", nil)
		req.Header.Set("Authorization", "Bearer "+"NOT_A_TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, apiError.StatusCode)
	})

	// Test error handling
	t.Run("Controller error", func(t *testing.T) {
		// Create a mock controller that returns an error
		mockController := &mockInstallController{
			readStatusError: assert.AnError,
		}

		// Create the API with the mock controller
		apiInstance, err := api.New(
			"password",
			api.WithInstallController(mockController),
			api.WithAuthController(&staticAuthController{"TOKEN"}),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/install/status", nil)
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

// Test the setInstallStatus endpoint sets install status correctly
func TestSetInstallStatus(t *testing.T) {
	// Create an install controller with the config manager
	installController, err := install.NewInstallController()
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance, err := api.New(
		"password",
		api.WithInstallController(installController),
		api.WithAuthController(&staticAuthController{"TOKEN"}),
		api.WithLogger(logger.NewDiscardLogger()),
	)
	require.NoError(t, err)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	t.Run("Valid status is passed", func(t *testing.T) {

		now := time.Now()
		status := types.Status{
			State:       types.StatePending,
			Description: "Install is pending",
			LastUpdated: now,
		}

		// Serialize the status to JSON
		statusJSON, err := json.Marshal(status)
		require.NoError(t, err)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/install/status", bytes.NewReader(statusJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)

		t.Logf("Response body: %s", rec.Body.String())

		// Parse the response body
		var respStatus types.Status
		err = json.NewDecoder(rec.Body).Decode(&respStatus)
		require.NoError(t, err)

		// Verify that the status was properly set
		assert.Equal(t, status.State, respStatus.State)
		assert.Equal(t, status.Description, respStatus.Description)
		assert.Equal(t, now.Format(time.RFC3339), respStatus.LastUpdated.Format(time.RFC3339))

		// Also verify that the status is in the store
		storedStatus, err := installController.GetStatus(t.Context())
		require.NoError(t, err)
		assert.Equal(t, status.State, storedStatus.State)
		assert.Equal(t, status.Description, storedStatus.Description)
		assert.Equal(t, now.Format(time.RFC3339), storedStatus.LastUpdated.Format(time.RFC3339))
	})

	// Test that the endpoint properly handles validation errors
	t.Run("Validation error", func(t *testing.T) {
		// Create a request with invalid JSON
		req := httptest.NewRequest(http.MethodPost, "/install/status",
			bytes.NewReader([]byte(`{"state": "INVALID_STATE"}`)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusBadRequest, rec.Code)

		t.Logf("Response body: %s", rec.Body.String())
	})

	// Test authorization errors
	t.Run("Authorization error", func(t *testing.T) {
		// Create a request with invalid JSON
		req := httptest.NewRequest(http.MethodPost, "/install/status",
			bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+"NOT_A_TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, apiError.StatusCode)
	})

	// Test controller error
	t.Run("Controller error", func(t *testing.T) {
		// Create a mock controller that returns an error
		mockController := &mockInstallController{
			setStatusError: assert.AnError,
		}

		// Create the API with the mock controller
		apiInstance, err := api.New(
			"password",
			api.WithInstallController(mockController),
			api.WithAuthController(&staticAuthController{"TOKEN"}),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a valid status
		status := types.Status{
			State:       types.StatePending,
			Description: "Installation in progress",
		}
		statusJSON, err := json.Marshal(status)
		require.NoError(t, err)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/install/status", bytes.NewReader(statusJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusInternalServerError, rec.Code)

		t.Logf("Response body: %s", rec.Body.String())
	})
}

// TestInstallWithAPIClient tests the install endpoints using the API client
func TestInstallWithAPIClient(t *testing.T) {
	password := "test-password"

	// Create a runtimeconfig to be used in the install process
	rc := runtimeconfig.New(nil)

	// Create a mock hostutils
	mockHostUtils := &hostutils.MockHostUtils{}
	mockHostUtils.On("ConfigureHost", mock.Anything, mock.Anything).Return(nil)

	// Create a config manager
	installationManager := installation.NewInstallationManager(
		installation.WithRuntimeConfig(rc),
		installation.WithHostUtils(mockHostUtils),
	)

	// Create an install controller with the config manager
	installController, err := install.NewInstallController(
		install.WithRuntimeConfig(rc),
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
	err = installationManager.SetConfig(initialConfig)
	require.NoError(t, err)

	// Set some initial status
	initialStatus := types.Status{
		State:       types.StatePending,
		Description: "Installation pending",
	}
	err = installationManager.SetStatus(initialStatus)
	require.NoError(t, err)

	// Create the API with controllers
	apiInstance, err := api.New(
		password,
		api.WithAuthController(&staticAuthController{"TOKEN"}),
		api.WithInstallController(installController),
		api.WithLogger(logger.NewDiscardLogger()),
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

	// Test GetInstallationConfig
	t.Run("GetInstallationConfig", func(t *testing.T) {
		config, err := c.GetInstallationConfig()
		require.NoError(t, err, "GetInstallationConfig should succeed")
		assert.NotNil(t, config, "InstallationConfig should not be nil")

		// Verify values
		assert.Equal(t, "/tmp/test-data-for-client", config.DataDirectory)
		assert.Equal(t, 9080, config.AdminConsolePort)
		assert.Equal(t, 9081, config.LocalArtifactMirrorPort)
		assert.Equal(t, "192.168.0.0/16", config.GlobalCIDR)
		assert.Equal(t, "eth1", config.NetworkInterface)
	})

	// Test GetInstallationStatus
	t.Run("GetInstallationStatus", func(t *testing.T) {
		status, err := c.GetInstallationStatus()
		require.NoError(t, err, "GetInstallationStatus should succeed")
		assert.NotNil(t, status, "InstallationStatus should not be nil")
		assert.Equal(t, types.StatePending, status.State)
		assert.Equal(t, "Installation pending", status.Description)
	})

	// Test ConfigureInstallation
	t.Run("ConfigureInstallation", func(t *testing.T) {
		// Create a valid config
		config := types.InstallationConfig{
			DataDirectory:           "/tmp/new-dir",
			AdminConsolePort:        8000,
			LocalArtifactMirrorPort: 8081,
			GlobalCIDR:              "10.0.0.0/16",
			NetworkInterface:        "eth0",
		}

		// Configure the installation using the client
		status, err := c.ConfigureInstallation(&config)
		require.NoError(t, err, "ConfigureInstallation should succeed with valid config")
		assert.NotNil(t, status, "Status should not be nil")

		// Verify the status was set correctly
		assert.Equal(t, types.StateRunning, status.State)
		assert.Equal(t, "Configuring installation", status.Description)

		// Get the config to verify it persisted
		newConfig, err := c.GetInstallationConfig()
		require.NoError(t, err, "GetInstallationConfig should succeed after setting config")
		assert.Equal(t, config.DataDirectory, newConfig.DataDirectory)
		assert.Equal(t, config.AdminConsolePort, newConfig.AdminConsolePort)
		assert.Equal(t, config.NetworkInterface, newConfig.NetworkInterface)

		// Verify host configuration was performed
		mockHostUtils.AssertExpectations(t)
	})

	// Test ConfigureInstallation validation error
	t.Run("ConfigureInstallation validation error", func(t *testing.T) {
		// Create an invalid config (port conflict)
		config := &types.InstallationConfig{
			DataDirectory:           "/tmp/new-dir",
			AdminConsolePort:        8080,
			LocalArtifactMirrorPort: 8080, // Same as AdminConsolePort
			GlobalCIDR:              "10.0.0.0/16",
			NetworkInterface:        "eth0",
		}

		// Configure the installation using the client
		_, err := c.ConfigureInstallation(config)
		require.Error(t, err, "ConfigureInstallation should fail with invalid config")

		// Verify the error is of type APIError
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
		// Error message should contain both variants of the port conflict message
		assert.True(t,
			strings.Contains(apiErr.Error(), "Admin Console Port and localArtifactMirrorPort cannot be equal") &&
				strings.Contains(apiErr.Error(), "adminConsolePort and Local Artifact Mirror Port cannot be equal"),
			"Error message should contain both variants of the port conflict message",
		)
	})

	// Test SetInstallStatus
	t.Run("SetInstallStatus", func(t *testing.T) {
		// Create a status
		status := &types.Status{
			State:       types.StateFailed,
			Description: "Installation failed",
		}

		// Set the status using the client
		newStatus, err := c.SetInstallStatus(status)
		require.NoError(t, err, "SetInstallStatus should succeed")
		assert.NotNil(t, newStatus, "Install should not be nil")
		assert.Equal(t, status, newStatus, "Install status should match the one set")
	})
}
