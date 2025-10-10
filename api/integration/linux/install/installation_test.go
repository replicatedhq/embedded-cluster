package install

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	apiclient "github.com/replicatedhq/embedded-cluster/api/client"
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	linuxinstallationmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestLinuxConfigureInstallation(t *testing.T) {
	// Test scenarios
	testCases := []struct {
		name                  string
		mockHostUtils         *hostutils.MockHostUtils
		mockNetUtils          *utils.MockNetUtils
		token                 string
		config                types.LinuxInstallationConfig
		expectedConfig        types.LinuxInstallationConfig
		expectedStatus        *types.Status
		expectedStatusCode    int
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
			mockNetUtils: func() *utils.MockNetUtils {
				mockNetUtils := &utils.MockNetUtils{}
				// We need to mock multiple calls because DetermineBestNetworkInterface is called during:
				// 1. Configuration process (SetConfigDefaults)
				// 2. GetConfig call which internally calls SetConfigDefaults again
				// 3. GetInstallationConfig after the request to verify the result
				mockNetUtils.On("DetermineBestNetworkInterface").Return("eth0", nil).Times(3)
				return mockNetUtils
			}(),
			token: "TOKEN",
			config: types.LinuxInstallationConfig{
				DataDirectory:           "/tmp/data",
				AdminConsolePort:        8000,
				LocalArtifactMirrorPort: 8081,
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
			},
			expectedConfig: types.LinuxInstallationConfig{
				DataDirectory:           "/tmp/data",
				AdminConsolePort:        8000,
				LocalArtifactMirrorPort: 8081,
				GlobalCIDR:              "10.0.0.0/16",
				PodCIDR:                 "10.0.0.0/17",
				ServiceCIDR:             "10.0.128.0/17",
				NetworkInterface:        "eth0",
			},
			expectedStatus: &types.Status{
				State:       types.StateSucceeded,
				Description: "Installation configured",
			},
			expectedStatusCode: http.StatusOK,
			expectedError:      false,
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
				// We need to mock multiple calls because DetermineBestNetworkInterface is called during:
				// 1. Configuration process (SetConfigDefaults)
				// 2. GetConfig call which internally calls SetConfigDefaults again
				// 3. GetInstallationConfig after the request to verify the result
				mockNetUtils.On("DetermineBestNetworkInterface").Return("eth0", nil).Times(3)
				mockNetUtils.On("FirstValidIPNet", "eth0").Return(&net.IPNet{IP: net.ParseIP("192.168.17.12"), Mask: net.CIDRMask(24, 32)}, nil)
				return mockNetUtils
			}(),
			token: "TOKEN",
			config: types.LinuxInstallationConfig{
				DataDirectory:           "/tmp/data",
				AdminConsolePort:        8000,
				LocalArtifactMirrorPort: 8081,
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
				HTTPProxy:               "http://proxy.example.com",
				HTTPSProxy:              "https://proxy.example.com",
				NoProxy:                 "somecompany.internal,192.168.17.0/24",
			},
			expectedConfig: types.LinuxInstallationConfig{
				DataDirectory:           "/tmp/data",
				AdminConsolePort:        8000,
				LocalArtifactMirrorPort: 8081,
				GlobalCIDR:              "10.0.0.0/16",
				PodCIDR:                 "10.0.0.0/17",
				ServiceCIDR:             "10.0.128.0/17",
				NetworkInterface:        "eth0",
				HTTPProxy:               "http://proxy.example.com",
				HTTPSProxy:              "https://proxy.example.com",
				NoProxy:                 "somecompany.internal,192.168.17.0/24",
			},
			expectedStatus: &types.Status{
				State:       types.StateSucceeded,
				Description: "Installation configured",
			},
			expectedStatusCode: http.StatusOK,
			expectedError:      false,
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
			name: "Config with default values",
			mockHostUtils: func() *hostutils.MockHostUtils {
				mockHostUtils := &hostutils.MockHostUtils{}
				mockHostUtils.On("ConfigureHost", mock.Anything,
					mock.MatchedBy(func(rc runtimeconfig.RuntimeConfig) bool {
						return rc.EmbeddedClusterHomeDirectory() == "/tmp/data" &&
							rc.AdminConsolePort() == 30000 &&
							rc.LocalArtifactMirrorPort() == 50000 &&
							rc.NetworkInterface() == "eth0" &&
							rc.GlobalCIDR() == "10.244.0.0/16" &&
							rc.ServiceCIDR() == "10.244.128.0/17" &&
							rc.PodCIDR() == "10.244.0.0/17" &&
							rc.NodePortRange() == "80-32767" &&
							rc.ProxySpec() == nil
					}),
					mock.Anything,
				).Return(nil).Once()
				return mockHostUtils
			}(),
			mockNetUtils: func() *utils.MockNetUtils {
				mockNetUtils := &utils.MockNetUtils{}
				// We need to mock multiple calls because DetermineBestNetworkInterface is called during:
				// 1. Configuration process (SetConfigDefaults)
				// 2. GetConfig call which internally calls SetConfigDefaults again
				// 3. GetInstallationConfig after the request to verify the result
				mockNetUtils.On("DetermineBestNetworkInterface").Return("eth0", nil).Times(3)
				return mockNetUtils
			}(),
			token: "TOKEN",
			config: types.LinuxInstallationConfig{
				DataDirectory: "/tmp/data", // We only provide the data directory
			},
			expectedConfig: types.LinuxInstallationConfig{
				DataDirectory:           "/tmp/data",
				AdminConsolePort:        30000,
				LocalArtifactMirrorPort: 50000,
				GlobalCIDR:              "10.244.0.0/16",
				PodCIDR:                 "10.244.0.0/17",
				ServiceCIDR:             "10.244.128.0/17",
				NetworkInterface:        "eth0",
			},
			expectedStatus: &types.Status{
				State:       types.StateSucceeded,
				Description: "Installation configured",
			},
			expectedStatusCode: http.StatusOK,
			expectedError:      false,
			validateRuntimeConfig: func(t *testing.T, rc runtimeconfig.RuntimeConfig) {
				assert.Equal(t, "/tmp/data", rc.EmbeddedClusterHomeDirectory())
				assert.Equal(t, 30000, rc.AdminConsolePort())
				assert.Equal(t, 50000, rc.LocalArtifactMirrorPort())
				assert.Equal(t, ecv1beta1.NetworkSpec{
					NetworkInterface: "eth0",
					GlobalCIDR:       "10.244.0.0/16",
					PodCIDR:          "10.244.0.0/17",
					ServiceCIDR:      "10.244.128.0/17",
					NodePortRange:    "80-32767",
				}, rc.Get().Network)
				assert.Equal(t, (*ecv1beta1.ProxySpec)(nil), rc.ProxySpec())
			},
		},
		{
			name:          "Invalid config - port conflict",
			mockHostUtils: &hostutils.MockHostUtils{},
			mockNetUtils: func() *utils.MockNetUtils {
				mockNetUtils := &utils.MockNetUtils{}
				mockNetUtils.On("DetermineBestNetworkInterface").Return("eth0", nil).Once()
				return mockNetUtils
			}(),
			token: "TOKEN",
			config: types.LinuxInstallationConfig{
				DataDirectory:           "/tmp/data",
				AdminConsolePort:        8080,
				LocalArtifactMirrorPort: 8080, // Same as AdminConsolePort
				GlobalCIDR:              "10.0.0.0/16",
				NetworkInterface:        "eth0",
			},
			expectedConfig: types.LinuxInstallationConfig{},
			expectedStatus: &types.Status{
				State:       types.StateFailed,
				Description: "validate: field errors: adminConsolePort and localArtifactMirrorPort cannot be equal",
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedError:      true,
		},
		{
			name:               "Unauthorized",
			mockHostUtils:      &hostutils.MockHostUtils{},
			mockNetUtils:       &utils.MockNetUtils{},
			token:              "NOT_A_TOKEN",
			config:             types.LinuxInstallationConfig{},
			expectedStatusCode: http.StatusUnauthorized,
			expectedError:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a runtime config
			rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
			// Set the expected data directory to match the test case
			if tc.config.DataDirectory != "" {
				rc.SetDataDir(tc.config.DataDirectory)
			}

			// Create an install controller with the config manager
			installController, err := linuxinstall.NewInstallController(
				linuxinstall.WithRuntimeConfig(rc),
				linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(states.StateApplicationConfigured))),
				linuxinstall.WithHostUtils(tc.mockHostUtils),
				linuxinstall.WithNetUtils(tc.mockNetUtils),
				linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
			)
			require.NoError(t, err)

			// Create the API with the install controller
			apiInstance := integration.NewAPIWithReleaseData(t, types.ModeInstall, types.TargetLinux,
				api.WithLinuxInstallController(installController),
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
			req := httptest.NewRequest(http.MethodPost, "/linux/install/installation/configure", bytes.NewReader(configJSON))
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

				// Verify that the status is not pending. We cannot check for an end state here because the host config is async
				// so the state might have moved from running to a final state before we get the response.
				assert.NotEqual(t, types.StatePending, status.State)
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
			}

			// Verify host configuration was performed for successful tests
			tc.mockHostUtils.AssertExpectations(t)
			tc.mockNetUtils.AssertExpectations(t)

			if tc.validateRuntimeConfig != nil {
				tc.validateRuntimeConfig(t, rc)
			}
		})
	}
}

// Test that config validation errors are properly returned
func TestLinuxConfigureInstallationValidation(t *testing.T) {
	rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
	rc.SetDataDir(t.TempDir())

	// Create an install controller with the config manager
	installController, err := linuxinstall.NewInstallController(
		linuxinstall.WithRuntimeConfig(rc),
		linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(states.StateApplicationConfigured))),
		linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance := integration.NewAPIWithReleaseData(t, types.ModeInstall, types.TargetLinux,
		api.WithLinuxInstallController(installController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test a validation error case with mixed CIDR settings
	config := types.LinuxInstallationConfig{
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
	req := httptest.NewRequest(http.MethodPost, "/linux/install/installation/configure", bytes.NewReader(configJSON))
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
func TestLinuxConfigureInstallationBadRequest(t *testing.T) {
	rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
	rc.SetDataDir(t.TempDir())

	// Create an install controller with the config manager
	installController, err := linuxinstall.NewInstallController(
		linuxinstall.WithRuntimeConfig(rc),
		linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(states.StateHostConfigured))),
		linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
	)
	require.NoError(t, err)

	apiInstance := integration.NewAPIWithReleaseData(t, types.ModeInstall, types.TargetLinux,
		api.WithLinuxInstallController(installController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)

	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Create a request with invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/linux/install/installation/configure",
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
func TestLinuxConfigureInstallationControllerError(t *testing.T) {
	// Create a mock controller that returns an error
	mockController := &linuxinstall.MockController{}
	mockController.On("ConfigureInstallation", mock.Anything, mock.Anything).Return(assert.AnError)

	// Create the API with the mock controller
	apiInstance := integration.NewAPIWithReleaseData(t, types.ModeInstall, types.TargetLinux,
		api.WithLinuxInstallController(mockController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)

	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Create a valid config request
	config := types.LinuxInstallationConfig{
		DataDirectory:    "/tmp/data",
		AdminConsolePort: 8000,
	}
	configJSON, err := json.Marshal(config)
	require.NoError(t, err)

	// Create a request
	req := httptest.NewRequest(http.MethodPost, "/linux/install/installation/configure", bytes.NewReader(configJSON))
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

// Test the getInstallationConfig endpoint returns installation data correctly
func TestLinuxGetInstallationConfig(t *testing.T) {
	rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
	tempDir := t.TempDir()
	rc.SetDataDir(tempDir)

	netUtils := &utils.MockNetUtils{}
	netUtils.On("ListValidNetworkInterfaces").Return([]string{"eth0", "eth1"}, nil).Once()
	netUtils.On("DetermineBestNetworkInterface").Return("eth0", nil).Times(2)

	// Create a config manager
	installationManager := linuxinstallationmanager.NewInstallationManager(
		linuxinstallationmanager.WithNetUtils(netUtils),
	)

	// Create an install controller with the config manager
	installController, err := linuxinstall.NewInstallController(
		linuxinstall.WithRuntimeConfig(rc),
		linuxinstall.WithInstallationManager(installationManager),
		linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
	)
	require.NoError(t, err)

	// Set some initial config
	initialConfig := types.LinuxInstallationConfig{
		DataDirectory:           "/some/directory",
		AdminConsolePort:        8080,
		LocalArtifactMirrorPort: 8081,
		GlobalCIDR:              "10.0.0.0/16",
		NetworkInterface:        "eth0",
	}
	err = installationManager.SetConfigValues(initialConfig)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance := integration.NewAPIWithReleaseData(t, types.ModeInstall, types.TargetLinux,
		api.WithLinuxInstallController(installController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test successful get
	t.Run("Success", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/linux/install/installation/config", nil)
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var configResponse types.LinuxInstallationConfigResponse
		err = json.NewDecoder(rec.Body).Decode(&configResponse)
		require.NoError(t, err)

		// Verify the installation data matches what we expect
		assert.Equal(t, initialConfig.DataDirectory, configResponse.Values.DataDirectory)
		assert.Equal(t, initialConfig.AdminConsolePort, configResponse.Values.AdminConsolePort)
		assert.Equal(t, initialConfig.LocalArtifactMirrorPort, configResponse.Values.LocalArtifactMirrorPort)
		assert.Equal(t, initialConfig.GlobalCIDR, configResponse.Values.GlobalCIDR)
		assert.Equal(t, initialConfig.NetworkInterface, configResponse.Values.NetworkInterface)

		// Verify that defaults are properly populated
		assert.Equal(t, 30000, configResponse.Defaults.AdminConsolePort)
		assert.Equal(t, rc.EmbeddedClusterHomeDirectory(), configResponse.Defaults.DataDirectory)
		assert.Equal(t, 50000, configResponse.Defaults.LocalArtifactMirrorPort)
		assert.Equal(t, "10.244.0.0/16", configResponse.Defaults.GlobalCIDR)
		assert.Equal(t, "eth0", configResponse.Defaults.NetworkInterface)
	})

	// Test get with default/empty configuration
	t.Run("Default configuration", func(t *testing.T) {
		netUtils := &utils.MockNetUtils{}
		netUtils.On("ListValidNetworkInterfaces").Return([]string{"eth0", "eth1"}, nil).Once()
		netUtils.On("DetermineBestNetworkInterface").Return("eth0", nil).Times(2)

		rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
		defaultTempDir := t.TempDir()
		rc.SetDataDir(defaultTempDir)

		// Create a fresh config manager without writing anything
		emptyInstallationManager := linuxinstallationmanager.NewInstallationManager(
			linuxinstallationmanager.WithNetUtils(netUtils),
		)

		// Create an install controller with the empty config manager
		emptyInstallController, err := linuxinstall.NewInstallController(
			linuxinstall.WithRuntimeConfig(rc),
			linuxinstall.WithInstallationManager(emptyInstallationManager),
			linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		emptyAPI := integration.NewAPIWithReleaseData(t, types.ModeInstall, types.TargetLinux,
			api.WithLinuxInstallController(emptyInstallController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		// Create a router and register the API routes
		emptyRouter := mux.NewRouter()
		emptyAPI.RegisterRoutes(emptyRouter)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/linux/install/installation/config", nil)
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		emptyRouter.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var configResponse types.LinuxInstallationConfigResponse
		err = json.NewDecoder(rec.Body).Decode(&configResponse)
		require.NoError(t, err)

		// Verify the installation data contains empty values
		assert.Equal(t, "", configResponse.Values.DataDirectory)
		assert.Equal(t, 0, configResponse.Values.AdminConsolePort)
		assert.Equal(t, 0, configResponse.Values.LocalArtifactMirrorPort)
		assert.Equal(t, "", configResponse.Values.GlobalCIDR)
		assert.Equal(t, "", configResponse.Values.NetworkInterface)

		// Verify that defaults are properly populated
		assert.Equal(t, 30000, configResponse.Defaults.AdminConsolePort)
		assert.Equal(t, rc.EmbeddedClusterHomeDirectory(), configResponse.Defaults.DataDirectory)
		assert.Equal(t, 50000, configResponse.Defaults.LocalArtifactMirrorPort)
		assert.Equal(t, "10.244.0.0/16", configResponse.Defaults.GlobalCIDR)
		assert.Equal(t, "eth0", configResponse.Defaults.NetworkInterface)

		// Verify proxy defaults (should be empty when no env vars set)
		assert.Equal(t, "", configResponse.Defaults.HTTPProxy)
		assert.Equal(t, "", configResponse.Defaults.HTTPSProxy)
		assert.Equal(t, "", configResponse.Defaults.NoProxy)
	})

	// Test authorization
	t.Run("Authorization error", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/linux/install/installation/config", nil)
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
		mockController := &linuxinstall.MockController{}
		mockController.On("GetInstallationConfig", mock.Anything).Return(types.LinuxInstallationConfigResponse{}, assert.AnError)

		// Create the API with the mock controller
		apiInstance := integration.NewAPIWithReleaseData(t, types.ModeInstall, types.TargetLinux,
			api.WithLinuxInstallController(mockController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/linux/install/installation/config", nil)
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

// TestLinuxInstallationConfigWithAPIClient tests the installation configuration endpoints using the API client
func TestLinuxInstallationConfigWithAPIClient(t *testing.T) {
	// Create a runtimeconfig to be used in the install process
	rc := runtimeconfig.New(nil, runtimeconfig.WithEnvSetter(&testEnvSetter{}))
	tempDir := t.TempDir()
	rc.SetDataDir(tempDir)

	// Create a mock hostutils
	mockHostUtils := &hostutils.MockHostUtils{}
	mockHostUtils.On("ConfigureHost", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Create a config manager
	installationManager := linuxinstallationmanager.NewInstallationManager(
		linuxinstallationmanager.WithHostUtils(mockHostUtils),
	)

	// Create an install controller with the config manager
	installController, err := linuxinstall.NewInstallController(
		linuxinstall.WithRuntimeConfig(rc),
		linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(states.StateApplicationConfigured))),
		linuxinstall.WithInstallationManager(installationManager),
		linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
	)
	require.NoError(t, err)

	// Set some initial config
	initialConfig := types.LinuxInstallationConfig{
		DataDirectory:           rc.EmbeddedClusterHomeDirectory(),
		AdminConsolePort:        9080,
		LocalArtifactMirrorPort: 9081,
		GlobalCIDR:              "192.168.0.0/16",
		NetworkInterface:        "eth1",
	}
	err = installationManager.SetConfigValues(initialConfig)
	require.NoError(t, err)

	// Set some initial status
	initialStatus := types.Status{
		State:       types.StatePending,
		Description: "Installation pending",
	}
	err = installationManager.SetStatus(initialStatus)
	require.NoError(t, err)

	// Create the API with controllers
	apiInstance := integration.NewAPIWithReleaseData(t, types.ModeInstall, types.TargetLinux,
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLinuxInstallController(installController),
		api.WithLogger(logger.NewDiscardLogger()),
	)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router.PathPrefix("/api").Subrouter())

	// Create a test server using the router
	server := httptest.NewServer(router)
	defer server.Close()

	// Create client with the predefined token
	c := apiclient.New(server.URL, apiclient.WithToken("TOKEN"))
	require.NoError(t, err, "API client login should succeed")

	// Test GetLinuxInstallationConfig
	t.Run("GetLinuxInstallationConfig", func(t *testing.T) {
		configResponse, err := c.GetLinuxInstallationConfig()
		require.NoError(t, err, "GetInstallationConfig should succeed")

		// Verify values
		// Note: DataDirectory gets overridden with the temp directory from RuntimeConfig
		assert.Equal(t, rc.EmbeddedClusterHomeDirectory(), configResponse.Values.DataDirectory)
		assert.Equal(t, 9080, configResponse.Values.AdminConsolePort)
		assert.Equal(t, 9081, configResponse.Values.LocalArtifactMirrorPort)
		assert.Equal(t, "192.168.0.0/16", configResponse.Values.GlobalCIDR)
		assert.Equal(t, "eth1", configResponse.Values.NetworkInterface)

		// Verify defaults are present
		assert.Equal(t, 30000, configResponse.Defaults.AdminConsolePort)
		assert.Equal(t, rc.EmbeddedClusterHomeDirectory(), configResponse.Defaults.DataDirectory)
		assert.Equal(t, 50000, configResponse.Defaults.LocalArtifactMirrorPort)
		assert.Equal(t, "10.244.0.0/16", configResponse.Defaults.GlobalCIDR)
		assert.NotEmpty(t, configResponse.Defaults.NetworkInterface)
	})

	// Test GetLinuxInstallationStatus
	t.Run("GetLinuxInstallationStatus", func(t *testing.T) {
		status, err := c.GetLinuxInstallationStatus()
		require.NoError(t, err, "GetLinuxInstallationStatus should succeed")
		assert.Equal(t, types.StatePending, status.State)
		assert.Equal(t, "Installation pending", status.Description)
	})

	// Test ConfigureLinuxInstallation
	t.Run("ConfigureLinuxInstallation", func(t *testing.T) {
		// Create a valid config
		config := types.LinuxInstallationConfig{
			DataDirectory:           "/tmp/new-dir",
			AdminConsolePort:        8000,
			LocalArtifactMirrorPort: 8081,
			GlobalCIDR:              "10.0.0.0/16",
			NetworkInterface:        "eth0",
		}

		// Update runtime config to match expected data directory for this test
		rc.SetDataDir(config.DataDirectory)

		// Configure the installation using the client
		_, err = c.ConfigureLinuxInstallation(config)
		require.NoError(t, err, "ConfigureLinuxInstallation should succeed with valid config")

		// Verify the status was set correctly
		var installStatus types.Status
		if !assert.Eventually(t, func() bool {
			installStatus, err = c.GetLinuxInstallationStatus()
			require.NoError(t, err, "GetLinuxInstallationStatus should succeed")
			return installStatus.State == types.StateSucceeded
		}, 1*time.Second, 100*time.Millisecond) {
			require.Equal(t, types.StateSucceeded, installStatus.State,
				"Installation not succeeded with state %s and description %s", installStatus.State, installStatus.Description)
		}

		// Get the config to verify it persisted
		newConfigResponse, err := c.GetLinuxInstallationConfig()
		require.NoError(t, err, "GetLinuxInstallationConfig should succeed after setting config")
		assert.Equal(t, rc.EmbeddedClusterHomeDirectory(), newConfigResponse.Values.DataDirectory)
		assert.Equal(t, config.AdminConsolePort, newConfigResponse.Values.AdminConsolePort)
		assert.Equal(t, config.NetworkInterface, newConfigResponse.Values.NetworkInterface)

		// Verify host configuration was performed
		mockHostUtils.AssertExpectations(t)
	})

	// Test ConfigureLinuxInstallation validation error
	t.Run("ConfigureLinuxInstallation validation error", func(t *testing.T) {
		// Create an invalid config (port conflict)
		config := types.LinuxInstallationConfig{
			DataDirectory:           "/tmp/new-dir",
			AdminConsolePort:        8080,
			LocalArtifactMirrorPort: 8080, // Same as AdminConsolePort
			GlobalCIDR:              "10.0.0.0/16",
			NetworkInterface:        "eth0",
		}

		// Configure the installation using the client
		_, err := c.ConfigureLinuxInstallation(config)
		require.Error(t, err, "ConfigureLinuxInstallation should fail with invalid config")

		// Verify the error is of type APIError
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
		// Error message should contain the same port conflict message for both fields
		assert.Equal(t, 2, strings.Count(apiErr.Error(), "adminConsolePort and localArtifactMirrorPort cannot be equal"))
	})
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
