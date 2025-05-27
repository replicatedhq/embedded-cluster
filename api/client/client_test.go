package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	// Test default client creation
	c := New("http://example.com")
	clientImpl, ok := c.(*client)
	assert.True(t, ok, "Expected c to be of type *client")
	assert.Equal(t, "http://example.com", clientImpl.apiURL)
	assert.Equal(t, http.DefaultClient, clientImpl.httpClient)
	assert.Empty(t, clientImpl.token)

	// Test with custom HTTP client
	customHTTPClient := &http.Client{}
	c = New("http://example.com", WithHTTPClient(customHTTPClient))
	clientImpl, ok = c.(*client)
	assert.True(t, ok, "Expected c to be of type *client")
	assert.Equal(t, customHTTPClient, clientImpl.httpClient)

	// Test with token
	c = New("http://example.com", WithToken("test-token"))
	clientImpl, ok = c.(*client)
	assert.True(t, ok, "Expected c to be of type *client")
	assert.Equal(t, "test-token", clientImpl.token)

	// Test with multiple options
	c = New("http://example.com", WithHTTPClient(customHTTPClient), WithToken("test-token"))
	clientImpl, ok = c.(*client)
	assert.True(t, ok, "Expected c to be of type *client")
	assert.Equal(t, customHTTPClient, clientImpl.httpClient)
	assert.Equal(t, "test-token", clientImpl.token)
}

func TestLogin(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/auth/login", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// Decode request body
		var loginReq struct {
			Password string `json:"password"`
		}
		err := json.NewDecoder(r.Body).Decode(&loginReq)
		require.NoError(t, err, "Failed to decode request body")

		// Check password
		if loginReq.Password == "correct-password" {
			// Return successful response
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(struct {
				Token string `json:"token"`
			}{
				Token: "test-token",
			})
		} else {
			// Return error response
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(types.APIError{
				StatusCode: http.StatusUnauthorized,
				Message:    "Invalid password",
			})
		}
	}))
	defer server.Close()

	// Test successful login
	c := New(server.URL)
	err := c.Login("correct-password")
	assert.NoError(t, err)

	// Check that token was set
	clientImpl, ok := c.(*client)
	require.True(t, ok, "Expected c to be of type *client")
	assert.Equal(t, "test-token", clientImpl.token)

	// Test failed login
	c = New(server.URL)
	err = c.Login("wrong-password")
	assert.Error(t, err)

	// Check that error is of type APIError
	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
	assert.Equal(t, "Invalid password", apiErr.Message)
}

func TestGetInstall(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/install", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.Install{
			Config: types.InstallationConfig{
				GlobalCIDR:       "10.0.0.0/24",
				AdminConsolePort: 8080,
			},
		})
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	install, err := c.GetInstall()
	assert.NoError(t, err)
	assert.NotNil(t, install)
	assert.Equal(t, "10.0.0.0/24", install.Config.GlobalCIDR)
	assert.Equal(t, 8080, install.Config.AdminConsolePort)

	// Test error response
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(types.APIError{
			StatusCode: http.StatusInternalServerError,
			Message:    "Internal Server Error",
		})
	}))
	defer errorServer.Close()

	c = New(errorServer.URL, WithToken("test-token"))
	install, err = c.GetInstall()
	assert.Error(t, err)
	assert.Nil(t, install)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestSetInstallConfig(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method and path
		assert.Equal(t, "POST", r.Method) // Corrected from PUT to POST based on implementation
		assert.Equal(t, "/api/install/config", r.URL.Path)

		// Check headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Decode request body
		var config types.InstallationConfig
		err := json.NewDecoder(r.Body).Decode(&config)
		require.NoError(t, err, "Failed to decode request body")

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.Install{
			Config: config,
		})
	}))
	defer server.Close()

	// Test successful set
	c := New(server.URL, WithToken("test-token"))
	config := types.InstallationConfig{
		GlobalCIDR:              "20.0.0.0/24",
		LocalArtifactMirrorPort: 9081,
	}
	install, err := c.SetInstallConfig(config)
	assert.NoError(t, err)
	assert.NotNil(t, install)
	assert.Equal(t, config, install.Config)

	// Test error response
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(types.APIError{
			StatusCode: http.StatusBadRequest,
			Message:    "Bad Request",
		})
	}))
	defer errorServer.Close()

	c = New(errorServer.URL, WithToken("test-token"))
	install, err = c.SetInstallConfig(config)
	assert.Error(t, err)
	assert.Nil(t, install)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "Bad Request", apiErr.Message)
}

func TestSetInstallStatus(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/install/status", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Decode request body
		var status types.InstallationStatus
		err := json.NewDecoder(r.Body).Decode(&status)
		require.NoError(t, err, "Failed to decode request body")

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.Install{
			Status: status,
		})
	}))
	defer server.Close()

	// Test successful set
	c := New(server.URL, WithToken("test-token"))
	status := types.InstallationStatus{
		State:       types.InstallationStateSucceeded,
		Description: "Installation successful",
	}
	install, err := c.SetInstallStatus(status)
	assert.NoError(t, err)
	assert.NotNil(t, install)
	assert.Equal(t, status, install.Status)

	// Test error response
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(types.APIError{
			StatusCode: http.StatusBadRequest,
			Message:    "Bad Request",
		})
	}))
	defer errorServer.Close()

	c = New(errorServer.URL, WithToken("test-token"))
	install, err = c.SetInstallStatus(status)
	assert.Error(t, err)
	assert.Nil(t, install)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "Bad Request", apiErr.Message)
}

func TestErrorFromResponse(t *testing.T) {
	// Create a response with an error
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewBufferString(`{"status_code": 400, "message": "Bad Request"}`)),
	}

	err := errorFromResponse(resp)
	assert.Error(t, err)

	// Check that error is of type APIError
	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "Bad Request", apiErr.Message)

	// Test with malformed JSON
	resp = &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(bytes.NewBufferString(`not a json`)),
	}

	err = errorFromResponse(resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response")
}
