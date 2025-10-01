package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
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
	err := c.Authenticate("correct-password")
	assert.NoError(t, err)

	// Check that token was set
	clientImpl, ok := c.(*client)
	require.True(t, ok, "Expected c to be of type *client")
	assert.Equal(t, "test-token", clientImpl.token)

	// Test failed login
	c = New(server.URL)
	err = c.Authenticate("wrong-password")
	assert.Error(t, err)

	// Check that error is of type APIError
	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
	assert.Equal(t, "Invalid password", apiErr.Message)
}

func TestLinuxGetInstallationConfig(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/linux/install/installation/config", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.LinuxInstallationConfigResponse{
			Values: types.LinuxInstallationConfig{
				GlobalCIDR:       "10.0.0.0/24",
				AdminConsolePort: 8080,
			},
		})
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	config, err := c.GetLinuxInstallationConfig()
	assert.NoError(t, err)
	assert.Equal(t, "10.0.0.0/24", config.Values.GlobalCIDR)
	assert.Equal(t, 8080, config.Values.AdminConsolePort)

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
	config, err = c.GetLinuxInstallationConfig()
	assert.Error(t, err)
	assert.Equal(t, types.LinuxInstallationConfigResponse{}, config)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestLinuxConfigureInstallation(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method and path
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/linux/install/installation/configure", r.URL.Path)

		// Check headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Decode request body
		var config types.LinuxInstallationConfig
		err := json.NewDecoder(r.Body).Decode(&config)
		require.NoError(t, err, "Failed to decode request body")

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.Status{
			State:       types.StateRunning,
			Description: "Configuring installation",
		})
	}))
	defer server.Close()

	// Test successful configure
	c := New(server.URL, WithToken("test-token"))
	config := types.LinuxInstallationConfig{
		GlobalCIDR:              "20.0.0.0/24",
		LocalArtifactMirrorPort: 9081,
	}
	status, err := c.ConfigureLinuxInstallation(config)
	assert.NoError(t, err)
	assert.Equal(t, types.StateRunning, status.State)
	assert.Equal(t, "Configuring installation", status.Description)

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
	status, err = c.ConfigureLinuxInstallation(config)
	assert.Error(t, err)
	assert.Equal(t, types.Status{}, status)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "Bad Request", apiErr.Message)
}

func TestLinuxSetupInfra(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/linux/install/infra/setup", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Decode request body
		var config types.LinuxInfraSetupRequest
		err := json.NewDecoder(r.Body).Decode(&config)
		require.NoError(t, err, "Failed to decode request body")

		assert.True(t, config.IgnoreHostPreflights)

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.Infra{
			Status: types.Status{
				State:       types.StateRunning,
				Description: "Installing infra",
			},
		})
	}))
	defer server.Close()

	// Test successful setup
	c := New(server.URL, WithToken("test-token"))
	infra, err := c.SetupLinuxInfra(true)
	assert.NoError(t, err)
	assert.Equal(t, types.StateRunning, infra.Status.State)
	assert.Equal(t, "Installing infra", infra.Status.Description)

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
	infra, err = c.SetupLinuxInfra(true)
	assert.Error(t, err)
	assert.Equal(t, types.Infra{}, infra)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestLinuxGetInfraStatus(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/linux/install/infra/status", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.Infra{
			Status: types.Status{
				State:       types.StateSucceeded,
				Description: "Installation successful",
			},
		})
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	infra, err := c.GetLinuxInfraStatus()
	assert.NoError(t, err)
	assert.Equal(t, types.StateSucceeded, infra.Status.State)
	assert.Equal(t, "Installation successful", infra.Status.Description)

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
	infra, err = c.GetLinuxInfraStatus()
	assert.Error(t, err)
	assert.Equal(t, types.Infra{}, infra)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestKubernetesGetInstallationConfig(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/kubernetes/install/installation/config", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.KubernetesInstallationConfigResponse{
			Values: types.KubernetesInstallationConfig{
				HTTPProxy:        "http://proxy.example.com",
				HTTPSProxy:       "https://proxy.example.com",
				NoProxy:          "localhost,127.0.0.1",
				AdminConsolePort: 8080,
			},
		})
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	config, err := c.GetKubernetesInstallationConfig()
	assert.NoError(t, err)
	assert.Equal(t, "http://proxy.example.com", config.Values.HTTPProxy)
	assert.Equal(t, "https://proxy.example.com", config.Values.HTTPSProxy)
	assert.Equal(t, "localhost,127.0.0.1", config.Values.NoProxy)
	assert.Equal(t, 8080, config.Values.AdminConsolePort)

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
	config, err = c.GetKubernetesInstallationConfig()
	assert.Error(t, err)
	assert.Equal(t, types.KubernetesInstallationConfigResponse{}, config)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestKubernetesConfigureInstallation(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method and path
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/kubernetes/install/installation/configure", r.URL.Path)

		// Check headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Decode request body
		var config types.KubernetesInstallationConfig
		err := json.NewDecoder(r.Body).Decode(&config)
		require.NoError(t, err, "Failed to decode request body")

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.Status{
			State:       types.StateSucceeded,
			Description: "Installation configured",
		})
	}))
	defer server.Close()

	// Test successful configure
	c := New(server.URL, WithToken("test-token"))
	config := types.KubernetesInstallationConfig{
		HTTPProxy:        "http://proxy.example.com",
		HTTPSProxy:       "https://proxy.example.com",
		NoProxy:          "localhost,127.0.0.1",
		AdminConsolePort: 8080,
	}
	status, err := c.ConfigureKubernetesInstallation(config)
	assert.NoError(t, err)
	assert.Equal(t, types.StateSucceeded, status.State)
	assert.Equal(t, "Installation configured", status.Description)

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
	status, err = c.ConfigureKubernetesInstallation(config)
	assert.Error(t, err)
	assert.Equal(t, types.Status{}, status)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "Bad Request", apiErr.Message)
}

func TestKubernetesGetInstallationStatus(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/kubernetes/install/installation/status", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.Status{
			State:       types.StateSucceeded,
			Description: "Installation successful",
		})
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	status, err := c.GetKubernetesInstallationStatus()
	assert.NoError(t, err)
	assert.Equal(t, types.StateSucceeded, status.State)
	assert.Equal(t, "Installation successful", status.Description)

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
	status, err = c.GetKubernetesInstallationStatus()
	assert.Error(t, err)
	assert.Equal(t, types.Status{}, status)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestKubernetesSetupInfra(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/kubernetes/install/infra/setup", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.Infra{
			Status: types.Status{
				State:       types.StateRunning,
				Description: "Installing infra",
			},
		})
	}))
	defer server.Close()

	// Test successful setup
	c := New(server.URL, WithToken("test-token"))
	infra, err := c.SetupKubernetesInfra()
	assert.NoError(t, err)
	assert.Equal(t, types.StateRunning, infra.Status.State)
	assert.Equal(t, "Installing infra", infra.Status.Description)

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
	infra, err = c.SetupKubernetesInfra()
	assert.Error(t, err)
	assert.Equal(t, types.Infra{}, infra)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestKubernetesGetInfraStatus(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/kubernetes/install/infra/status", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.Infra{
			Status: types.Status{
				State:       types.StateSucceeded,
				Description: "Installation successful",
			},
		})
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	infra, err := c.GetKubernetesInfraStatus()
	assert.NoError(t, err)
	assert.Equal(t, types.StateSucceeded, infra.Status.State)
	assert.Equal(t, "Installation successful", infra.Status.Description)

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
	infra, err = c.GetKubernetesInfraStatus()
	assert.Error(t, err)
	assert.Equal(t, types.Infra{}, infra)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
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

func TestLinuxGetAppConfigValues(t *testing.T) {
	// Define expected values once
	expectedValues := types.AppConfigValues{
		"test-key1": types.AppConfigValue{Value: "test-value1"},
		"test-key2": types.AppConfigValue{Value: "test-value2"},
		"test-key3": types.AppConfigValue{Value: "test-value3"},
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/linux/install/app/config/values", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		response := types.AppConfigValuesResponse{Values: expectedValues}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	values, err := c.GetLinuxInstallAppConfigValues()
	assert.NoError(t, err)
	assert.Equal(t, expectedValues, values)

	// Test authentication (without token)
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/linux/install/app/config/values", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Empty(t, r.Header.Get("Authorization"))

		// Return unauthorized response
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(types.APIError{
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized",
		})
	}))
	defer authServer.Close()

	c = New(authServer.URL)
	values, err = c.GetLinuxInstallAppConfigValues()
	assert.Error(t, err)
	assert.Nil(t, values)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
	assert.Equal(t, "Unauthorized", apiErr.Message)

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
	values, err = c.GetLinuxInstallAppConfigValues()
	assert.Error(t, err)
	assert.Nil(t, values)

	apiErr, ok = err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestKubernetesGetAppConfigValues(t *testing.T) {
	// Define expected values once
	expectedValues := types.AppConfigValues{
		"test-key1": types.AppConfigValue{Value: "test-value1"},
		"test-key2": types.AppConfigValue{Value: "test-value2"},
		"test-key3": types.AppConfigValue{Value: "test-value3"},
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/kubernetes/install/app/config/values", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		response := types.AppConfigValuesResponse{Values: expectedValues}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	values, err := c.GetKubernetesInstallAppConfigValues()
	assert.NoError(t, err)
	assert.Equal(t, expectedValues, values)

	// Test authentication (without token)
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/kubernetes/install/app/config/values", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Empty(t, r.Header.Get("Authorization"))

		// Return unauthorized response
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(types.APIError{
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized",
		})
	}))
	defer authServer.Close()

	c = New(authServer.URL)
	values, err = c.GetKubernetesInstallAppConfigValues()
	assert.Error(t, err)
	assert.Nil(t, values)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
	assert.Equal(t, "Unauthorized", apiErr.Message)

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
	values, err = c.GetKubernetesInstallAppConfigValues()
	assert.Error(t, err)
	assert.Nil(t, values)

	apiErr, ok = err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestLinuxPatchAppConfigValues(t *testing.T) {
	// Define expected config values once
	expectedValues := types.AppConfigValues{
		"test-item":     types.AppConfigValue{Value: "new-value"},
		"required-item": types.AppConfigValue{Value: "required-value"},
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method and path
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/api/linux/install/app/config/values", r.URL.Path)

		// Check headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Decode request body
		var req types.PatchAppConfigValuesRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err, "Failed to decode request body")

		// Verify the request contains expected values
		assert.Equal(t, "new-value", req.Values["test-item"].Value)
		assert.Equal(t, "required-value", req.Values["required-item"].Value)

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.AppConfigValuesResponse{Values: expectedValues})
	}))
	defer server.Close()

	// Test successful patch
	c := New(server.URL, WithToken("test-token"))
	configValues := types.AppConfigValues{
		"test-item":     types.AppConfigValue{Value: "new-value"},
		"required-item": types.AppConfigValue{Value: "required-value"},
	}
	config, err := c.PatchLinuxInstallAppConfigValues(configValues)
	require.NoError(t, err)
	assert.Equal(t, expectedValues, config)

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
	configValues, err = c.PatchLinuxInstallAppConfigValues(configValues)
	assert.Error(t, err)
	assert.Equal(t, types.AppConfigValues{}, configValues)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "Bad Request", apiErr.Message)
}

func TestKubernetesPatchAppConfigValues(t *testing.T) {
	// Define expected config values once
	expectedValues := types.AppConfigValues{
		"test-item":     types.AppConfigValue{Value: "new-value"},
		"required-item": types.AppConfigValue{Value: "required-values"},
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method and path
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/api/kubernetes/install/app/config/values", r.URL.Path)

		// Check headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Decode request body
		var req types.PatchAppConfigValuesRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err, "Failed to decode request body")

		// Verify the request contains expected values
		assert.Equal(t, "new-value", req.Values["test-item"].Value)
		assert.Equal(t, "required-value", req.Values["required-item"].Value)

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.AppConfigValuesResponse{Values: expectedValues})
	}))
	defer server.Close()

	// Test successful patch
	c := New(server.URL, WithToken("test-token"))
	configValues := types.AppConfigValues{
		"test-item":     types.AppConfigValue{Value: "new-value"},
		"required-item": types.AppConfigValue{Value: "required-value"},
	}
	configValuesResponse, err := c.PatchKubernetesInstallAppConfigValues(configValues)
	assert.NoError(t, err)
	assert.Equal(t, expectedValues, configValuesResponse)

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
	configValuesResponse, err = c.PatchKubernetesInstallAppConfigValues(configValues)
	assert.Error(t, err)
	assert.Equal(t, types.AppConfigValues{}, configValuesResponse)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "Bad Request", apiErr.Message)
}

func TestLinuxTemplateAppConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/linux/install/app/config/template", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var req types.TemplateAppConfigRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Mock server returns templated results (as if processed by the template engine)
		config := types.AppConfig{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "database",
					Title: "DATABASE CONFIGURATION",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "db_host",
							Title:   "Host: localhost",
							Type:    "text",
							Default: multitype.FromString("localhost"),
							Value:   multitype.FromString("localhost"),
						},
					},
				},
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(config)
	}))
	defer server.Close()

	c := New(server.URL, WithToken("test-token"))
	values := types.AppConfigValues{
		"db_host": types.AppConfigValue{Value: "localhost"},
	}

	config, err := c.TemplateLinuxInstallAppConfig(values)
	require.NoError(t, err)
	assert.Equal(t, "database", config.Groups[0].Name)
	assert.Equal(t, "DATABASE CONFIGURATION", config.Groups[0].Title)
	assert.Equal(t, "Host: localhost", config.Groups[0].Items[0].Title)
	assert.Equal(t, "localhost", config.Groups[0].Items[0].Value.StrVal)
}

func TestKubernetesTemplateAppConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/kubernetes/install/app/config/template", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var req types.TemplateAppConfigRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Mock server returns templated results (as if processed by the template engine)
		config := types.AppConfig{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "application",
					Title: "Application Settings",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "app_name",
							Title:   "APPLICATION NAME",
							Type:    "text",
							Default: multitype.FromString("my-app"),
							Value:   multitype.FromString("myapp"),
						},
					},
				},
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(config)
	}))
	defer server.Close()

	c := New(server.URL, WithToken("test-token"))
	values := types.AppConfigValues{
		"app_name": types.AppConfigValue{Value: "myapp"},
	}

	config, err := c.TemplateKubernetesInstallAppConfig(values)
	require.NoError(t, err)
	assert.Equal(t, "application", config.Groups[0].Name)
	assert.Equal(t, "Application Settings", config.Groups[0].Title)
	assert.Equal(t, "APPLICATION NAME", config.Groups[0].Items[0].Title)
	assert.Equal(t, "myapp", config.Groups[0].Items[0].Value.StrVal)
}

func TestRunLinuxAppPreflights(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/linux/install/app-preflights/run", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.InstallAppPreflightsStatusResponse{
			Status: types.Status{
				State:       types.StateRunning,
				Description: "App preflights running",
			},
			Output: &types.PreflightsOutput{
				Pass: []types.PreflightsRecord{
					{
						Title:   "App Dependencies",
						Message: "All dependencies available",
					},
				},
			},
			Titles: []string{"App Dependencies"},
		})
	}))
	defer server.Close()

	// Test successful run
	c := New(server.URL, WithToken("test-token"))
	status, err := c.RunLinuxInstallAppPreflights()
	assert.NoError(t, err)
	assert.Equal(t, types.StateRunning, status.Status.State)
	assert.Equal(t, "App preflights running", status.Status.Description)
	assert.Equal(t, "App Dependencies", status.Output.Pass[0].Title)
	assert.Equal(t, "All dependencies available", status.Output.Pass[0].Message)
	assert.Equal(t, []string{"App Dependencies"}, status.Titles)

	// Test error response
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(types.APIError{
			StatusCode: http.StatusConflict,
			Message:    "App preflights already running",
		})
	}))
	defer errorServer.Close()

	c = New(errorServer.URL, WithToken("test-token"))
	status, err = c.RunLinuxInstallAppPreflights()
	assert.Error(t, err)
	assert.Equal(t, types.InstallAppPreflightsStatusResponse{}, status)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusConflict, apiErr.StatusCode)
	assert.Equal(t, "App preflights already running", apiErr.Message)
}

func TestGetLinuxAppPreflightsStatus(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/linux/install/app-preflights/status", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.InstallAppPreflightsStatusResponse{
			Status: types.Status{
				State:       types.StateSucceeded,
				Description: "App preflights succeeded",
			},
			Output: &types.PreflightsOutput{
				Pass: []types.PreflightsRecord{
					{
						Title:   "Storage Check",
						Message: "Sufficient storage available",
					},
				},
				Fail: []types.PreflightsRecord{
					{
						Title:   "Network Check",
						Message: "Network connectivity issues",
					},
				},
			},
			Titles: []string{"Storage Check", "Network Check"},
		})
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	status, err := c.GetLinuxInstallAppPreflightsStatus()
	assert.NoError(t, err)
	assert.Equal(t, types.StateSucceeded, status.Status.State)
	assert.Equal(t, "App preflights succeeded", status.Status.Description)
	assert.Equal(t, "Storage Check", status.Output.Pass[0].Title)
	assert.Equal(t, "Sufficient storage available", status.Output.Pass[0].Message)
	assert.Equal(t, "Network Check", status.Output.Fail[0].Title)
	assert.Equal(t, "Network connectivity issues", status.Output.Fail[0].Message)
	assert.Equal(t, []string{"Storage Check", "Network Check"}, status.Titles)

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
	status, err = c.GetLinuxInstallAppPreflightsStatus()
	assert.Error(t, err)
	assert.Equal(t, types.InstallAppPreflightsStatusResponse{}, status)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestRunKubernetesAppPreflights(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/kubernetes/install/app-preflights/run", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.InstallAppPreflightsStatusResponse{
			Status: types.Status{
				State:       types.StateRunning,
				Description: "App preflights running on Kubernetes",
			},
			Output: &types.PreflightsOutput{
				Pass: []types.PreflightsRecord{
					{
						Title:   "Cluster Resources",
						Message: "Sufficient cluster resources",
					},
				},
			},
			Titles: []string{"Cluster Resources"},
		})
	}))
	defer server.Close()

	// Test successful run
	c := New(server.URL, WithToken("test-token"))
	status, err := c.RunKubernetesInstallAppPreflights()
	assert.NoError(t, err)
	assert.Equal(t, types.StateRunning, status.Status.State)
	assert.Equal(t, "App preflights running on Kubernetes", status.Status.Description)
	assert.Equal(t, "Cluster Resources", status.Output.Pass[0].Title)
	assert.Equal(t, "Sufficient cluster resources", status.Output.Pass[0].Message)
	assert.Equal(t, []string{"Cluster Resources"}, status.Titles)

	// Test error response
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(types.APIError{
			StatusCode: http.StatusConflict,
			Message:    "App preflights already running",
		})
	}))
	defer errorServer.Close()

	c = New(errorServer.URL, WithToken("test-token"))
	status, err = c.RunKubernetesInstallAppPreflights()
	assert.Error(t, err)
	assert.Equal(t, types.InstallAppPreflightsStatusResponse{}, status)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusConflict, apiErr.StatusCode)
	assert.Equal(t, "App preflights already running", apiErr.Message)
}

func TestGetKubernetesAppPreflightsStatus(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/kubernetes/install/app-preflights/status", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.InstallAppPreflightsStatusResponse{
			Status: types.Status{
				State:       types.StateFailed,
				Description: "App preflights failed on Kubernetes",
			},
			Output: &types.PreflightsOutput{
				Pass: []types.PreflightsRecord{
					{
						Title:   "RBAC Check",
						Message: "Sufficient permissions",
					},
				},
				Fail: []types.PreflightsRecord{
					{
						Title:   "Storage Class",
						Message: "No default storage class found",
					},
				},
			},
			Titles: []string{"RBAC Check", "Storage Class"},
		})
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	status, err := c.GetKubernetesInstallAppPreflightsStatus()
	assert.NoError(t, err)
	assert.Equal(t, types.StateFailed, status.Status.State)
	assert.Equal(t, "App preflights failed on Kubernetes", status.Status.Description)
	assert.Equal(t, "RBAC Check", status.Output.Pass[0].Title)
	assert.Equal(t, "Sufficient permissions", status.Output.Pass[0].Message)
	assert.Equal(t, "Storage Class", status.Output.Fail[0].Title)
	assert.Equal(t, "No default storage class found", status.Output.Fail[0].Message)
	assert.Equal(t, []string{"RBAC Check", "Storage Class"}, status.Titles)

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
	status, err = c.GetKubernetesInstallAppPreflightsStatus()
	assert.Error(t, err)
	assert.Equal(t, types.InstallAppPreflightsStatusResponse{}, status)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestClient_InstallLinuxApp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/linux/install/app/install", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		appInstall := types.AppInstall{
			Status: types.Status{State: types.StateRunning, Description: "Installing app"},
			Logs:   "Installation started\n",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(appInstall)
	}))
	defer server.Close()

	c := New(server.URL, WithToken("test-token"))
	appInstall, err := c.InstallLinuxApp()

	require.NoError(t, err)
	assert.Equal(t, types.StateRunning, appInstall.Status.State)
	assert.Equal(t, "Installing app", appInstall.Status.Description)
	assert.Equal(t, "Installation started\n", appInstall.Logs)
}

func TestClient_GetLinuxAppInstallStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/linux/install/app/status", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		appInstall := types.AppInstall{
			Status: types.Status{State: types.StateSucceeded, Description: "App installed successfully"},
			Logs:   "Installation completed\n",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(appInstall)
	}))
	defer server.Close()

	c := New(server.URL, WithToken("test-token"))
	appInstall, err := c.GetLinuxAppInstallStatus()

	require.NoError(t, err)
	assert.Equal(t, types.StateSucceeded, appInstall.Status.State)
	assert.Equal(t, "App installed successfully", appInstall.Status.Description)
	assert.Equal(t, "Installation completed\n", appInstall.Logs)
}

func TestClient_InstallKubernetesApp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/kubernetes/install/app/install", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		appInstall := types.AppInstall{
			Status: types.Status{State: types.StateRunning, Description: "Installing app"},
			Logs:   "Kubernetes app installation started\n",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(appInstall)
	}))
	defer server.Close()

	c := New(server.URL, WithToken("test-token"))
	appInstall, err := c.InstallKubernetesApp()

	require.NoError(t, err)
	assert.Equal(t, types.StateRunning, appInstall.Status.State)
	assert.Equal(t, "Installing app", appInstall.Status.Description)
	assert.Equal(t, "Kubernetes app installation started\n", appInstall.Logs)
}

func TestClient_GetKubernetesAppInstallStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/kubernetes/install/app/status", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		appInstall := types.AppInstall{
			Status: types.Status{State: types.StateFailed, Description: "App installation failed"},
			Logs:   "Installation failed with error\n",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(appInstall)
	}))
	defer server.Close()

	c := New(server.URL, WithToken("test-token"))
	appInstall, err := c.GetKubernetesAppInstallStatus()

	require.NoError(t, err)
	assert.Equal(t, types.StateFailed, appInstall.Status.State)
	assert.Equal(t, "App installation failed", appInstall.Status.Description)
	assert.Equal(t, "Installation failed with error\n", appInstall.Logs)
}

func TestClient_AppInstallErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiError := types.APIError{
			StatusCode: http.StatusInternalServerError,
			Message:    "Internal server error",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(apiError)
	}))
	defer server.Close()

	c := New(server.URL, WithToken("test-token"))

	t.Run("InstallLinuxApp error", func(t *testing.T) {
		_, err := c.InstallLinuxApp()
		require.Error(t, err)
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok)
		assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
		assert.Equal(t, "Internal server error", apiErr.Message)
	})

	t.Run("GetLinuxAppInstallStatus error", func(t *testing.T) {
		_, err := c.GetLinuxAppInstallStatus()
		require.Error(t, err)
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok)
		assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	})

	t.Run("InstallKubernetesApp error", func(t *testing.T) {
		_, err := c.InstallKubernetesApp()
		require.Error(t, err)
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok)
		assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	})

	t.Run("GetKubernetesAppInstallStatus error", func(t *testing.T) {
		_, err := c.GetKubernetesAppInstallStatus()
		require.Error(t, err)
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok)
		assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	})
}

func TestClient_AppInstallWithoutToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify no auth header is sent
		assert.Empty(t, r.Header.Get("Authorization"))

		apiError := types.APIError{
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(apiError)
	}))
	defer server.Close()

	c := New(server.URL) // No token provided

	t.Run("InstallLinuxApp without token", func(t *testing.T) {
		_, err := c.InstallLinuxApp()
		require.Error(t, err)
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok)
		assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
	})

	t.Run("GetLinuxAppInstallStatus without token", func(t *testing.T) {
		_, err := c.GetLinuxAppInstallStatus()
		require.Error(t, err)
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok)
		assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
	})
}

func TestLinuxGetInstallationStatus(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/linux/install/installation/status", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.Status{
			State:       types.StateSucceeded,
			Description: "Installation successful",
		})
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	status, err := c.GetLinuxInstallationStatus()
	assert.NoError(t, err)
	assert.Equal(t, types.StateSucceeded, status.State)
	assert.Equal(t, "Installation successful", status.Description)

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
	status, err = c.GetLinuxInstallationStatus()
	assert.Error(t, err)
	assert.Equal(t, types.Status{}, status)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestLinuxGetUpgradeAppConfigValues(t *testing.T) {
	// Define expected values once
	expectedValues := types.AppConfigValues{
		"upgrade-key1": types.AppConfigValue{Value: "upgrade-value1"},
		"upgrade-key2": types.AppConfigValue{Value: "upgrade-value2"},
		"upgrade-key3": types.AppConfigValue{Value: "upgrade-value3"},
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/linux/upgrade/app/config/values", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		response := types.AppConfigValuesResponse{Values: expectedValues}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	values, err := c.GetLinuxUpgradeAppConfigValues()
	assert.NoError(t, err)
	assert.Equal(t, expectedValues, values)

	// Test authentication (without token)
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/linux/upgrade/app/config/values", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Empty(t, r.Header.Get("Authorization"))

		// Return unauthorized response
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(types.APIError{
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized",
		})
	}))
	defer authServer.Close()

	c = New(authServer.URL)
	values, err = c.GetLinuxUpgradeAppConfigValues()
	assert.Error(t, err)
	assert.Nil(t, values)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
	assert.Equal(t, "Unauthorized", apiErr.Message)

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
	values, err = c.GetLinuxUpgradeAppConfigValues()
	assert.Error(t, err)
	assert.Nil(t, values)

	apiErr, ok = err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestLinuxPatchUpgradeAppConfigValues(t *testing.T) {
	// Define expected config values once
	expectedValues := types.AppConfigValues{
		"upgrade-item":  types.AppConfigValue{Value: "new-upgrade-value"},
		"required-item": types.AppConfigValue{Value: "required-upgrade-value"},
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method and path
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/api/linux/upgrade/app/config/values", r.URL.Path)

		// Check headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Decode request body
		var req types.PatchAppConfigValuesRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err, "Failed to decode request body")

		// Verify the request contains expected values
		assert.Equal(t, "new-upgrade-value", req.Values["upgrade-item"].Value)
		assert.Equal(t, "required-upgrade-value", req.Values["required-item"].Value)

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.AppConfigValuesResponse{Values: expectedValues})
	}))
	defer server.Close()

	// Test successful patch
	c := New(server.URL, WithToken("test-token"))
	configValues := types.AppConfigValues{
		"upgrade-item":  types.AppConfigValue{Value: "new-upgrade-value"},
		"required-item": types.AppConfigValue{Value: "required-upgrade-value"},
	}
	config, err := c.PatchLinuxUpgradeAppConfigValues(configValues)
	require.NoError(t, err)
	assert.Equal(t, expectedValues, config)

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
	configValues, err = c.PatchLinuxUpgradeAppConfigValues(configValues)
	assert.Error(t, err)
	assert.Equal(t, types.AppConfigValues{}, configValues)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "Bad Request", apiErr.Message)
}

func TestLinuxTemplateUpgradeAppConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/linux/upgrade/app/config/template", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var req types.TemplateAppConfigRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Mock server returns templated results (as if processed by the template engine)
		config := types.AppConfig{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "upgrade-settings",
					Title: "UPGRADE CONFIGURATION",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "upgrade_mode",
							Title:   "Mode: automatic",
							Type:    "text",
							Default: multitype.FromString("automatic"),
							Value:   multitype.FromString("automatic"),
						},
					},
				},
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(config)
	}))
	defer server.Close()

	c := New(server.URL, WithToken("test-token"))
	values := types.AppConfigValues{
		"upgrade_mode": types.AppConfigValue{Value: "automatic"},
	}

	config, err := c.TemplateLinuxUpgradeAppConfig(values)
	require.NoError(t, err)
	assert.Equal(t, "upgrade-settings", config.Groups[0].Name)
	assert.Equal(t, "UPGRADE CONFIGURATION", config.Groups[0].Title)
	assert.Equal(t, "Mode: automatic", config.Groups[0].Items[0].Title)
	assert.Equal(t, "automatic", config.Groups[0].Items[0].Value.StrVal)
}

func TestKubernetesGetUpgradeAppConfigValues(t *testing.T) {
	// Define expected values once
	expectedValues := types.AppConfigValues{
		"k8s-upgrade-key1": types.AppConfigValue{Value: "k8s-upgrade-value1"},
		"k8s-upgrade-key2": types.AppConfigValue{Value: "k8s-upgrade-value2"},
		"k8s-upgrade-key3": types.AppConfigValue{Value: "k8s-upgrade-value3"},
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/kubernetes/upgrade/app/config/values", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		response := types.AppConfigValuesResponse{Values: expectedValues}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	values, err := c.GetKubernetesUpgradeAppConfigValues()
	assert.NoError(t, err)
	assert.Equal(t, expectedValues, values)

	// Test authentication (without token)
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/kubernetes/upgrade/app/config/values", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Empty(t, r.Header.Get("Authorization"))

		// Return unauthorized response
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(types.APIError{
			StatusCode: http.StatusUnauthorized,
			Message:    "Unauthorized",
		})
	}))
	defer authServer.Close()

	c = New(authServer.URL)
	values, err = c.GetKubernetesUpgradeAppConfigValues()
	assert.Error(t, err)
	assert.Nil(t, values)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
	assert.Equal(t, "Unauthorized", apiErr.Message)

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
	values, err = c.GetKubernetesUpgradeAppConfigValues()
	assert.Error(t, err)
	assert.Nil(t, values)

	apiErr, ok = err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestKubernetesPatchUpgradeAppConfigValues(t *testing.T) {
	// Define expected config values once
	expectedValues := types.AppConfigValues{
		"k8s-upgrade-item": types.AppConfigValue{Value: "new-k8s-upgrade-value"},
		"required-item":    types.AppConfigValue{Value: "required-k8s-upgrade-value"},
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request method and path
		assert.Equal(t, "PATCH", r.Method)
		assert.Equal(t, "/api/kubernetes/upgrade/app/config/values", r.URL.Path)

		// Check headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Decode request body
		var req types.PatchAppConfigValuesRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err, "Failed to decode request body")

		// Verify the request contains expected values
		assert.Equal(t, "new-k8s-upgrade-value", req.Values["k8s-upgrade-item"].Value)
		assert.Equal(t, "required-k8s-upgrade-value", req.Values["required-item"].Value)

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(types.AppConfigValuesResponse{Values: expectedValues})
	}))
	defer server.Close()

	// Test successful patch
	c := New(server.URL, WithToken("test-token"))
	configValues := types.AppConfigValues{
		"k8s-upgrade-item": types.AppConfigValue{Value: "new-k8s-upgrade-value"},
		"required-item":    types.AppConfigValue{Value: "required-k8s-upgrade-value"},
	}
	configValuesResponse, err := c.PatchKubernetesUpgradeAppConfigValues(configValues)
	assert.NoError(t, err)
	assert.Equal(t, expectedValues, configValuesResponse)

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
	configValuesResponse, err = c.PatchKubernetesUpgradeAppConfigValues(configValues)
	assert.Error(t, err)
	assert.Equal(t, types.AppConfigValues{}, configValuesResponse)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "Bad Request", apiErr.Message)
}

func TestKubernetesTemplateUpgradeAppConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/kubernetes/upgrade/app/config/template", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var req types.TemplateAppConfigRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Mock server returns templated results (as if processed by the template engine)
		config := types.AppConfig{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "k8s-upgrade-settings",
					Title: "Kubernetes Upgrade Settings",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "k8s_upgrade_strategy",
							Title:   "UPGRADE STRATEGY",
							Type:    "text",
							Default: multitype.FromString("rolling"),
							Value:   multitype.FromString("rolling"),
						},
					},
				},
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(config)
	}))
	defer server.Close()

	c := New(server.URL, WithToken("test-token"))
	values := types.AppConfigValues{
		"k8s_upgrade_strategy": types.AppConfigValue{Value: "rolling"},
	}

	config, err := c.TemplateKubernetesUpgradeAppConfig(values)
	require.NoError(t, err)
	assert.Equal(t, "k8s-upgrade-settings", config.Groups[0].Name)
	assert.Equal(t, "Kubernetes Upgrade Settings", config.Groups[0].Title)
	assert.Equal(t, "UPGRADE STRATEGY", config.Groups[0].Items[0].Title)
	assert.Equal(t, "rolling", config.Groups[0].Items[0].Value.StrVal)
}
