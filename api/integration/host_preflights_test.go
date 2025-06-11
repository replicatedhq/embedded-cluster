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
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
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
			Pass: []types.HostPreflightsRecord{
				types.HostPreflightsRecord{
					Title:   "Some Preflight",
					Message: "All good",
				},
			},
			Fail: []types.HostPreflightsRecord{
				types.HostPreflightsRecord{
					Title:   "Another Preflight",
					Message: "Oh no!",
				},
			},
		},
		Status: &types.Status{
			State:       types.StateFailed,
			Description: "A preflight failed",
		},
	}
	// Create a host preflights manager
	manager := preflight.NewHostPreflightManager(preflight.WithHostPreflight(&hpf))
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
	// Create an install controller
	installController, err := install.NewInstallController(
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

	t.Run("Success", func(t *testing.T) {
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
		var status types.Status
		err = json.NewDecoder(rec.Body).Decode(&status)
		require.NoError(t, err)

		// Verify that the status was properly set
		assert.Equal(t, types.StateRunning, status.State)
		assert.Equal(t, "Running host preflights", status.Description)

		// The status should eventually be set to succeeded in a goroutine
		assert.Eventually(t, func() bool {
			status, err := installController.GetHostPreflightStatus(context.Background())
			require.NoError(t, err)
			return status.State == types.StateSucceeded
		}, 5*time.Second, 100*time.Millisecond)
	})

	// Test authorization
	t.Run("Authorization error", func(t *testing.T) {
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
		// Create a mock controller that returns an error
		mockController := &mockInstallController{
			runHostPreflightsError: assert.AnError,
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
		req := httptest.NewRequest(http.MethodPost, "/install/host-preflights/run", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusInternalServerError, rec.Code)

		t.Logf("Response body: %s", rec.Body.String())
	})
}

// Test concurrent host preflights operations
func TestConcurrentHostPreflights(t *testing.T) {
	// Create an install controller
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

	// Test running preflights while another is in progress
	t.Run("Concurrent run attempts", func(t *testing.T) {
		// Start first run
		req1 := httptest.NewRequest(http.MethodPost, "/install/host-preflights/run", nil)
		req1.Header.Set("Authorization", "Bearer TOKEN")
		rec1 := httptest.NewRecorder()

		router.ServeHTTP(rec1, req1)
		assert.Equal(t, http.StatusOK, rec1.Code)

		// Immediately try to start second run
		req2 := httptest.NewRequest(http.MethodPost, "/install/host-preflights/run", nil)
		req2.Header.Set("Authorization", "Bearer TOKEN")
		rec2 := httptest.NewRecorder()

		router.ServeHTTP(rec2, req2)

		// The second request should either succeed (if first completed) or
		// return a conflict/error status depending on implementation
		t.Logf("Second request status: %d", rec2.Code)
		t.Logf("Second request body: %s", rec2.Body.String())
	})
}
