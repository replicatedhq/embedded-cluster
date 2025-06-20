package integration

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/api"
	apiclient "github.com/replicatedhq/embedded-cluster/api/client"
	"github.com/replicatedhq/embedded-cluster/api/controllers/install"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/preflight"
	preflightstore "github.com/replicatedhq/embedded-cluster/api/internal/store/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/pkg/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	metadatafake "k8s.io/client-go/metadata/fake"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var (
	//go:embed assets/license.yaml
	licenseData []byte
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

func (m *mockInstallController) GetInstallationConfig(ctx context.Context) (types.InstallationConfig, error) {
	if m.getInstallationConfigError != nil {
		return types.InstallationConfig{}, m.getInstallationConfigError
	}
	return types.InstallationConfig{}, nil
}

func (m *mockInstallController) ConfigureInstallation(ctx context.Context, config types.InstallationConfig) error {
	return m.configureInstallationError
}

func (m *mockInstallController) GetInstallationStatus(ctx context.Context) (types.Status, error) {
	if m.readStatusError != nil {
		return types.Status{}, m.readStatusError
	}
	return types.Status{}, nil
}

func (m *mockInstallController) RunHostPreflights(ctx context.Context, opts install.RunHostPreflightsOptions) error {
	return m.runHostPreflightsError
}

func (m *mockInstallController) GetHostPreflightStatus(ctx context.Context) (types.Status, error) {
	if m.getHostPreflightStatusError != nil {
		return types.Status{}, m.getHostPreflightStatusError
	}
	return types.Status{}, nil
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

func (m *mockInstallController) GetInfra(ctx context.Context) (types.Infra, error) {
	if m.getInfraError != nil {
		return types.Infra{}, m.getInfraError
	}
	return types.Infra{}, nil
}

func (m *mockInstallController) SetStatus(ctx context.Context, status types.Status) error {
	return m.setStatusError
}

func (m *mockInstallController) GetStatus(ctx context.Context) (types.Status, error) {
	return types.Status{}, m.readStatusError
}

func TestConfigureInstallation(t *testing.T) {
	// Test scenarios
	testCases := []struct {
		name                  string
		mockHostUtils         *hostutils.MockHostUtils
		mockNetUtils          *utils.MockNetUtils
		token                 string
		config                types.InstallationConfig
		expectedStatus        int
		expectedError         bool
		validateRuntimeConfig func(t *testing.T, rc runtimeconfig.RuntimeConfig)
	}{
		{
			name: "Valid config",
			mockHostUtils: func() *hostutils.MockHostUtils {
				mockHostUtils := &hostutils.MockHostUtils{}
				mockHostUtils.On("ConfigureHost", mock.Anything,
					mock.MatchedBy(func(rc runtimeconfig.RuntimeConfig) bool {
						return rc.EmbeddedClusterHomeDirectory() == "/tmp/data" &&
							rc.AdminConsolePort() == 8000 &&
							rc.LocalArtifactMirrorPort() == 8081 &&
							rc.NetworkInterface() == "eth0" &&
							rc.GlobalCIDR() == "10.0.0.0/16" &&
							rc.PodCIDR() == "10.0.0.0/17" &&
							rc.ServiceCIDR() == "10.0.128.0/17" &&
							rc.NodePortRange() == "80-32767"
					}),
					mock.Anything,
				).Return(nil).Once()
				return mockHostUtils
			}(),
			mockNetUtils: &utils.MockNetUtils{},
			token:        "TOKEN",
			config: types.InstallationConfig{
				DataDirectory:           "/tmp/data",
				AdminConsolePort:        8000,
				LocalArtifactMirrorPort: 8081,
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
			validateRuntimeConfig: func(t *testing.T, rc runtimeconfig.RuntimeConfig) {
				assert.Equal(t, "/tmp/data", rc.EmbeddedClusterHomeDirectory())
				assert.Equal(t, 8000, rc.AdminConsolePort())
				assert.Equal(t, 8081, rc.LocalArtifactMirrorPort())
				assert.Equal(t, ecv1beta1.NetworkSpec{
					NetworkInterface: "eth0",
					GlobalCIDR:       "10.0.0.0/16",
					PodCIDR:          "10.0.0.0/17",
					ServiceCIDR:      "10.0.128.0/17",
					NodePortRange:    "80-32767",
				}, rc.Get().Network)
				assert.Nil(t, rc.Get().Proxy)
			},
		},
		{
			name: "Valid config with proxy",
			mockHostUtils: func() *hostutils.MockHostUtils {
				mockHostUtils := &hostutils.MockHostUtils{}
				mockHostUtils.On("ConfigureHost", mock.Anything,
					mock.MatchedBy(func(rc runtimeconfig.RuntimeConfig) bool {
						return rc.EmbeddedClusterHomeDirectory() == "/tmp/data" &&
							rc.AdminConsolePort() == 8000 &&
							rc.LocalArtifactMirrorPort() == 8081 &&
							rc.NetworkInterface() == "eth0" &&
							rc.GlobalCIDR() == "10.0.0.0/16" &&
							rc.PodCIDR() == "10.0.0.0/17" &&
							rc.ServiceCIDR() == "10.0.128.0/17" &&
							rc.NodePortRange() == "80-32767" &&
							rc.ProxySpec().HTTPProxy == "http://proxy.example.com" &&
							rc.ProxySpec().HTTPSProxy == "https://proxy.example.com" &&
							rc.ProxySpec().ProvidedNoProxy == "somecompany.internal,192.168.17.0/24"
					}),
					mock.Anything,
				).Return(nil).Once()
				return mockHostUtils
			}(),
			mockNetUtils: func() *utils.MockNetUtils {
				mockNetUtils := &utils.MockNetUtils{}
				mockNetUtils.On("FirstValidIPNet", "eth0").Return(&net.IPNet{IP: net.ParseIP("192.168.17.12"), Mask: net.CIDRMask(24, 32)}, nil)
				return mockNetUtils
			}(),
			token: "TOKEN",
			config: types.InstallationConfig{
				DataDirectory:           "/tmp/data",
				AdminConsolePort:        8000,
				LocalArtifactMirrorPort: 8081,
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
				HTTPProxy:               "http://proxy.example.com",
				HTTPSProxy:              "https://proxy.example.com",
				NoProxy:                 "somecompany.internal,192.168.17.0/24",
			},
			expectedStatus: http.StatusOK,
			expectedError:  false,
			validateRuntimeConfig: func(t *testing.T, rc runtimeconfig.RuntimeConfig) {
				assert.Equal(t, "/tmp/data", rc.EmbeddedClusterHomeDirectory())
				assert.Equal(t, 8000, rc.AdminConsolePort())
				assert.Equal(t, 8081, rc.LocalArtifactMirrorPort())
				assert.Equal(t, ecv1beta1.NetworkSpec{
					NetworkInterface: "eth0",
					GlobalCIDR:       "10.0.0.0/16",
					PodCIDR:          "10.0.0.0/17",
					ServiceCIDR:      "10.0.128.0/17",
					NodePortRange:    "80-32767",
				}, rc.Get().Network)
				assert.Equal(t, &ecv1beta1.ProxySpec{
					HTTPProxy:       "http://proxy.example.com",
					HTTPSProxy:      "https://proxy.example.com",
					NoProxy:         "localhost,127.0.0.1,.cluster.local,.svc,169.254.169.254,10.0.0.0/17,10.0.128.0/17,somecompany.internal,192.168.17.0/24",
					ProvidedNoProxy: "somecompany.internal,192.168.17.0/24",
				}, rc.Get().Proxy)
			},
		},
		{
			name:          "Invalid config - port conflict",
			mockHostUtils: &hostutils.MockHostUtils{},
			mockNetUtils:  &utils.MockNetUtils{},
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
			mockNetUtils:   &utils.MockNetUtils{},
			token:          "NOT_A_TOKEN",
			config:         types.InstallationConfig{},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a runtime config
			rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))

			// Create an install controller with the config manager
			installController, err := install.NewInstallController(
				install.WithRuntimeConfig(rc),
				install.WithStateMachine(install.NewStateMachine(install.WithCurrentState(install.StateHostConfigured))),
				install.WithHostUtils(tc.mockHostUtils),
				install.WithNetUtils(tc.mockNetUtils),
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

				// Verify that the status is not pending. We cannot check for an end state here because the hots config is async
				// so the state might have moved from running to a final state before we get the response.
				assert.NotEqual(t, types.StatePending, status.State)
			}

			if !tc.expectedError {
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
			tc.mockNetUtils.AssertExpectations(t)

			if tc.validateRuntimeConfig != nil {
				tc.validateRuntimeConfig(t, rc)
			}
		})
	}
}

// Test that config validation errors are properly returned
func TestConfigureInstallationValidation(t *testing.T) {
	rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
	rc.SetDataDir(t.TempDir())

	// Create an install controller with the config manager
	installController, err := install.NewInstallController(
		install.WithRuntimeConfig(rc),
		install.WithStateMachine(install.NewStateMachine(install.WithCurrentState(install.StateHostConfigured))),
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
	assert.Contains(t, apiError.Error(), "serviceCidr is required when globalCidr is not set")
	// Also verify the field name is correct
	assert.Equal(t, "serviceCidr", apiError.Errors[0].Field)
}

// Test that the endpoint properly handles malformed JSON
func TestConfigureInstallationBadRequest(t *testing.T) {
	rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
	rc.SetDataDir(t.TempDir())

	// Create an install controller with the config manager
	installController, err := install.NewInstallController(
		install.WithRuntimeConfig(rc),
		install.WithStateMachine(install.NewStateMachine(install.WithCurrentState(install.StateHostConfigured))),
	)
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
	rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
	rc.SetDataDir(t.TempDir())

	// Create a config manager
	installationManager := installation.NewInstallationManager()

	// Create an install controller with the config manager
	installController, err := install.NewInstallController(
		install.WithRuntimeConfig(rc),
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
		netUtils := &utils.MockNetUtils{}
		netUtils.On("ListValidNetworkInterfaces").Return([]string{"eth0", "eth1"}, nil).Once()
		netUtils.On("DetermineBestNetworkInterface").Return("eth0", nil).Once()

		rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
		rc.SetDataDir(t.TempDir())

		// Create a fresh config manager without writing anything
		emptyInstallationManager := installation.NewInstallationManager(
			installation.WithNetUtils(netUtils),
		)

		// Create an install controller with the empty config manager
		emptyInstallController, err := install.NewInstallController(
			install.WithRuntimeConfig(rc),
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
	err = installController.SetStatus(t.Context(), initialStatus)
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
	rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
	rc.SetDataDir(t.TempDir())

	// Create a mock hostutils
	mockHostUtils := &hostutils.MockHostUtils{}
	mockHostUtils.On("ConfigureHost", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Create a config manager
	installationManager := installation.NewInstallationManager(
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
	c := apiclient.New(server.URL, apiclient.WithToken("TOKEN"))
	require.NoError(t, err, "API client login should succeed")

	// Test GetInstallationConfig
	t.Run("GetInstallationConfig", func(t *testing.T) {
		config, err := c.GetInstallationConfig()
		require.NoError(t, err, "GetInstallationConfig should succeed")

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
		_, err = c.ConfigureInstallation(config)
		require.NoError(t, err, "ConfigureInstallation should succeed with valid config")

		// Verify the status was set correctly
		var installStatus types.Status
		if !assert.Eventually(t, func() bool {
			installStatus, err = c.GetInstallationStatus()
			require.NoError(t, err, "GetInstallationStatus should succeed")
			return installStatus.State == types.StateSucceeded
		}, 1*time.Second, 100*time.Millisecond) {
			require.Equal(t, types.StateSucceeded, installStatus.State,
				"Installation not succeeded with state %s and description %s", installStatus.State, installStatus.Description)
		}

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
		config := types.InstallationConfig{
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
		// Error message should contain the same port conflict message for both fields
		assert.Equal(t, 2, strings.Count(apiErr.Error(), "adminConsolePort and localArtifactMirrorPort cannot be equal"))
	})

	// Test SetInstallStatus
	t.Run("SetInstallStatus", func(t *testing.T) {
		// Create a status
		status := types.Status{
			State:       types.StateFailed,
			Description: "Installation failed",
		}

		// Set the status using the client
		newStatus, err := c.SetInstallStatus(status)
		require.NoError(t, err, "SetInstallStatus should succeed")
		assert.Equal(t, status, newStatus, "Install status should match the one set")
	})
}

// Test the setupInfra endpoint runs infrastructure setup correctly
func TestPostSetupInfra(t *testing.T) {
	// Create schemes
	scheme := runtime.NewScheme()
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))

	metascheme := metadatafake.NewTestScheme()
	require.NoError(t, metav1.AddMetaToScheme(metascheme))
	require.NoError(t, corev1.AddToScheme(metascheme))

	t.Run("Success", func(t *testing.T) {
		// Create mocks
		k0sMock := &k0s.MockK0s{}
		helmMock := &helm.MockClient{}
		hostutilsMock := &hostutils.MockHostUtils{}
		fakeKcli := clientfake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(testControllerNode(t)).
			WithStatusSubresource(&ecv1beta1.Installation{}, &apiextensionsv1.CustomResourceDefinition{}).
			WithInterceptorFuncs(testInterceptorFuncs(t)).
			Build()
		fakeMcli := metadatafake.NewSimpleMetadataClient(metascheme)

		// Create a runtime config
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())
		rc.SetNetworkSpec(ecv1beta1.NetworkSpec{
			NetworkInterface: "eth0",
			ServiceCIDR:      "10.96.0.0/12",
			PodCIDR:          "10.244.0.0/16",
		})

		// Create host preflights with successful status
		hpf := types.HostPreflights{}
		hpf.Status = types.Status{
			State:       types.StateSucceeded,
			Description: "Host preflights succeeded",
		}

		// Create host preflights manager
		pfManager := preflight.NewHostPreflightManager(
			preflight.WithHostPreflightStore(preflightstore.NewMemoryStore(preflightstore.WithHostPreflight(hpf))),
		)

		// Create infra manager with mocks
		infraManager := infra.NewInfraManager(
			infra.WithK0s(k0sMock),
			infra.WithKubeClient(fakeKcli),
			infra.WithMetadataClient(fakeMcli),
			infra.WithHelmClient(helmMock),
			infra.WithLicense(licenseData),
			infra.WithHostUtils(hostutilsMock),
			infra.WithKotsInstaller(func() error {
				return nil
			}),
			infra.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease: &release.ChannelRelease{
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.example.com",
						ProxyRegistryDomain: "some-proxy.example.com",
					},
				},
			}),
		)

		// Setup mock expectations
		k0sConfig := &k0sv1beta1.ClusterConfig{
			Spec: &k0sv1beta1.ClusterSpec{
				Network: &k0sv1beta1.Network{
					PodCIDR:     "10.244.0.0/16",
					ServiceCIDR: "10.96.0.0/12",
				},
			},
		}
		mock.InOrder(
			k0sMock.On("IsInstalled").Return(false, nil),
			k0sMock.On("WriteK0sConfig", mock.Anything, "eth0", "", "10.244.0.0/16", "10.96.0.0/12", mock.Anything, mock.Anything).Return(k0sConfig, nil),
			hostutilsMock.On("CreateSystemdUnitFiles", mock.Anything, mock.Anything, rc, false).Return(nil),
			k0sMock.On("Install", rc).Return(nil),
			k0sMock.On("WaitForK0s").Return(nil),
			hostutilsMock.On("AddInsecureRegistry", mock.Anything).Return(nil),
			helmMock.On("Install", mock.Anything, mock.Anything).Times(4).Return(nil, nil), // 4 addons
			helmMock.On("Close").Return(nil),
		)

		// Create an install controller with the mocked managers
		installController, err := install.NewInstallController(
			install.WithRuntimeConfig(rc),
			install.WithStateMachine(install.NewStateMachine(install.WithCurrentState(install.StatePreflightsSucceeded))),
			install.WithHostPreflightManager(pfManager),
			install.WithInfraManager(infraManager),
			install.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease: &release.ChannelRelease{
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.example.com",
						ProxyRegistryDomain: "some-proxy.example.com",
					},
				},
			}),
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

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/install/infra/setup", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)

		t.Logf("Response body: %s", rec.Body.String())

		// Parse the response body
		var infra types.Infra
		err = json.NewDecoder(rec.Body).Decode(&infra)
		require.NoError(t, err)

		// Verify that the status is not pending. We cannot check for an end state here because the hots config is async
		// so the state might have moved from running to a final state before we get the response.
		assert.NotEqual(t, types.StatePending, infra.Status.State)

		// Helper function to get infra status
		getInfraStatus := func() types.Infra {
			// Create a request to get infra status
			req := httptest.NewRequest(http.MethodGet, "/install/infra/status", nil)
			req.Header.Set("Authorization", "Bearer TOKEN")
			rec := httptest.NewRecorder()

			// Serve the request
			router.ServeHTTP(rec, req)

			// Check the response
			assert.Equal(t, http.StatusOK, rec.Code)

			// Parse the response body
			var infra types.Infra
			err = json.NewDecoder(rec.Body).Decode(&infra)
			require.NoError(t, err)

			// Log the infra status
			t.Logf("Infra Status: %s, Description: %s", infra.Status.State, infra.Status.Description)

			return infra
		}

		// The status should eventually be set to succeeded in a goroutine
		assert.Eventually(t, func() bool {
			infra := getInfraStatus()

			// Fail the test if the status is Failed
			if infra.Status.State == types.StateFailed {
				t.Fatalf("Infrastructure setup failed: %s", infra.Status.Description)
			}

			return infra.Status.State == types.StateSucceeded
		}, 30*time.Second, 500*time.Millisecond, "Infrastructure setup did not succeed in time")

		// Verify that the mock expectations were met
		k0sMock.AssertExpectations(t)
		hostutilsMock.AssertExpectations(t)
		helmMock.AssertExpectations(t)

		// Verify installation was created
		gotInst, err := kubeutils.GetLatestInstallation(t.Context(), fakeKcli)
		require.NoError(t, err)
		assert.Equal(t, ecv1beta1.InstallationStateInstalled, gotInst.Status.State)

		// Verify version metadata configmap was created
		var gotConfigmap corev1.ConfigMap
		err = fakeKcli.Get(t.Context(), client.ObjectKey{Namespace: "embedded-cluster", Name: "version-metadata-0-0-0"}, &gotConfigmap)
		require.NoError(t, err)

		// Verify kotsadm namespace and kotsadm-password secret were created
		var gotKotsadmNamespace corev1.Namespace
		err = fakeKcli.Get(t.Context(), client.ObjectKey{Name: runtimeconfig.KotsadmNamespace}, &gotKotsadmNamespace)
		require.NoError(t, err)

		var gotKotsadmPasswordSecret corev1.Secret
		err = fakeKcli.Get(t.Context(), client.ObjectKey{Namespace: runtimeconfig.KotsadmNamespace, Name: "kotsadm-password"}, &gotKotsadmPasswordSecret)
		require.NoError(t, err)
		assert.NotEmpty(t, gotKotsadmPasswordSecret.Data["passwordBcrypt"])

		// Get infra status again and verify more details
		infra = getInfraStatus()
		assert.Contains(t, infra.Logs, "[k0s]")
		assert.Contains(t, infra.Logs, "[metadata]")
		assert.Contains(t, infra.Logs, "[addons]")
		assert.Contains(t, infra.Logs, "[extensions]")
		assert.Len(t, infra.Components, 6)
	})

	// Test authorization
	t.Run("Authorization error", func(t *testing.T) {
		// Create the API
		apiInstance, err := api.New(
			"password",
			api.WithAuthController(&staticAuthController{"TOKEN"}),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/install/infra/setup", nil)
		req.Header.Set("Authorization", "Bearer NOT_A_TOKEN")
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

	// Test preflight checks not completed
	t.Run("Preflight checks not completed", func(t *testing.T) {
		// Create host preflights with running status (not completed)
		hpf := types.HostPreflights{}
		hpf.Status = types.Status{
			State:       types.StateRunning,
			Description: "Host preflights running",
		}

		// Create managers
		pfManager := preflight.NewHostPreflightManager(
			preflight.WithHostPreflightStore(preflightstore.NewMemoryStore(preflightstore.WithHostPreflight(hpf))),
		)

		// Create an install controller
		installController, err := install.NewInstallController(
			install.WithStateMachine(install.NewStateMachine(install.WithCurrentState(install.StatePreflightsRunning))),
			install.WithHostPreflightManager(pfManager),
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

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/install/infra/setup", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusConflict, rec.Code)

		t.Logf("Response body: %s", rec.Body.String())

		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusConflict, apiError.StatusCode)
		assert.Contains(t, apiError.Message, "invalid transition from PreflightsRunning to InfrastructureInstalling")
	})

	// Test k0s already installed error
	t.Run("K0s already installed", func(t *testing.T) {
		// Create a runtime config
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())
		rc.SetNetworkSpec(ecv1beta1.NetworkSpec{
			NetworkInterface: "eth0",
		})

		// Create host preflights with successful status
		hpf := types.HostPreflights{}
		hpf.Status = types.Status{
			State:       types.StateSucceeded,
			Description: "Host preflights succeeded",
		}

		// Create managers
		pfManager := preflight.NewHostPreflightManager(
			preflight.WithHostPreflightStore(preflightstore.NewMemoryStore(preflightstore.WithHostPreflight(hpf))),
		)

		// Create an install controller
		installController, err := install.NewInstallController(
			install.WithRuntimeConfig(rc),
			install.WithStateMachine(install.NewStateMachine(install.WithCurrentState(install.StateSucceeded))),
			install.WithHostPreflightManager(pfManager),
			install.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease:        &release.ChannelRelease{},
			}),
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

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/install/infra/setup", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Contains(t, rec.Body.String(), "invalid transition from Succeeded to InfrastructureInstalling")
	})

	// Test k0s install error
	t.Run("K0s install error", func(t *testing.T) {
		// Create mocks
		k0sMock := &k0s.MockK0s{}
		hostutilsMock := &hostutils.MockHostUtils{}

		// Create a runtime config
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())
		rc.SetNetworkSpec(ecv1beta1.NetworkSpec{
			NetworkInterface: "eth0",
			ServiceCIDR:      "10.96.0.0/12",
			PodCIDR:          "10.244.0.0/16",
		})

		// Create host preflights with successful status
		hpf := types.HostPreflights{}
		hpf.Status = types.Status{
			State:       types.StateSucceeded,
			Description: "Host preflights succeeded",
		}

		// Create managers
		pfManager := preflight.NewHostPreflightManager(
			preflight.WithHostPreflightStore(preflightstore.NewMemoryStore(preflightstore.WithHostPreflight(hpf))),
		)
		infraManager := infra.NewInfraManager(
			infra.WithK0s(k0sMock),
			infra.WithHostUtils(hostutilsMock),
			infra.WithLicense(licenseData),
		)

		// Setup k0s mock expectations with failure
		k0sConfig := &k0sv1beta1.ClusterConfig{}
		mock.InOrder(
			k0sMock.On("IsInstalled").Return(false, nil),
			k0sMock.On("WriteK0sConfig", mock.Anything, "eth0", "", "10.244.0.0/16", "10.96.0.0/12", mock.Anything, mock.Anything).Return(k0sConfig, nil),
			hostutilsMock.On("CreateSystemdUnitFiles", mock.Anything, mock.Anything, rc, false).Return(nil),
			k0sMock.On("Install", mock.Anything).Return(errors.New("failed to install k0s")),
		)

		// Create an install controller
		installController, err := install.NewInstallController(
			install.WithHostPreflightManager(pfManager),
			install.WithInfraManager(infraManager),
			install.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease:        &release.ChannelRelease{},
			}),
			install.WithRuntimeConfig(rc),
			install.WithStateMachine(install.NewStateMachine(install.WithCurrentState(install.StatePreflightsSucceeded))),
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

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/install/infra/setup", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)

		// The status should eventually be set to failed due to k0s install error
		assert.Eventually(t, func() bool {
			// Create a request to get infra status
			req := httptest.NewRequest(http.MethodGet, "/install/infra/status", nil)
			req.Header.Set("Authorization", "Bearer TOKEN")
			rec := httptest.NewRecorder()

			// Serve the request
			router.ServeHTTP(rec, req)

			// Check the response
			assert.Equal(t, http.StatusOK, rec.Code)

			// Parse the response body
			var infra types.Infra
			err = json.NewDecoder(rec.Body).Decode(&infra)
			require.NoError(t, err)

			t.Logf("Infra Status: %s, Description: %s", infra.Status.State, infra.Status.Description)
			return infra.Status.State == types.StateFailed && strings.Contains(infra.Status.Description, "failed to install k0s")
		}, 10*time.Second, 100*time.Millisecond, "Infrastructure setup did not fail in time")

		// Verify that the mock expectations were met
		k0sMock.AssertExpectations(t)
		hostutilsMock.AssertExpectations(t)
	})
}

func testControllerNode(t *testing.T) *corev1.Node {
	hostname, err := os.Hostname()
	require.NoError(t, err)
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: strings.ToLower(hostname),
			Labels: map[string]string{
				"node-role.kubernetes.io/control-plane": "",
			},
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
}

func testInterceptorFuncs(t *testing.T) interceptor.Funcs {
	return interceptor.Funcs{
		Create: func(ctx context.Context, cli client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition); ok {
				err := cli.Create(ctx, obj, opts...)
				if err != nil {
					return err
				}
				// Update status to ready after creation
				crd.Status.Conditions = []apiextensionsv1.CustomResourceDefinitionCondition{
					{Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue},
					{Type: apiextensionsv1.NamesAccepted, Status: apiextensionsv1.ConditionTrue},
				}
				return cli.Status().Update(ctx, crd)
			}
			return cli.Create(ctx, obj, opts...)
		},
	}
}

type testEnvSetter struct {
	env map[string]string
}

func (e *testEnvSetter) Setenv(key string, val string) error {
	if e.env == nil {
		e.env = make(map[string]string)
	}
	e.env[key] = val
	return nil
}
