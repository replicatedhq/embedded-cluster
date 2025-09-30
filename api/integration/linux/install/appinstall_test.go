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
	appcontroller "github.com/replicatedhq/embedded-cluster/api/controllers/app"
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	appinstallmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/install"
	apppreflightmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/preflight"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	appinstallstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestGetAppInstallStatus tests the GET /linux/install/app/status endpoint
func TestGetAppInstallStatus(t *testing.T) {
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
		appController, err := appcontroller.NewAppController(
			appcontroller.WithAppInstallManager(appInstallManager),
			appcontroller.WithStateMachine(linuxinstall.NewStateMachine()),
			appcontroller.WithStore(mockStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create Linux install controller with runtime config
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine()),
			linuxinstall.WithAppController(appController),
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
			linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
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
	t.Run("Success", func(t *testing.T) {
		// Create a real runtime config with temp directory
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		// Create mock metrics reporter - this is the key test for reporting handlers
		mockMetricsReporter := &metrics.MockReporter{}
		mockMetricsReporter.On("ReportInstallationSucceeded", mock.Anything)

		// Create mock app install manager that succeeds
		mockAppInstallManager := &appinstallmanager.MockAppInstallManager{}
		mockAppInstallManager.On("Install", mock.Anything, mock.Anything).Return(nil)
		mockAppInstallManager.On("GetStatus").Return(types.AppInstall{
			Status: types.Status{
				State:       types.StateRunning,
				Description: "Installing application",
			},
		}, nil)

		// Create mock store
		mockStore := &store.MockStore{}
		mockStore.AppConfigMockStore.On("GetKotsadmConfigValues").Return(kotsv1beta1.ConfigValues{}, nil)
		mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

		// Create state machine starting from AppPreflightsSucceeded (valid state for app install)
		stateMachine := linuxinstall.NewStateMachine(
			linuxinstall.WithCurrentState(states.StateAppPreflightsSucceeded),
		)

		// Create real app install controller with mock manager
		appController, err := appcontroller.NewAppController(
			appcontroller.WithAppInstallManager(mockAppInstallManager),
			appcontroller.WithStateMachine(stateMachine),
			appcontroller.WithStore(mockStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create Linux install controller with mock metrics reporter and embedded app controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(stateMachine),
			linuxinstall.WithAppController(appController),
			linuxinstall.WithMetricsReporter(mockMetricsReporter),
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

		mockAppInstallManager.AssertExpectations(t)

		// Wait for the event handler goroutine to complete
		// TODO: find a better way to do this
		time.Sleep(1 * time.Second)
		// Verify that ReportInstallationSucceeded was called
		mockMetricsReporter.AssertExpectations(t)
	})

	t.Run("Invalid state transition", func(t *testing.T) {
		// Create state machine starting from invalid state (infrastructure installing)
		stateMachine := linuxinstall.NewStateMachine(
			linuxinstall.WithCurrentState(states.StateInfrastructureInstalling),
		)

		// Create simple app install controller
		mockStore := &store.MockStore{}
		appController, err := appcontroller.NewAppController(
			appcontroller.WithStateMachine(stateMachine),
			appcontroller.WithStore(mockStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create Linux install controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(stateMachine),
			linuxinstall.WithAppController(appController),
			linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
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
		mockMetricsReporter := &metrics.MockReporter{}
		mockMetricsReporter.On("ReportInstallationFailed", mock.Anything, mock.MatchedBy(func(err error) bool {
			return err.Error() == "install app: installation failed"
		}))

		// Create mock app install manager that fails
		mockAppInstallManager := &appinstallmanager.MockAppInstallManager{}
		mockAppInstallManager.On("Install", mock.Anything, mock.Anything).Return(errors.New("installation failed"))
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
		appController, err := appcontroller.NewAppController(
			appcontroller.WithAppInstallManager(mockAppInstallManager),
			appcontroller.WithStateMachine(stateMachine),
			appcontroller.WithStore(mockStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create Linux install controller with mock metrics reporter and embedded app controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(stateMachine),
			linuxinstall.WithAppController(appController),
			linuxinstall.WithMetricsReporter(mockMetricsReporter),
			linuxinstall.WithStore(mockStore),
			linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
			linuxinstall.WithRuntimeConfig(rc),
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

		mockAppInstallManager.AssertExpectations(t)
		mockStore.AppInstallMockStore.AssertExpectations(t)

		// Wait for the event handler goroutine to complete
		// TODO: find a better way to do this
		time.Sleep(1 * time.Second)
		// Verify that ReportInstallationFailed was called
		mockMetricsReporter.AssertExpectations(t)
	})

	t.Run("Authorization error", func(t *testing.T) {
		// Create simple Linux install controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
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
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/install", bytes.NewReader([]byte(`{}`)))
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	// Test app preflight bypass - should succeed when ignoreAppPreflights is true
	t.Run("App preflight bypass with failed preflights - ignored", func(t *testing.T) {
		// Create mock store
		mockStore := &store.MockStore{}
		mockStore.AppConfigMockStore.On("GetKotsadmConfigValues").Return(kotsv1beta1.ConfigValues{}, nil)
		mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

		// Create mock app install manager that succeeds
		mockAppInstallManager := &appinstallmanager.MockAppInstallManager{}
		mockAppInstallManager.On("Install", mock.Anything, mock.Anything).Return(nil)
		mockAppInstallManager.On("GetStatus").Return(types.AppInstall{
			Status: types.Status{
				State:       types.StateRunning,
				Description: "Installing application",
			},
		}, nil)

		// Create mock app preflight manager that returns non-strict failures (can be bypassed)
		mockAppPreflightManager := &apppreflightmanager.MockAppPreflightManager{}
		mockAppPreflightManager.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
			Fail: []types.PreflightsRecord{
				{
					Title:   "Non-strict preflight failure",
					Message: "This is a non-strict failure",
					Strict:  false, // This allows bypass
				},
			},
		}, nil)

		// Create state machine starting from AppPreflightsFailed
		stateMachine := linuxinstall.NewStateMachine(
			linuxinstall.WithCurrentState(states.StateAppPreflightsFailed),
		)

		// Create real app install controller with mock app preflight manager
		appController, err := appcontroller.NewAppController(
			appcontroller.WithAppInstallManager(mockAppInstallManager),
			appcontroller.WithAppPreflightManager(mockAppPreflightManager),
			appcontroller.WithStateMachine(stateMachine),
			appcontroller.WithStore(mockStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create Linux install controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(stateMachine),
			linuxinstall.WithAppController(appController),
			linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create the API
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

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
		mockAppPreflightManager.AssertExpectations(t)
	})

	// Test app preflight bypass denied - should fail when ignoreAppPreflights is false and preflights failed
	t.Run("App preflight bypass denied with failed preflights", func(t *testing.T) {
		// Create mock store
		mockStore := &store.MockStore{}

		// Create mock app preflight manager that returns non-strict failures (method should be called but bypass denied)
		mockAppPreflightManager := &apppreflightmanager.MockAppPreflightManager{}
		mockAppPreflightManager.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
			Fail: []types.PreflightsRecord{
				{
					Title:   "Non-strict preflight failure",
					Message: "This is a non-strict failure",
					Strict:  false, // Non-strict but bypass still denied due to ignoreAppPreflights=false
				},
			},
		}, nil)

		// Create state machine starting from AppPreflightsFailed
		stateMachine := linuxinstall.NewStateMachine(
			linuxinstall.WithCurrentState(states.StateAppPreflightsFailed),
		)

		// Create real app install controller with mock app preflight manager
		appController, err := appcontroller.NewAppController(
			appcontroller.WithAppPreflightManager(mockAppPreflightManager),
			appcontroller.WithStateMachine(stateMachine),
			appcontroller.WithStore(mockStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create Linux install controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(stateMachine),
			linuxinstall.WithAppController(appController),
			linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create the API
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

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

		// Verify mocks
		mockAppPreflightManager.AssertExpectations(t)
	})

	// Test strict app preflight bypass blocked - should fail even with ignoreAppPreflights=true when strict failures exist
	t.Run("Strict app preflight bypass blocked", func(t *testing.T) {
		// Create mock store
		mockStore := &store.MockStore{}

		// Create mock app preflight manager that returns strict failures (cannot be bypassed)
		mockAppPreflightManager := &apppreflightmanager.MockAppPreflightManager{}
		mockAppPreflightManager.On("GetAppPreflightOutput", mock.Anything).Return(&types.PreflightsOutput{
			Fail: []types.PreflightsRecord{
				{
					Title:   "Strict preflight failure",
					Message: "This is a strict failure that cannot be bypassed",
					Strict:  true, // Strict failure - cannot be bypassed
				},
			},
		}, nil)

		// Create state machine starting from AppPreflightsFailed
		stateMachine := linuxinstall.NewStateMachine(
			linuxinstall.WithCurrentState(states.StateAppPreflightsFailed),
		)

		// Create real app install controller with mock app preflight manager
		appController, err := appcontroller.NewAppController(
			appcontroller.WithAppPreflightManager(mockAppPreflightManager),
			appcontroller.WithStateMachine(stateMachine),
			appcontroller.WithStore(mockStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create Linux install controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(stateMachine),
			linuxinstall.WithAppController(appController),
			linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create the API
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request with ignoreAppPreflights=true (should still fail due to strict failures)
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

		// Check the response - should fail because of strict preflight failures
		require.Equal(t, http.StatusBadRequest, rec.Code)

		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, apiError.StatusCode)
		assert.Contains(t, apiError.Message, "installation blocked: strict app preflight checks failed")

		// Verify mocks
		mockAppPreflightManager.AssertExpectations(t)
	})
}
