package install

import (
	"encoding/json"
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
	appinstallmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/install"
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
		appInstallController, err := appinstall.NewInstallController(
			appinstall.WithAppInstallManager(appInstallManager),
			appinstall.WithStateMachine(linuxinstall.NewStateMachine()),
			appinstall.WithStore(mockStore),
			appinstall.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create Linux install controller with runtime config
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine()),
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
		mockReporter := &metrics.MockReporter{}
		mockReporter.On("ReportInstallationSucceeded", mock.Anything)

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
		appInstallController, err := appinstall.NewInstallController(
			appinstall.WithAppInstallManager(mockAppInstallManager),
			appinstall.WithStateMachine(stateMachine),
			appinstall.WithStore(mockStore),
			appinstall.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create Linux install controller with mock metrics reporter and embedded app controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(stateMachine),
			linuxinstall.WithAppInstallController(appInstallController),
			linuxinstall.WithMetricsReporter(mockReporter),
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
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/install", nil)
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
		mockAppInstallManager.AssertExpectations(t)
	})

	t.Run("Invalid state transition", func(t *testing.T) {
		// Create state machine starting from invalid state (infrastructure installing)
		stateMachine := linuxinstall.NewStateMachine(
			linuxinstall.WithCurrentState(states.StateInfrastructureInstalling),
		)

		// Create simple app install controller
		mockStore := &store.MockStore{}
		appInstallController, err := appinstall.NewInstallController(
			appinstall.WithStateMachine(stateMachine),
			appinstall.WithStore(mockStore),
			appinstall.WithReleaseData(integration.DefaultReleaseData()),
		)
		require.NoError(t, err)

		// Create Linux install controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(stateMachine),
			linuxinstall.WithAppInstallController(appInstallController),
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
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/install", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response - should fail with conflict
		require.Equal(t, http.StatusConflict, rec.Code)
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
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/install", nil)
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}