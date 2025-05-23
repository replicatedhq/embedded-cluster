package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	"github.com/replicatedhq/embedded-cluster/api/controllers/auth"
	"github.com/replicatedhq/embedded-cluster/api/controllers/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/installation"
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
			installation.WithNetUtils(&mockNetUtils{iface: "eth0"}),
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
		assert.NotEmpty(t, loginResponse.SessionToken)

		// Now use this token to access a protected route
		getInstallReq := httptest.NewRequest(http.MethodGet, "/install", nil)
		getInstallReq.Header.Set("Authorization", loginResponse.SessionToken)
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
		req.Header.Set("Authorization", "invalid-token")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check that we got a 401 Unauthorized
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
