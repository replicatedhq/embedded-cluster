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
		json.NewEncoder(w).Encode(types.LinuxInstallationConfig{
			GlobalCIDR:       "10.0.0.0/24",
			AdminConsolePort: 8080,
		})
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	config, err := c.GetLinuxInstallationConfig()
	assert.NoError(t, err)
	assert.Equal(t, "10.0.0.0/24", config.GlobalCIDR)
	assert.Equal(t, 8080, config.AdminConsolePort)

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
	assert.Equal(t, types.LinuxInstallationConfig{}, config)

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
		json.NewEncoder(w).Encode(types.KubernetesInstallationConfig{
			HTTPProxy:        "http://proxy.example.com",
			HTTPSProxy:       "https://proxy.example.com",
			NoProxy:          "localhost,127.0.0.1",
			AdminConsolePort: 8080,
		})
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	config, err := c.GetKubernetesInstallationConfig()
	assert.NoError(t, err)
	assert.Equal(t, "http://proxy.example.com", config.HTTPProxy)
	assert.Equal(t, "https://proxy.example.com", config.HTTPSProxy)
	assert.Equal(t, "localhost,127.0.0.1", config.NoProxy)
	assert.Equal(t, 8080, config.AdminConsolePort)

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
	assert.Equal(t, types.KubernetesInstallationConfig{}, config)

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

func TestLinuxGetAppConfig(t *testing.T) {
	// Define expected config once
	expectedConfig := types.AppConfig{
		Groups: []kotsv1beta1.ConfigGroup{
			{
				Name:  "test-group",
				Title: "Test Group",
				Items: []kotsv1beta1.ConfigItem{
					{
						Name:    "test-item",
						Type:    "text",
						Title:   "Test Item",
						Default: multitype.BoolOrString{StrVal: "default"},
						Value:   multitype.BoolOrString{StrVal: "value"},
					},
				},
			},
		},
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/linux/install/app/config", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expectedConfig)
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	config, err := c.GetLinuxAppConfig()
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, config)

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
	config, err = c.GetLinuxAppConfig()
	assert.Error(t, err)
	assert.Equal(t, types.AppConfig{}, config)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestKubernetesGetAppConfig(t *testing.T) {
	// Define expected config once
	expectedConfig := types.AppConfig{
		Groups: []kotsv1beta1.ConfigGroup{
			{
				Name:  "test-group",
				Title: "Test Group",
				Items: []kotsv1beta1.ConfigItem{
					{
						Name:    "test-item",
						Type:    "text",
						Title:   "Test Item",
						Default: multitype.BoolOrString{StrVal: "default"},
						Value:   multitype.BoolOrString{StrVal: "value"},
					},
				},
			},
		},
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/kubernetes/install/app/config", r.URL.Path)

		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expectedConfig)
	}))
	defer server.Close()

	// Test successful get
	c := New(server.URL, WithToken("test-token"))
	config, err := c.GetKubernetesAppConfig()
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, config)

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
	config, err = c.GetKubernetesAppConfig()
	assert.Error(t, err)
	assert.Equal(t, types.AppConfig{}, config)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestLinuxGetAppConfigValues(t *testing.T) {
	// Define expected values once
	expectedValues := map[string]string{
		"test-key1": "test-value1",
		"test-key2": "test-value2",
		"test-key3": "test-value3",
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
	values, err := c.GetLinuxAppConfigValues()
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
	values, err = c.GetLinuxAppConfigValues()
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
	values, err = c.GetLinuxAppConfigValues()
	assert.Error(t, err)
	assert.Nil(t, values)

	apiErr, ok = err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestKubernetesGetAppConfigValues(t *testing.T) {
	// Define expected values once
	expectedValues := map[string]string{
		"test-key1": "test-value1",
		"test-key2": "test-value2",
		"test-key3": "test-value3",
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
	values, err := c.GetKubernetesAppConfigValues()
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
	values, err = c.GetKubernetesAppConfigValues()
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
	values, err = c.GetKubernetesAppConfigValues()
	assert.Error(t, err)
	assert.Nil(t, values)

	apiErr, ok = err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	assert.Equal(t, "Internal Server Error", apiErr.Message)
}

func TestLinuxPatchAppConfigValues(t *testing.T) {
	// Define expected config once
	expectedConfig := types.AppConfig{
		Groups: []kotsv1beta1.ConfigGroup{
			{
				Name:  "test-group",
				Title: "Test Group",
				Items: []kotsv1beta1.ConfigItem{
					{
						Name:    "test-item",
						Type:    "text",
						Title:   "Test Item",
						Default: multitype.BoolOrString{StrVal: "default"},
						Value:   multitype.BoolOrString{StrVal: "value"},
					},
				},
			},
		},
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
		assert.Equal(t, "new-value", req.Values["test-item"])
		assert.Equal(t, "required-value", req.Values["required-item"])

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expectedConfig)
	}))
	defer server.Close()

	// Test successful set
	c := New(server.URL, WithToken("test-token"))
	configValues := map[string]string{
		"test-item":     "new-value",
		"required-item": "required-value",
	}
	config, err := c.PatchLinuxAppConfigValues(configValues)
	require.NoError(t, err)
	assert.Equal(t, expectedConfig, config)

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
	config, err = c.PatchLinuxAppConfigValues(configValues)
	assert.Error(t, err)
	assert.Equal(t, types.AppConfig{}, config)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "Bad Request", apiErr.Message)
}

func TestKubernetesPatchAppConfigValues(t *testing.T) {
	// Define expected config once
	expectedConfig := types.AppConfig{
		Groups: []kotsv1beta1.ConfigGroup{
			{
				Name:  "test-group",
				Title: "Test Group",
				Items: []kotsv1beta1.ConfigItem{
					{
						Name:    "test-item",
						Type:    "text",
						Title:   "Test Item",
						Default: multitype.BoolOrString{StrVal: "default"},
						Value:   multitype.BoolOrString{StrVal: "value"},
					},
				},
			},
		},
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
		assert.Equal(t, "new-value", req.Values["test-item"])
		assert.Equal(t, "required-value", req.Values["required-item"])

		// Return successful response
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expectedConfig)
	}))
	defer server.Close()

	// Test successful set
	c := New(server.URL, WithToken("test-token"))
	configValues := map[string]string{
		"test-item":     "new-value",
		"required-item": "required-value",
	}
	config, err := c.PatchKubernetesAppConfigValues(configValues)
	assert.NoError(t, err)
	assert.Equal(t, expectedConfig, config)

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
	config, err = c.PatchKubernetesAppConfigValues(configValues)
	assert.Error(t, err)
	assert.Equal(t, types.AppConfig{}, config)

	apiErr, ok := err.(*types.APIError)
	require.True(t, ok, "Expected err to be of type *types.APIError")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "Bad Request", apiErr.Message)
}
