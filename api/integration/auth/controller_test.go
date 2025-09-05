package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	"github.com/replicatedhq/embedded-cluster/api/client"
	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	linuxinstallation "github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/installation"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthLoginAndTokenValidation(t *testing.T) {
	// Create an auth controller
	authController, err := auth.NewAuthController("password")
	require.NoError(t, err)

	// Create an install controller
	installController, err := linuxinstall.NewInstallController(
		linuxinstall.WithInstallationManager(linuxinstallation.NewInstallationManager(
			linuxinstallation.WithNetUtils(&utils.MockNetUtils{}),
		)),
		linuxinstall.WithReleaseData(integration.DefaultReleaseData()),
		linuxinstall.WithHelmClient(&helm.MockClient{}),
	)
	require.NoError(t, err)

	// Create the API with the auth controller
	apiInstance := integration.NewTargetLinuxAPIWithReleaseData(t,
		api.WithAuthController(authController),
		api.WithLinuxInstallController(installController),
		api.WithLogger(logger.NewDiscardLogger()),
	)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test successful login
	t.Run("successful login", func(t *testing.T) {
		// Create login request with correct password
		loginReq := types.AuthRequest{
			Password: "password",
		}
		loginReqJSON, err := json.Marshal(loginReq)
		require.NoError(t, err)

		// Make the login request
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginReqJSON))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the login response
		assert.Equal(t, http.StatusOK, rec.Code)

		var loginResponse types.AuthResponse
		err = json.NewDecoder(rec.Body).Decode(&loginResponse)
		require.NoError(t, err)

		// Validate that we got a session token
		assert.NotEmpty(t, loginResponse.Token)

		// Now use this token to access a protected route
		getInstallReq := httptest.NewRequest(http.MethodGet, "/install", nil)
		getInstallReq.Header.Set("Authorization", "Bearer "+loginResponse.Token)
		getInstallRec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(getInstallRec, getInstallReq)

		// Check that we got a 200 OK (or at least not a 401 Unauthorized)
		assert.NotEqual(t, http.StatusUnauthorized, getInstallRec.Code)
	})

	// Test failed login with incorrect password
	t.Run("failed login with incorrect password", func(t *testing.T) {
		// Create login request with incorrect password
		loginReq := types.AuthRequest{
			Password: "wrong-password",
		}
		loginReqJSON, err := json.Marshal(loginReq)
		require.NoError(t, err)

		// Make the login request
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(loginReqJSON))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check that we got a 401 Unauthorized
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	// Test access to protected route without token
	t.Run("access protected route without token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/linux/install/installation/config", nil)
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check that we got a 401 Unauthorized
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	// Test access to protected route with invalid token
	t.Run("access protected route with invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/linux/install/installation/config", nil)
		req.Header.Set("Authorization", "Bearer "+"invalid-token")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check that we got a 401 Unauthorized
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestAPIClientLogin(t *testing.T) {
	// Create the API with the auth controller
	apiInstance := integration.NewTargetLinuxAPIWithReleaseData(t,
		api.WithLogger(logger.NewDiscardLogger()),
	)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router.PathPrefix("/api").Subrouter())

	// Create a test server using the router
	server := httptest.NewServer(router)
	defer server.Close()

	// Test successful login with API client
	t.Run("successful login with client", func(t *testing.T) {
		// Create an API client
		c := client.New(server.URL)

		// Login with the client
		err := c.Authenticate("password")
		require.NoError(t, err, "API client login should succeed with correct password")

		// Verify we can make authenticated requests after login
		_, err = c.GetLinuxInstallationStatus()
		require.NoError(t, err, "API client should be able to get installation status after successful login")
	})

	// Test failed login with incorrect password
	t.Run("failed login with incorrect password", func(t *testing.T) {
		// Create a new client for this test
		c := client.New(server.URL)

		// Attempt to login with wrong password
		err := c.Authenticate("wrong-password")
		require.Error(t, err, "API client login should fail with wrong password")

		// Check that the error is of correct type
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode, "Error should have Unauthorized status code")

		// Verify we can't make authenticated requests
		_, err = c.GetLinuxInstallationStatus()
		require.Error(t, err, "API client should not be able to get installation status after failed login")
	})
}
