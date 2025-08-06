package install

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	appinstall "github.com/replicatedhq/embedded-cluster/api/controllers/app/install"
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	apppreflightmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/preflight"
	appreleasemanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/release"
	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	apppreflightstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Test the getAppPreflightsStatus endpoint returns app preflights status correctly
func TestGetAppPreflightsStatus(t *testing.T) {
	apf := types.AppPreflights{
		Output: &types.PreflightsOutput{
			Pass: []types.PreflightsRecord{
				{
					Title:   "Some App Preflight",
					Message: "All good",
				},
			},
			Fail: []types.PreflightsRecord{
				{
					Title:   "Another App Preflight",
					Message: "Oh no!",
				},
			},
		},
		Titles: []string{
			"Some App Preflight",
			"Another App Preflight",
		},
		Status: types.Status{
			State:       types.StateFailed,
			Description: "An app preflight failed",
		},
	}

	// Create real app preflight manager with in-memory store
	appPreflightManager := apppreflightmanager.NewAppPreflightManager(
		apppreflightmanager.WithAppPreflightStore(
			apppreflightstore.NewMemoryStore(apppreflightstore.WithAppPreflight(apf)),
		),
	)

	// Create mock store with proper app config store
	mockStore := &store.MockStore{}
	mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

	// Create real app install controller
	appInstallController, err := appinstall.NewInstallController(
		appinstall.WithAppPreflightManager(appPreflightManager),
		appinstall.WithStateMachine(linuxinstall.NewStateMachine()),
		appinstall.WithStore(mockStore),
		appinstall.WithReleaseData(integration.DefaultReleaseData()),
	)
	require.NoError(t, err)

	// Create Linux install controller
	installController, err := linuxinstall.NewInstallController(
		linuxinstall.WithAppInstallController(appInstallController),
		linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance := integration.NewAPIWithReleaseData(t,
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
		req := httptest.NewRequest(http.MethodGet, "/linux/install/app-preflights/status", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusOK, rec.Code, "expected status ok, got %d with body %s", rec.Code, rec.Body.String())
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var status types.InstallAppPreflightsStatusResponse
		err = json.NewDecoder(rec.Body).Decode(&status)
		require.NoError(t, err)

		// Check the parsed response
		assert.Equal(t, apf.Titles, status.Titles)
		assert.Equal(t, apf.Output, status.Output)
		assert.Equal(t, apf.Status, status.Status)
	})

	// Test authorization error
	t.Run("Authorization error", func(t *testing.T) {
		// Create a request without authorization header
		req := httptest.NewRequest(http.MethodGet, "/linux/install/app-preflights/status", nil)
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	// Test controller error
	t.Run("Controller error", func(t *testing.T) {
		// Create a mock controller that returns an error
		mockController := &linuxinstall.MockController{}
		mockController.On("GetAppPreflightTitles", mock.Anything).Return([]string{}, assert.AnError)

		// Create the API with the mock controller
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(mockController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/linux/install/app-preflights/status", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusInternalServerError, rec.Code)

		// Verify the mock was called
		mockController.AssertExpectations(t)
	})
}

func TestPostRunAppPreflights(t *testing.T) {
	// Create a real runtime config with temp directory
	rc := runtimeconfig.New(nil)
	rc.SetDataDir(t.TempDir())

	t.Run("Success", func(t *testing.T) {
		// Mock preflight runner (external dependency)
		runner := &preflights.MockPreflightRunner{}

		// Mock the preflight runner expectations
		runner.On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).Return(&types.PreflightsOutput{
			Pass: []types.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
		}, "", nil)

		// Create real app preflight manager with mock runner
		appPreflightManager := apppreflightmanager.NewAppPreflightManager(
			apppreflightmanager.WithAppPreflightStore(
				apppreflightstore.NewMemoryStore(),
			),
			apppreflightmanager.WithPreflightRunner(runner),
		)

		// Create mock store with proper app config store
		mockStore := &store.MockStore{}
		mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

		// Create mock app release manager
		mockAppReleaseManager := &appreleasemanager.MockAppReleaseManager{}
		mockAppReleaseManager.On("ExtractAppPreflightSpec", mock.Anything, mock.Anything, mock.Anything).Return(&troubleshootv1beta2.PreflightSpec{
			Analyzers: []*troubleshootv1beta2.Analyze{
				{
					ClusterVersion: &troubleshootv1beta2.ClusterVersion{
						Outcomes: []*troubleshootv1beta2.Outcome{
							{
								Pass: &troubleshootv1beta2.SingleOutcome{
									Message: "Kubernetes version is supported",
								},
							},
						},
					},
				},
			},
		}, nil)

		// Create real app install controller with proper state machine
		appInstallController, err := appinstall.NewInstallController(
			appinstall.WithAppPreflightManager(appPreflightManager),
			appinstall.WithAppReleaseManager(mockAppReleaseManager),
			appinstall.WithStateMachine(linuxinstall.NewStateMachine(
				linuxinstall.WithCurrentState(states.StateInfrastructureInstallSucceeded), // App preflights can run from InfrastructureInstallSucceeded
			)),
			appinstall.WithStore(mockStore),
			appinstall.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create Linux install controller with runtime config
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(
				linuxinstall.WithCurrentState(states.StateInfrastructureInstallSucceeded),
			)),
			linuxinstall.WithAppInstallController(appInstallController),
			linuxinstall.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease: &release.ChannelRelease{
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.example.com",
						ProxyRegistryDomain: "some-proxy.example.com",
					},
				},
				AppConfig: &kotsv1beta1.Config{},
			}),
			linuxinstall.WithRuntimeConfig(rc),
		)
		require.NoError(t, err)

		// Create the API with runtime config in the API config
		apiInstance, err := api.New(types.APIConfig{
			Password: "password",
			LinuxConfig: types.LinuxConfig{
				RuntimeConfig: rc,
			},
			ReleaseData: integration.DefaultReleaseData(),
		},
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request (no body needed)
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app-preflights/run", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusOK, rec.Code, "expected status ok, got %d with body %s", rec.Code, rec.Body.String())
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body (should be the status response)
		var response types.InstallAppPreflightsStatusResponse
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify the mocks were called (note: runner runs asynchronously in goroutine)
		mockAppReleaseManager.AssertExpectations(t)
	})

	t.Run("Invalid state", func(t *testing.T) {
		// Create Linux install controller with wrong state
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(
				linuxinstall.WithCurrentState(states.StateNew), // Wrong state
			)),
			linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
			linuxinstall.WithRuntimeConfig(rc),
		)
		require.NoError(t, err)

		// Create the API with runtime config
		apiInstance, err := api.New(types.APIConfig{
			Password: "password",
			LinuxConfig: types.LinuxConfig{
				RuntimeConfig: rc,
			},
			ReleaseData: integration.DefaultReleaseData(),
		},
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app-preflights/run", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusConflict, rec.Code)

		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusConflict, apiError.StatusCode)
	})

	// Test authorization
	t.Run("Authorization error", func(t *testing.T) {
		// Create a basic install controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create the API with runtime config
		apiInstance, err := api.New(types.APIConfig{
			Password: "password",
			LinuxConfig: types.LinuxConfig{
				RuntimeConfig: rc,
			},
			ReleaseData: integration.DefaultReleaseData(),
		},
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request without authorization header
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app-preflights/run", nil)
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
