package integration

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
	"github.com/replicatedhq/embedded-cluster/api/controllers/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/installation"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthLoginAndTokenValidation(t *testing.T) {
	password := "test-password"

	// Create an auth controller
	authController, err := auth.NewAuthController(password)
	require.NoError(t, err)

	// Create an install controller
	installController, err := install.NewInstallController(
		install.WithInstallationManager(installation.NewInstallationManager(
			installation.WithNetUtils(&mockNetUtils{ifaces: []string{"eth0", "eth1"}}),
		)),
	)
	require.NoError(t, err)

	// Create the API with the auth controller
	apiInstance, err := api.New(
		password,
		api.WithAuthController(authController),
		api.WithInstallController(installController),
		api.WithLogger(api.NewDiscardLogger()),
	)
	require.NoError(t, err)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test successful login
	t.Run("successful login", func(t *testing.T) {
		// Create login request with correct password
		loginReq := api.AuthRequest{
			Password: password,
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

		var loginResponse api.AuthResponse
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
		loginReq := api.AuthRequest{
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
		req := httptest.NewRequest(http.MethodGet, "/install", nil)
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check that we got a 401 Unauthorized
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	// Test access to protected route with invalid token
	t.Run("access protected route with invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/install", nil)
		req.Header.Set("Authorization", "Bearer "+"invalid-token")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check that we got a 401 Unauthorized
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func TestAPIClientLogin(t *testing.T) {
	password := "test-password"

	// Create an auth controller
	authController, err := auth.NewAuthController(password)
	require.NoError(t, err)

	// Create an install controller
	installController, err := install.NewInstallController(
		install.WithInstallationManager(installation.NewInstallationManager()),
	)
	require.NoError(t, err)

	// Create the API with the auth controller
	apiInstance, err := api.New(
		password,
		api.WithAuthController(authController),
		api.WithInstallController(installController),
		api.WithLogger(api.NewDiscardLogger()),
	)
	require.NoError(t, err)

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
		err := c.Login(password)
		require.NoError(t, err, "API client login should succeed with correct password")

		// Verify we can make authenticated requests after login
		install, err := c.GetInstall()
		require.NoError(t, err, "API client should be able to get install after successful login")
		assert.NotNil(t, install, "Install should not be nil")
	})

	// Test failed login with incorrect password
	t.Run("failed login with incorrect password", func(t *testing.T) {
		// Create a new client for this test
		c := client.New(server.URL)

		// Attempt to login with wrong password
		err := c.Login("wrong-password")
		require.Error(t, err, "API client login should fail with wrong password")

		// Check that the error is of correct type
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode, "Error should have Unauthorized status code")

		// Verify we can't make authenticated requests
		_, err = c.GetInstall()
		require.Error(t, err, "API client should not be able to get install after failed login")
	})
}
