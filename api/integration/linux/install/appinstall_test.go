package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	appinstall "github.com/replicatedhq/embedded-cluster/api/controllers/app/install"
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	appinstallmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/install"
	appreleasemanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/release"
	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	appinstallstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestGetAppInstallStatus tests the GET /linux/install/app/status endpoint
func TestGetAppInstallStatus(t *testing.T) {
	// Create mock helm chart archive
	mockChartArchive := []byte("mock-helm-chart-archive-data")

	// Create test release data with helm chart archives
	releaseData := &release.ReleaseData{
		HelmChartArchives:     [][]byte{mockChartArchive},
		EmbeddedClusterConfig: &ecv1beta1.Config{},
		ChannelRelease: &release.ChannelRelease{
			DefaultDomains: release.Domains{
				ReplicatedAppDomain: "replicated.example.com",
				ProxyRegistryDomain: "some-proxy.example.com",
			},
		},
		AppConfig: &kotsv1beta1.Config{},
	}

	t.Run("Success", func(t *testing.T) {
		// Create app install status
		appInstallStatus := types.AppInstall{
			Status: types.Status{
				State:       types.StateRunning,
				Description: "Installing application",
			},
			Logs: "Installation in progress...",
		}

		// Create real app install manager with in-memory store
		appInstallManager, err := appinstallmanager.NewAppInstallManager(
			appinstallmanager.WithAppInstallStore(
				appinstallstore.NewMemoryStore(appinstallstore.WithAppInstall(appInstallStatus)),
			),
		)
		require.NoError(t, err)

		// Create mock store
		mockStore := &store.MockStore{}

		// Create real app install controller
		appInstallController, err := appinstall.NewInstallController(
			appinstall.WithAppInstallManager(appInstallManager),
			appinstall.WithStateMachine(linuxinstall.NewStateMachine()),
			appinstall.WithStore(mockStore),
			appinstall.WithReleaseData(releaseData),
		)
		require.NoError(t, err)

		// Create Linux install controller with runtime config
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine()),
			linuxinstall.WithAppInstallController(appInstallController),
			linuxinstall.WithReleaseData(releaseData),
			linuxinstall.WithRuntimeConfig(runtimeconfig.New(nil)),
		)
		require.NoError(t, err)

		// Create the API
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		// Create a new router and register API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/linux/install/app/status", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusOK, rec.Code, "expected status ok, got %d with body %s", rec.Code, rec.Body.String())
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var response types.AppInstall
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify the response
		assert.Equal(t, appInstallStatus.Status.State, response.Status.State)
		assert.Equal(t, appInstallStatus.Status.Description, response.Status.Description)
		assert.Equal(t, appInstallStatus.Logs, response.Logs)
	})

	t.Run("Authorization error", func(t *testing.T) {
		// Create simple Linux install controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithReleaseData(releaseData),
		)
		require.NoError(t, err)

		// Create the API
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		// Create a new router and register API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request without authorization
		req := httptest.NewRequest(http.MethodGet, "/linux/install/app/status", nil)
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Controller error", func(t *testing.T) {
		// Create mock controller that returns an error
		mockController := &linuxinstall.MockController{}
		mockController.On("GetAppInstallStatus", mock.Anything).Return(types.AppInstall{}, assert.AnError)

		// Create the API with mock controller
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(mockController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		// Create a new router and register API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/linux/install/app/status", nil)
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

// TestPostInstallApp tests the POST /linux/install/app/install endpoint
func TestPostInstallApp(t *testing.T) {
	// Create test release data
	releaseData := &release.ReleaseData{
		EmbeddedClusterConfig: &ecv1beta1.Config{},
		ChannelRelease: &release.ChannelRelease{
			DefaultDomains: release.Domains{
				ReplicatedAppDomain: "replicated.example.com",
				ProxyRegistryDomain: "some-proxy.example.com",
			},
		},
		AppConfig: &kotsv1beta1.Config{},
	}

	t.Run("Success", func(t *testing.T) {
		// Create a real runtime config with temp directory
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		// Create mock metrics reporter - this is the key test for reporting handlers
		mockReporter := &metrics.MockReporter{}
		mockReporter.On("ReportInstallationSucceeded", mock.Anything)

		// Create mock app and kots config values
		testAppConfigValues := types.AppConfigValues{
			"service": {
				Value: "ClusterIP",
			},
		}
		testKotsConfigValues := kotsv1beta1.ConfigValues{
			Spec: kotsv1beta1.ConfigValuesSpec{
				Values: map[string]kotsv1beta1.ConfigValue{
					"enable_ingress": {
						Value: "1",
					},
				},
			},
		}

		// Create app config manager with mock store
		mockAppConfigManager := &appconfig.MockAppConfigManager{}
		mockAppConfigManager.On("GetConfigValues").Return(testAppConfigValues, nil)
		mockAppConfigManager.On("GetKotsadmConfigValues").Return(testKotsConfigValues, nil)

		// Create mock app release manager that returns installable charts
		mockAppReleaseManager := &appreleasemanager.MockAppReleaseManager{}
		testInstallableCharts := []types.InstallableHelmChart{
			{
				Archive: []byte("mock-helm-chart-archive-data"),
				Values: map[string]any{
					"service": map[string]any{
						"port": 80,
					},
				},
				CR: &kotsv1beta2.HelmChart{
					Spec: kotsv1beta2.HelmChartSpec{
						ReleaseName: "test-app",
						Namespace:   "default",
					},
				},
			},
		}
		mockAppReleaseManager.On("ExtractInstallableHelmCharts", mock.Anything, testAppConfigValues, mock.AnythingOfType("*v1beta1.ProxySpec")).Return(testInstallableCharts, nil)

		// Create mock app install manager that succeeds
		mockAppInstallManager := &appinstallmanager.MockAppInstallManager{}
		mockAppInstallManager.On("Install", mock.Anything, testInstallableCharts, testKotsConfigValues).Return(nil)
		mockAppInstallManager.On("GetStatus").Return(types.AppInstall{
			Status: types.Status{
				State:       types.StateRunning,
				Description: "Installing application",
			},
		}, nil)

		// Create state machine starting from AppPreflightsSucceeded (valid state for app install)
		stateMachine := linuxinstall.NewStateMachine(
			linuxinstall.WithCurrentState(states.StateAppPreflightsSucceeded),
		)

		// Create real app install controller with mock managers
		appInstallController, err := appinstall.NewInstallController(
			appinstall.WithAppInstallManager(mockAppInstallManager),
			appinstall.WithAppReleaseManager(mockAppReleaseManager),
			appinstall.WithAppConfigManager(mockAppConfigManager),
			appinstall.WithStateMachine(stateMachine),
			appinstall.WithStore(&store.MockStore{}),
			appinstall.WithReleaseData(releaseData),
		)
		require.NoError(t, err)

		// Create Linux install controller with mock metrics reporter and embedded app controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(stateMachine),
			linuxinstall.WithAppInstallController(appInstallController),
			linuxinstall.WithMetricsReporter(mockReporter),
			linuxinstall.WithReleaseData(releaseData),
			linuxinstall.WithRuntimeConfig(rc),
		)
		require.NoError(t, err)

		// Create the API
		cfg := types.APIConfig{
			Password:    "password",
			ReleaseData: releaseData,
			LinuxConfig: types.LinuxConfig{
				RuntimeConfig: rc,
			},
		}
		apiInstance, err := api.New(cfg,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a new router and register API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/install", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusOK, rec.Code, "expected status ok, got %d with body %s", rec.Code, rec.Body.String())

		// Wait for the state machine to transition to Succeeded
		assert.Eventually(t, func() bool {
			return stateMachine.CurrentState() == states.StateSucceeded
		}, 10*time.Second, 100*time.Millisecond, "state should transition to Succeeded")

		// Verify that ReportInstallationSucceeded was called
		mockReporter.AssertExpectations(t)
		mockAppConfigManager.AssertExpectations(t)
		mockAppReleaseManager.AssertExpectations(t)
		mockAppInstallManager.AssertExpectations(t)
	})

	t.Run("Invalid state transition", func(t *testing.T) {
		// Create a real runtime config with temp directory
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		// Create state machine starting from invalid state (infrastructure installing)
		stateMachine := linuxinstall.NewStateMachine(
			linuxinstall.WithCurrentState(states.StateInfrastructureInstalling),
		)

		// Create simple app install controller
		mockStore := &store.MockStore{}
		appInstallController, err := appinstall.NewInstallController(
			appinstall.WithStateMachine(stateMachine),
			appinstall.WithStore(mockStore),
			appinstall.WithReleaseData(releaseData),
		)
		require.NoError(t, err)

		// Create Linux install controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(stateMachine),
			linuxinstall.WithAppInstallController(appInstallController),
			linuxinstall.WithReleaseData(releaseData),
		)
		require.NoError(t, err)

		// Create the API
		cfg := types.APIConfig{
			Password:    "password",
			ReleaseData: releaseData,
			LinuxConfig: types.LinuxConfig{
				RuntimeConfig: rc,
			},
		}
		apiInstance, err := api.New(cfg,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a new router and register API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/install", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response - should fail with conflict
		require.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("App installation failure", func(t *testing.T) {
		// Create a real runtime config with temp directory
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		// Create mock metrics reporter expecting failure report
		mockReporter := &metrics.MockReporter{}
		mockReporter.On("ReportInstallationFailed", mock.Anything, mock.MatchedBy(func(err error) bool {
			return err.Error() == "install app: installation failed"
		}))

		// Create mock app install manager that fails
		mockAppInstallManager := &appinstallmanager.MockAppInstallManager{}
		mockAppInstallManager.On("Install", mock.Anything, mock.Anything, mock.Anything).Return(errors.New("installation failed"))
		mockAppInstallManager.On("GetStatus").Return(types.AppInstall{
			Status: types.Status{
				State:       types.StateFailed,
				Description: "Failed to install application",
			},
		}, nil)

		// Create mock store that returns failure status from app install store
		mockStore := &store.MockStore{}
		mockStore.AppConfigMockStore.On("GetKotsadmConfigValues").Return(kotsv1beta1.ConfigValues{}, nil)
		mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
		mockStore.AppInstallMockStore.On("GetStatus").Return(types.Status{
			State:       types.StateFailed,
			Description: "install app: installation failed",
		}, nil)

		// Create state machine starting from AppPreflightsSucceeded (valid state for app install)
		stateMachine := linuxinstall.NewStateMachine(
			linuxinstall.WithCurrentState(states.StateAppPreflightsSucceeded),
		)

		// Create real app install controller with mock manager
		appInstallController, err := appinstall.NewInstallController(
			appinstall.WithAppInstallManager(mockAppInstallManager),
			appinstall.WithStateMachine(stateMachine),
			appinstall.WithStore(mockStore),
			appinstall.WithReleaseData(releaseData),
		)
		require.NoError(t, err)

		// Create Linux install controller with mock metrics reporter and embedded app controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(stateMachine),
			linuxinstall.WithAppInstallController(appInstallController),
			linuxinstall.WithMetricsReporter(mockReporter),
			linuxinstall.WithStore(mockStore),
			linuxinstall.WithReleaseData(releaseData),
			linuxinstall.WithRuntimeConfig(rc),
		)
		require.NoError(t, err)

		// Create the API
		cfg := types.APIConfig{
			Password:    "password",
			ReleaseData: releaseData,
			LinuxConfig: types.LinuxConfig{
				RuntimeConfig: rc,
			},
		}
		apiInstance, err := api.New(cfg,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a new router and register API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/install", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusOK, rec.Code, "expected status ok, got %d with body %s", rec.Code, rec.Body.String())

		// Wait for the state machine to transition to AppInstallFailed
		assert.Eventually(t, func() bool {
			return stateMachine.CurrentState() == states.StateAppInstallFailed
		}, 10*time.Second, 100*time.Millisecond, "state should transition to AppInstallFailed")

		// Verify that ReportInstallationFailed was called
		mockReporter.AssertExpectations(t)
		mockAppInstallManager.AssertExpectations(t)
		mockStore.AppInstallMockStore.AssertExpectations(t)
	})

	t.Run("Authorization error", func(t *testing.T) {
		// Create a real runtime config with temp directory
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		// Create simple Linux install controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithReleaseData(releaseData),
		)
		require.NoError(t, err)

		// Create the API
		cfg := types.APIConfig{
			Password:    "password",
			ReleaseData: releaseData,
			LinuxConfig: types.LinuxConfig{
				RuntimeConfig: rc,
			},
		}
		apiInstance, err := api.New(cfg,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a new router and register API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request without authorization
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/install", bytes.NewReader([]byte(`{}`)))
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	// Test app preflight bypass - should succeed when ignoreAppPreflights is true
	t.Run("App preflight bypass with failed preflights - ignored", func(t *testing.T) {
		// Create a real runtime config with temp directory
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		// Create mock store
		mockStore := &store.MockStore{}
		mockStore.AppConfigMockStore.On("GetKotsadmConfigValues").Return(kotsv1beta1.ConfigValues{}, nil)
		mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

		// Create mock app install manager that succeeds
		mockAppInstallManager := &appinstallmanager.MockAppInstallManager{}
		mockAppInstallManager.On("Install", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockAppInstallManager.On("GetStatus").Return(types.AppInstall{
			Status: types.Status{
				State:       types.StateRunning,
				Description: "Installing application",
			},
		}, nil)

		// Create state machine starting from AppPreflightsFailed
		stateMachine := linuxinstall.NewStateMachine(
			linuxinstall.WithCurrentState(states.StateAppPreflightsFailed),
		)

		// Create real app install controller
		appInstallController, err := appinstall.NewInstallController(
			appinstall.WithAppInstallManager(mockAppInstallManager),
			appinstall.WithStateMachine(stateMachine),
			appinstall.WithStore(mockStore),
			appinstall.WithReleaseData(releaseData),
		)
		require.NoError(t, err)

		// Create Linux install controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(stateMachine),
			linuxinstall.WithAppInstallController(appInstallController),
			linuxinstall.WithReleaseData(releaseData),
		)
		require.NoError(t, err)

		// Create the API
		cfg := types.APIConfig{
			Password:    "password",
			ReleaseData: releaseData,
			LinuxConfig: types.LinuxConfig{
				RuntimeConfig: rc,
			},
		}
		apiInstance, err := api.New(cfg,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request with ignoreAppPreflights=true
		requestBody := types.InstallAppRequest{
			IgnoreAppPreflights: true,
		}
		reqBodyBytes, err := json.Marshal(requestBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/install", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Authorization", "Bearer TOKEN")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response - should succeed because preflights are bypassed
		require.Equal(t, http.StatusOK, rec.Code, "expected status ok, got %d with body %s", rec.Code, rec.Body.String())

		// Wait for the state machine to transition through StateAppPreflightsFailedBypassed to StateSucceeded
		assert.Eventually(t, func() bool {
			return stateMachine.CurrentState() == states.StateSucceeded
		}, 10*time.Second, 100*time.Millisecond, "state should transition to Succeeded")

		// Verify mocks
		mockAppInstallManager.AssertExpectations(t)
	})

	// Test app preflight bypass denied - should fail when ignoreAppPreflights is false and preflights failed
	t.Run("App preflight bypass denied with failed preflights", func(t *testing.T) {
		// Create a real runtime config with temp directory
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		// Create mock store
		mockStore := &store.MockStore{}

		// Create state machine starting from AppPreflightsFailed
		stateMachine := linuxinstall.NewStateMachine(
			linuxinstall.WithCurrentState(states.StateAppPreflightsFailed),
		)

		// Create real app install controller
		appInstallController, err := appinstall.NewInstallController(
			appinstall.WithStateMachine(stateMachine),
			appinstall.WithStore(mockStore),
			appinstall.WithReleaseData(releaseData),
		)
		require.NoError(t, err)

		// Create Linux install controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(stateMachine),
			linuxinstall.WithAppInstallController(appInstallController),
			linuxinstall.WithReleaseData(releaseData),
		)
		require.NoError(t, err)

		// Create the API
		cfg := types.APIConfig{
			Password:    "password",
			ReleaseData: releaseData,
			LinuxConfig: types.LinuxConfig{
				RuntimeConfig: rc,
			},
		}
		apiInstance, err := api.New(cfg,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request with ignoreAppPreflights=false
		requestBody := types.InstallAppRequest{
			IgnoreAppPreflights: false,
		}
		reqBodyBytes, err := json.Marshal(requestBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/install", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Authorization", "Bearer TOKEN")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response - should fail because preflights failed and not bypassed
		require.Equal(t, http.StatusBadRequest, rec.Code)

		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, apiError.StatusCode)
		assert.Contains(t, apiError.Message, "app preflight checks failed")
	})
}
