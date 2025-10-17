package install

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	kubernetesinstall "github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/install"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	kubernetesinstallationmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/kubernetes/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestKubernetesConfigureInstallation(t *testing.T) {
	// Test scenarios
	testCases := []struct {
		name                 string
		token                string
		config               types.KubernetesInstallationConfig
		expectedConfig       types.KubernetesInstallationConfig
		expectedStatus       *types.Status
		expectedStatusCode   int
		expectedError        bool
		validateInstallation func(t *testing.T, ki kubernetesinstallation.Installation)
	}{
		{
			name:  "Valid config",
			token: "TOKEN",
			config: types.KubernetesInstallationConfig{
				AdminConsolePort: 9000,
				HTTPProxy:        "http://proxy.example.com",
				HTTPSProxy:       "https://proxy.example.com",
				NoProxy:          "somecompany.internal,192.168.17.0/24",
			},
			expectedConfig: types.KubernetesInstallationConfig{
				AdminConsolePort: 9000,
				HTTPProxy:        "http://proxy.example.com",
				HTTPSProxy:       "https://proxy.example.com",
				NoProxy:          "somecompany.internal,192.168.17.0/24",
			},
			expectedStatus: &types.Status{
				State:       types.StateSucceeded,
				Description: "Installation configured",
			},
			expectedStatusCode: http.StatusOK,
			expectedError:      false,
			validateInstallation: func(t *testing.T, ki kubernetesinstallation.Installation) {
				assert.Equal(t, 9000, ki.AdminConsolePort())
				assert.Equal(t, &ecv1beta1.ProxySpec{
					HTTPProxy:       "http://proxy.example.com",
					HTTPSProxy:      "https://proxy.example.com",
					NoProxy:         "somecompany.internal,192.168.17.0/24",
					ProvidedNoProxy: "somecompany.internal,192.168.17.0/24",
				}, ki.ProxySpec())
			},
		},
		{
			name:  "Valid config with default admin console port",
			token: "TOKEN",
			config: types.KubernetesInstallationConfig{
				AdminConsolePort: 30000, // Use the default value explicitly
				HTTPProxy:        "http://proxy.example.com",
				HTTPSProxy:       "https://proxy.example.com",
				NoProxy:          "somecompany.internal,192.168.17.0/24",
			},
			expectedConfig: types.KubernetesInstallationConfig{
				AdminConsolePort: 30000,
				HTTPProxy:        "http://proxy.example.com",
				HTTPSProxy:       "https://proxy.example.com",
				NoProxy:          "somecompany.internal,192.168.17.0/24",
			},
			expectedStatus: &types.Status{
				State:       types.StateSucceeded,
				Description: "Installation configured",
			},
			expectedStatusCode: http.StatusOK,
			expectedError:      false,
			validateInstallation: func(t *testing.T, ki kubernetesinstallation.Installation) {
				assert.Equal(t, ecv1beta1.DefaultAdminConsolePort, ki.AdminConsolePort())
				assert.Equal(t, &ecv1beta1.ProxySpec{
					HTTPProxy:       "http://proxy.example.com",
					HTTPSProxy:      "https://proxy.example.com",
					NoProxy:         "somecompany.internal,192.168.17.0/24",
					ProvidedNoProxy: "somecompany.internal,192.168.17.0/24",
				}, ki.ProxySpec())
			},
		},
		{
			name:  "Invalid config - port conflict with manager",
			token: "TOKEN",
			config: types.KubernetesInstallationConfig{
				AdminConsolePort: 30080, // Same as DefaultManagerPort
				HTTPProxy:        "http://proxy.example.com",
				HTTPSProxy:       "https://proxy.example.com",
				NoProxy:          "somecompany.internal,192.168.17.0/24",
			},
			expectedConfig: types.KubernetesInstallationConfig{},
			expectedStatus: &types.Status{
				State:       types.StateFailed,
				Description: "validate: field errors: adminConsolePort cannot be the same as the manager port",
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedError:      true,
		},
		{
			name:  "Valid config - missing admin console port will use defaults",
			token: "TOKEN",
			config: types.KubernetesInstallationConfig{
				AdminConsolePort: 0, // Missing port
				HTTPProxy:        "http://proxy.example.com",
				HTTPSProxy:       "https://proxy.example.com",
				NoProxy:          "somecompany.internal,192.168.17.0/24",
			},
			expectedConfig: types.KubernetesInstallationConfig{
				AdminConsolePort: 30000, // DefaultAdminConsolePort
				HTTPProxy:        "http://proxy.example.com",
				HTTPSProxy:       "https://proxy.example.com",
				NoProxy:          "somecompany.internal,192.168.17.0/24",
			},
			expectedStatus: &types.Status{
				State:       types.StateSucceeded,
				Description: "Installation configured",
			},
			expectedStatusCode: http.StatusOK,
			expectedError:      false,
		},
		{
			name:               "Unauthorized",
			token:              "NOT_A_TOKEN",
			config:             types.KubernetesInstallationConfig{},
			expectedStatusCode: http.StatusUnauthorized,
			expectedError:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ki := kubernetesinstallation.New(nil)

			// Create an install controller with the mock installation
			installController, err := kubernetesinstall.NewInstallController(
				kubernetesinstall.WithInstallation(ki),
				kubernetesinstall.WithStateMachine(kubernetesinstall.NewStateMachine(kubernetesinstall.WithCurrentState(states.StateApplicationConfigured))),
				kubernetesinstall.WithReleaseData(integration.DefaultReleaseData()),
				kubernetesinstall.WithHelmClient(&helm.MockClient{}),
			)
			require.NoError(t, err)

			// Create the API with the install controller
			apiInstance := integration.NewTargetKubernetesAPIWithReleaseData(t, types.ModeInstall,
				api.WithKubernetesInstallController(installController),
				api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
				api.WithLogger(logger.NewDiscardLogger()),
			)

			// Create a router and register the API routes
			router := mux.NewRouter()
			apiInstance.RegisterRoutes(router)

			// Serialize the config to JSON
			configJSON, err := json.Marshal(tc.config)
			require.NoError(t, err)

			// Create a request
			req := httptest.NewRequest(http.MethodPost, "/kubernetes/install/installation/configure", bytes.NewReader(configJSON))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tc.token)
			rec := httptest.NewRecorder()

			// Serve the request
			router.ServeHTTP(rec, req)

			// Check the response
			assert.Equal(t, tc.expectedStatusCode, rec.Code)

			t.Logf("Response body: %s", rec.Body.String())

			// Parse the response body
			if tc.expectedError {
				var apiError types.APIError
				err = json.NewDecoder(rec.Body).Decode(&apiError)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedStatusCode, apiError.StatusCode)
				assert.NotEmpty(t, apiError.Message)
			} else {
				var status types.Status
				err = json.NewDecoder(rec.Body).Decode(&status)
				require.NoError(t, err)

				// Wait for the state to transition from Pending (async operation should start quickly)
				assert.Eventually(t, func() bool {
					status, err := installController.GetInstallationStatus(t.Context())
					require.NoError(t, err)
					t.Logf("Installation Status: %s", status.State)
					return status.State != types.StatePending
				}, 5*time.Second, 100*time.Millisecond, "Installation configuration state did not transition from Pending")
			}

			// We might not have an expected status if the test is expected to fail before running the controller logic
			if tc.expectedStatus != nil {
				// The status is set in a goroutine, so we need to wait for it
				assert.Eventually(t, func() bool {
					status, err := installController.GetInstallationStatus(t.Context())
					require.NoError(t, err)
					return status.State == tc.expectedStatus.State
				}, 1*time.Second, 100*time.Millisecond, fmt.Sprintf("Expected status to be %s", tc.expectedStatus.State))

				// Get the final status to check the description
				finalStatus, err := installController.GetInstallationStatus(t.Context())
				require.NoError(t, err)
				assert.Contains(t, finalStatus.Description, tc.expectedStatus.Description)
			}

			if !tc.expectedError {
				// Verify that the config is in the store
				storedConfig, err := installController.GetInstallationConfig(t.Context())
				require.NoError(t, err)
				assert.Equal(t, tc.expectedConfig, storedConfig.Resolved)

				// Verify that the installation was updated
				if tc.validateInstallation != nil {
					tc.validateInstallation(t, ki)
				}
			}
		})
	}
}

// Test that config validation errors are properly returned for Kubernetes installation
func TestKubernetesConfigureInstallationValidation(t *testing.T) {
	ki := kubernetesinstallation.New(nil)
	ki.SetManagerPort(9001)

	// Create an install controller with the mock installation
	installController, err := kubernetesinstall.NewInstallController(
		kubernetesinstall.WithInstallation(ki),
		kubernetesinstall.WithStateMachine(kubernetesinstall.NewStateMachine(kubernetesinstall.WithCurrentState(states.StateApplicationConfigured))),
		kubernetesinstall.WithReleaseData(integration.DefaultReleaseData()),
		kubernetesinstall.WithHelmClient(&helm.MockClient{}),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance := integration.NewTargetKubernetesAPIWithReleaseData(t, types.ModeInstall,
		api.WithKubernetesInstallController(installController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test a validation error case with port conflict
	config := types.KubernetesInstallationConfig{
		AdminConsolePort: 9001, // Same as ManagerPort
		HTTPProxy:        "http://proxy.example.com",
		HTTPSProxy:       "https://proxy.example.com",
		NoProxy:          "somecompany.internal,192.168.17.0/24",
	}

	// Serialize the config to JSON
	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	// Create a request
	req := httptest.NewRequest(http.MethodPost, "/kubernetes/install/installation/configure", bytes.NewReader(configJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+"TOKEN")
	rec := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(rec, req)

	// Check the response
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	t.Logf("Response body: %s", rec.Body.String())

	// We expect a ValidationError with specific error about port conflict
	var apiError types.APIError
	err = json.NewDecoder(rec.Body).Decode(&apiError)
	require.NoError(t, err)
	assert.Contains(t, apiError.Error(), "adminConsolePort cannot be the same as the manager port")
	// Also verify the field name is correct
	assert.Equal(t, "adminConsolePort", apiError.Errors[0].Field)
}

// Test that the endpoint properly handles malformed JSON for Kubernetes installation
func TestKubernetesConfigureInstallationBadRequest(t *testing.T) {
	ki := kubernetesinstallation.New(nil)

	// Create an install controller with the mock installation
	installController, err := kubernetesinstall.NewInstallController(
		kubernetesinstall.WithInstallation(ki),
		kubernetesinstall.WithStateMachine(kubernetesinstall.NewStateMachine(kubernetesinstall.WithCurrentState(states.StateApplicationConfigured))),
		kubernetesinstall.WithReleaseData(integration.DefaultReleaseData()),
		kubernetesinstall.WithHelmClient(&helm.MockClient{}),
	)
	require.NoError(t, err)

	apiInstance := integration.NewTargetKubernetesAPIWithReleaseData(t, types.ModeInstall,
		api.WithKubernetesInstallController(installController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)

	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Create a request with invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/kubernetes/install/installation/configure",
		bytes.NewReader([]byte(`{"adminConsolePort": "not-a-number"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+"TOKEN")
	rec := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(rec, req)

	// Check the response
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	t.Logf("Response body: %s", rec.Body.String())
}

// Test that the server returns proper errors when the API controller fails for Kubernetes installation
func TestKubernetesConfigureInstallationControllerError(t *testing.T) {
	// Create a mock controller that returns an error
	mockController := &kubernetesinstall.MockController{}
	mockController.On("ConfigureInstallation", mock.Anything, mock.Anything).Return(assert.AnError)

	// Create the API with the mock controller
	apiInstance := integration.NewTargetKubernetesAPIWithReleaseData(t, types.ModeInstall,
		api.WithKubernetesInstallController(mockController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)

	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Create a valid config request
	config := types.KubernetesInstallationConfig{
		AdminConsolePort: 9000,
		HTTPProxy:        "http://proxy.example.com",
		HTTPSProxy:       "https://proxy.example.com",
		NoProxy:          "somecompany.internal,192.168.17.0/24",
	}
	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	// Create a request
	req := httptest.NewRequest(http.MethodPost, "/kubernetes/install/installation/configure", bytes.NewReader(configJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+"TOKEN")
	rec := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(rec, req)

	// Check the response
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	t.Logf("Response body: %s", rec.Body.String())

	// Verify mock expectations
	mockController.AssertExpectations(t)
}

// Test the getInstall endpoint returns installation data correctly for Kubernetes
func TestKubernetesGetInstallationConfig(t *testing.T) {
	ki := kubernetesinstallation.New(nil)

	// Create a config manager
	installationManager := kubernetesinstallationmanager.NewInstallationManager()

	// Create an install controller with the config manager
	installController, err := kubernetesinstall.NewInstallController(
		kubernetesinstall.WithInstallation(ki),
		kubernetesinstall.WithInstallationManager(installationManager),
		kubernetesinstall.WithReleaseData(integration.DefaultReleaseData()),
		kubernetesinstall.WithHelmClient(&helm.MockClient{}),
	)
	require.NoError(t, err)

	// Set some initial config
	initialConfig := types.KubernetesInstallationConfig{
		AdminConsolePort: 8800,
		HTTPProxy:        "http://proxy.example.com",
		HTTPSProxy:       "https://proxy.example.com",
		NoProxy:          "somecompany.internal,192.168.17.0/24",
	}
	err = installationManager.SetConfigValues(initialConfig)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance := integration.NewTargetKubernetesAPIWithReleaseData(t, types.ModeInstall,
		api.WithKubernetesInstallController(installController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test successful get
	t.Run("Success", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/kubernetes/install/installation/config", nil)
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var configResponse types.KubernetesInstallationConfigResponse
		err = json.NewDecoder(rec.Body).Decode(&configResponse)
		require.NoError(t, err)

		// Verify the installation data matches what we expect
		assert.Equal(t, initialConfig.AdminConsolePort, configResponse.Values.AdminConsolePort)
		assert.Equal(t, initialConfig.HTTPProxy, configResponse.Values.HTTPProxy)
		assert.Equal(t, initialConfig.HTTPSProxy, configResponse.Values.HTTPSProxy)
		assert.Equal(t, initialConfig.NoProxy, configResponse.Values.NoProxy)

		// Verify that defaults are properly populated
		assert.Equal(t, ecv1beta1.DefaultAdminConsolePort, configResponse.Defaults.AdminConsolePort)
	})

	// Test get with default/empty configuration
	t.Run("Default configuration", func(t *testing.T) {
		ki := kubernetesinstallation.New(nil)

		// Create a fresh config manager without writing anything
		emptyInstallationManager := kubernetesinstallationmanager.NewInstallationManager()

		// Create an install controller with the empty config manager
		emptyInstallController, err := kubernetesinstall.NewInstallController(
			kubernetesinstall.WithInstallation(ki),
			kubernetesinstall.WithInstallationManager(emptyInstallationManager),
			kubernetesinstall.WithReleaseData(integration.DefaultReleaseData()),
			kubernetesinstall.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		emptyAPI := integration.NewTargetKubernetesAPIWithReleaseData(t, types.ModeInstall,
			api.WithKubernetesInstallController(emptyInstallController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		// Create a router and register the API routes
		emptyRouter := mux.NewRouter()
		emptyAPI.RegisterRoutes(emptyRouter)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/kubernetes/install/installation/config", nil)
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		emptyRouter.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var configResponse types.KubernetesInstallationConfigResponse
		err = json.NewDecoder(rec.Body).Decode(&configResponse)
		require.NoError(t, err)

		// Verify the installation data contains empty values
		assert.Equal(t, 0, configResponse.Values.AdminConsolePort)
		assert.Equal(t, "", configResponse.Values.HTTPProxy)
		assert.Equal(t, "", configResponse.Values.HTTPSProxy)
		assert.Equal(t, "", configResponse.Values.NoProxy)

		// Verify that defaults are properly populated
		assert.Equal(t, ecv1beta1.DefaultAdminConsolePort, configResponse.Defaults.AdminConsolePort)
	})

	// Test authorization
	t.Run("Authorization error", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/kubernetes/install/installation/config", nil)
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
		mockController := &kubernetesinstall.MockController{}
		mockController.On("GetInstallationConfig", mock.Anything).Return(types.KubernetesInstallationConfigResponse{}, assert.AnError)

		// Create the API with the mock controller
		apiInstance := integration.NewTargetKubernetesAPIWithReleaseData(t, types.ModeInstall,
			api.WithKubernetesInstallController(mockController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/kubernetes/install/installation/config", nil)
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

		// Verify mock expectations
		mockController.AssertExpectations(t)
	})
}
