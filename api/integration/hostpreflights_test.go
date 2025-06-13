package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	"github.com/replicatedhq/embedded-cluster/api/controllers/install"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Test the getHostPreflightsStatus endpoint returns host preflights status correctly
func TestGetHostPreflightsStatus(t *testing.T) {
	hpf := types.HostPreflights{
		Titles: []string{
			"Some Preflight",
			"Another Preflight",
		},
		Output: &types.HostPreflightsOutput{
			Pass: []types.HostPreflightsRecord{types.HostPreflightsRecord{
				Title:   "Some Preflight",
				Message: "All good",
			}},
			Fail: []types.HostPreflightsRecord{types.HostPreflightsRecord{
				Title:   "Another Preflight",
				Message: "Oh no!",
			}},
		},
		Status: &types.Status{
			State:       types.StateFailed,
			Description: "A preflight failed",
		},
	}
	runner := &preflights.MockPreflightRunner{}
	// Create a host preflights manager
	manager := preflight.NewHostPreflightManager(
		preflight.WithHostPreflightStore(preflight.NewMemoryStore(&hpf)),
		preflight.WithPreflightRunner(runner),
	)
	// Create an install controller
	installController, err := install.NewInstallController(install.WithHostPreflightManager(manager))
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
		req := httptest.NewRequest(http.MethodGet, "/install/host-preflights/status", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var status types.InstallHostPreflightsStatusResponse
		err = json.NewDecoder(rec.Body).Decode(&status)
		require.NoError(t, err)

		// Verify the status matches what we expect
		assert.Equal(t, hpf.Status, status.Status)
		assert.Equal(t, hpf.Output, status.Output)
		assert.Equal(t, hpf.Titles, status.Titles)
	})

	// Test authorization
	t.Run("Authorization error", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/install/host-preflights/status", nil)
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

	// Test error handling
	t.Run("Controller error", func(t *testing.T) {
		// Create a mock controller that returns an error
		mockController := &mockInstallController{
			getHostPreflightStatusError: assert.AnError,
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
		req := httptest.NewRequest(http.MethodGet, "/install/host-preflights/status", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
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

// Test the postRunHostPreflights endpoint runs host preflights correctly
func TestPostRunHostPreflights(t *testing.T) {
	// Create a runtime config
	rc := runtimeconfig.New(nil)
	rc.SetDataDir(t.TempDir())

	t.Run("Success", func(t *testing.T) {
		// Mock preflight runner
		runner := &preflights.MockPreflightRunner{}

		// Creeate the installation struct
		inst := types.NewInstallation()

		// Create a host preflights manager with the mock runner
		pfManager := preflight.NewHostPreflightManager(
			preflight.WithRuntimeConfig(rc),
			preflight.WithPreflightRunner(runner),
		)

		// Create an installation manager
		iManager := installation.NewInstallationManager(
			installation.WithRuntimeConfig(rc),
			installation.WithInstallationStore(installation.NewMemoryStore(inst)),
		)

		// Create an install controller with the mocked manager
		installController, err := install.NewInstallController(
			install.WithHostPreflightManager(pfManager),
			install.WithInstallationManager(iManager),
			// Mock the release data used by the preflight runner
			install.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease: &release.ChannelRelease{
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.example.com",
						ProxyRegistryDomain: "some-proxy.example.com",
					},
				},
			}),
			install.WithRuntimeConfig(rc),
		)
		require.NoError(t, err)

		// Get the node IP for the preflight runner
		nodeIP, err := netutils.FirstValidAddress(inst.Config.NetworkInterface)
		require.NoError(t, err)

		// Mock the preflight spec's returned by prepare and used in run
		hpfc := &troubleshootv1beta2.HostPreflightSpec{}

		mock.InOrder(
			runner.On("Prepare", mock.Anything, preflights.PrepareOptions{
				K0sDataDir:       rc.EmbeddedClusterK0sSubDir(),
				OpenEBSDataDir:   rc.EmbeddedClusterOpenEBSLocalSubDir(),
				NodeIP:           nodeIP,
				ReplicatedAppURL: "https://replicated.example.com",
				ProxyRegistryURL: "https://some-proxy.example.com",
			}).Return(hpfc, nil),
			// For a successful run, we expect the runner to return an output without any errors or warnings
			runner.On("Run", mock.Anything, hpfc, mock.Anything, rc).Return(&types.HostPreflightsOutput{}, "", nil),
			runner.On("SaveToDisk", mock.Anything, mock.Anything).Return(nil),
			runner.On("CopyBundleTo", mock.Anything, mock.Anything).Return(nil),
		)

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
		req := httptest.NewRequest(http.MethodPost, "/install/host-preflights/run", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)

		t.Logf("Response body: %s", rec.Body.String())

		// Parse the response body
		var status types.InstallHostPreflightsStatusResponse
		err = json.NewDecoder(rec.Body).Decode(&status)
		require.NoError(t, err)

		// Verify that the status was properly set
		assert.Equal(t, types.StateRunning, status.Status.State)
		assert.Equal(t, "Running host preflights", status.Status.Description)

		// The status should eventually be set to succeeded in a goroutine
		assert.Eventually(t, func() bool {
			status, err := installController.GetHostPreflightStatus(context.Background())
			t.Logf("Status: %s, Description: %s", status.State, status.Description)
			require.NoError(t, err)
			return status.State == types.StateSucceeded
		}, 5*time.Second, 100*time.Millisecond)

		// Verify that the mock expectations were met
		runner.AssertExpectations(t)
	})

	// Test authorization
	t.Run("Authorization error", func(t *testing.T) {
		// Mock preflight runner (not used in this test case)
		runner := &preflights.MockPreflightRunner{}

		// Create a host preflights manager
		manager := preflight.NewHostPreflightManager(
			preflight.WithPreflightRunner(runner),
		)

		// Create an install controller
		installController, err := install.NewInstallController(
			install.WithHostPreflightManager(manager),
			install.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease:        &release.ChannelRelease{},
			}),
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

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/install/host-preflights/run", nil)
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

	// Test controller error
	t.Run("Controller error", func(t *testing.T) {
		// Mock preflight runner that returns an error
		runner := &preflights.MockPreflightRunner{}
		runner.On("Prepare", mock.Anything, mock.Anything).Return(nil, assert.AnError)

		// Create a host preflights manager with the failing mock runner
		manager := preflight.NewHostPreflightManager(
			preflight.WithPreflightRunner(runner),
		)

		// Create an install controller with the failing manager
		installController, err := install.NewInstallController(
			install.WithHostPreflightManager(manager),
			install.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease:        &release.ChannelRelease{},
			}),
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

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/install/host-preflights/run", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusInternalServerError, rec.Code)

		t.Logf("Response body: %s", rec.Body.String())

		// Verify that the mock expectations were met
		runner.AssertExpectations(t)
	})

	// Test controller error that takes place as part of the async run go routine
	t.Run("Controller run error", func(t *testing.T) {
		// Mock the preflight spec's returned by prepare and used in run
		hpfc := &troubleshootv1beta2.HostPreflightSpec{}
		// Mock preflight runner that returns an error
		runner := &preflights.MockPreflightRunner{}
		mock.InOrder(
			runner.On("Prepare", mock.Anything, mock.Anything).Return(hpfc, nil),
			runner.On("Run", mock.Anything, hpfc, mock.Anything, mock.Anything).Return(nil, "this is an error", assert.AnError),
		)
		// Create a host preflights manager with the failing mock runner
		manager := preflight.NewHostPreflightManager(
			preflight.WithPreflightRunner(runner),
		)

		// Create an install controller with the failing manager
		installController, err := install.NewInstallController(
			install.WithHostPreflightManager(manager),
			install.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease:        &release.ChannelRelease{},
			}),
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

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/install/host-preflights/run", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)

		t.Logf("Response body: %s", rec.Body.String())

		// Parse the response body
		var status types.InstallHostPreflightsStatusResponse
		err = json.NewDecoder(rec.Body).Decode(&status)
		require.NoError(t, err)

		// Verify that the status was properly set
		assert.Equal(t, types.StateRunning, status.Status.State)
		assert.Equal(t, "Running host preflights", status.Status.Description)

		// The status should eventually be set to failed in a goroutine
		assert.Eventually(t, func() bool {
			status, err := installController.GetHostPreflightStatus(context.Background())
			t.Logf("Status: %s, Description: %s", status.State, status.Description)
			require.NoError(t, err)
			return status.State == types.StateFailed
		}, 5*time.Second, 100*time.Millisecond)

		// Verify that the mock expectations were met
		runner.AssertExpectations(t)
	})

	// Test we get a conflict error if preflights are already running
	t.Run("Preflights already running errror", func(t *testing.T) {
		// Create a host preflights manager with the failing mock runner
		hp := types.NewHostPreflights()
		hp.Status = &types.Status{
			State:       types.StateRunning,
			Description: "Preflights running",
		}
		manager := preflight.NewHostPreflightManager(
			preflight.WithHostPreflightStore(preflight.NewMemoryStore(hp)),
		)

		// Create an install controller with the failing manager
		installController, err := install.NewInstallController(
			install.WithHostPreflightManager(manager),
			install.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease:        &release.ChannelRelease{},
			}),
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

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/install/host-preflights/run", nil)
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
	})
}
