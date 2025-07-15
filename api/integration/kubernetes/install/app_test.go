package install

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	apiclient "github.com/replicatedhq/embedded-cluster/api/client"
	kubernetesinstall "github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/install"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKubernetesGetAppConfig(t *testing.T) {
	// Create an app config
	appConfig := kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
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
		},
	}

	// Create config values that should be applied to the config
	configValues := map[string]string{
		"test-item": "applied-value",
	}

	// Create an install controller with the config values
	installController, err := kubernetesinstall.NewInstallController(
		kubernetesinstall.WithConfigValues(configValues),
		kubernetesinstall.WithReleaseData(&release.ReleaseData{
			AppConfig: &appConfig,
		}),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance := integration.NewAPIWithReleaseData(t,
		api.WithKubernetesInstallController(installController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test successful get
	t.Run("Success", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/kubernetes/install/app/config", nil)
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var response types.AppConfig
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify the raw app config is returned
		assert.Equal(t, response.Groups[0].Items[0].Value.String(), "value", "app config should return raw config schema without values applied")
	})

	// Test authorization
	t.Run("Authorization error", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/kubernetes/install/app/config", nil)
		req.Header.Set("Authorization", "Bearer "+"NOT_A_TOKEN")
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
}

func TestKubernetesPatchAppConfigValues(t *testing.T) {
	// Create an app config
	appConfig := kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
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
						{
							Name:    "another-item",
							Type:    "text",
							Title:   "Another Item",
							Default: multitype.BoolOrString{StrVal: "default2"},
							Value:   multitype.BoolOrString{StrVal: "value2"},
						},
						{
							Name:     "required-item",
							Type:     "text",
							Title:    "Required Item",
							Required: true,
						},
					},
				},
			},
		},
	}

	// Test successful set and get
	t.Run("Success", func(t *testing.T) {
		// Create an install controller with the app config
		installController, err := kubernetesinstall.NewInstallController(
			kubernetesinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithKubernetesInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request to set config values
		setRequest := types.PatchAppConfigValuesRequest{
			Values: map[string]string{
				"test-item":     "new-value",
				"required-item": "required-value",
			},
		}

		reqBodyBytes, err := json.Marshal(setRequest)
		require.NoError(t, err)

		// Create a request to set config values
		req := httptest.NewRequest(http.MethodPatch, "/kubernetes/install/app/config/values", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var response types.AppConfig
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify the raw app config is returned
		assert.Equal(t, "value", response.Groups[0].Items[0].Value.String(), "first item should return raw config schema value")
		assert.Equal(t, "value2", response.Groups[0].Items[1].Value.String(), "second item should return raw config schema value")
	})

	// Test authorization
	t.Run("Authorization error", func(t *testing.T) {
		// Create an install controller with the app config
		installController, err := kubernetesinstall.NewInstallController(
			kubernetesinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithKubernetesInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request to set config values
		setRequest := types.PatchAppConfigValuesRequest{
			Values: map[string]string{
				"test-item":     "new-value",
				"required-item": "required-value",
			},
		}

		reqBodyBytes, err := json.Marshal(setRequest)
		require.NoError(t, err)

		// Create a request with invalid token
		req := httptest.NewRequest(http.MethodPatch, "/kubernetes/install/app/config/values", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+"NOT_A_TOKEN")
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

	// Test invalid state transition
	t.Run("Invalid state transition", func(t *testing.T) {
		// Create an install controller with the app config
		installController, err := kubernetesinstall.NewInstallController(
			kubernetesinstall.WithStateMachine(kubernetesinstall.NewStateMachine(kubernetesinstall.WithCurrentState(kubernetesinstall.StateSucceeded))),
			kubernetesinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithKubernetesInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request to set config values
		setRequest := types.PatchAppConfigValuesRequest{
			Values: map[string]string{
				"test-item":     "new-value",
				"required-item": "required-value",
			},
		}

		reqBodyBytes, err := json.Marshal(setRequest)
		require.NoError(t, err)

		// Create a request
		req := httptest.NewRequest(http.MethodPatch, "/kubernetes/install/app/config/values", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusConflict, apiError.StatusCode)
		assert.Contains(t, apiError.Message, "invalid transition")
	})

	// Test missing required item
	t.Run("Missing required item", func(t *testing.T) {
		// Create an install controller with the app config
		installController, err := kubernetesinstall.NewInstallController(
			kubernetesinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithKubernetesInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request to set config values without the required item
		setRequest := types.PatchAppConfigValuesRequest{
			Values: map[string]string{
				"test-item": "new-value",
				// required-item is intentionally omitted
			},
		}

		reqBodyBytes, err := json.Marshal(setRequest)
		require.NoError(t, err)

		// Create a request
		req := httptest.NewRequest(http.MethodPatch, "/kubernetes/install/app/config/values", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, apiError.StatusCode)
		assert.Len(t, apiError.Errors, 1)
		assert.Equal(t, apiError.Errors[0].Field, "required-item")
		assert.Equal(t, apiError.Errors[0].Message, "item is required")
	})
}

func TestKubernetesGetAppConfigValues(t *testing.T) {
	// Create an app config
	appConfig := kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
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
		},
	}

	// Create config values that should be applied to the config
	configValues := map[string]string{
		"test-item": "applied-value",
	}

	// Create an install controller with the config values
	installController, err := kubernetesinstall.NewInstallController(
		kubernetesinstall.WithConfigValues(configValues),
		kubernetesinstall.WithReleaseData(&release.ReleaseData{
			AppConfig: &appConfig,
		}),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance := integration.NewAPIWithReleaseData(t,
		api.WithKubernetesInstallController(installController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test successful get
	t.Run("Success", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/kubernetes/install/app/config/values", nil)
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var response types.AppConfigValuesResponse
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify the app config values are returned from the store
		assert.Equal(t, configValues, response.Values, "app config values should be returned from store")
	})

	// Test authorization
	t.Run("Authorization error", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/kubernetes/install/app/config/values", nil)
		req.Header.Set("Authorization", "Bearer "+"NOT_A_TOKEN")
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
}

// TestInstallController_PatchAppConfigValuesWithAPIClient tests the PatchAppConfigValues endpoint using the API client
func TestInstallController_PatchAppConfigValuesWithAPIClient(t *testing.T) {
	// Create an app config
	appConfig := kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
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
						{
							Name:     "required-item",
							Type:     "text",
							Title:    "Required Item",
							Required: true,
						},
					},
				},
			},
		},
	}

	// Create an install controller with the app config
	installController, err := kubernetesinstall.NewInstallController(
		kubernetesinstall.WithStateMachine(kubernetesinstall.NewStateMachine(kubernetesinstall.WithCurrentState(kubernetesinstall.StateNew))),
		kubernetesinstall.WithReleaseData(&release.ReleaseData{
			AppConfig: &appConfig,
		}),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance := integration.NewAPIWithReleaseData(t,
		api.WithKubernetesInstallController(installController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)
	require.NoError(t, err)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router.PathPrefix("/api").Subrouter())

	// Create a test server using the router
	server := httptest.NewServer(router)
	defer server.Close()

	// Create client with the predefined token
	c := apiclient.New(server.URL, apiclient.WithToken("TOKEN"))

	// Test PatchKubernetesAppConfigValues
	t.Run("PatchKubernetesAppConfigValues", func(t *testing.T) {
		// Create config values to set
		configValues := map[string]string{
			"test-item":     "new-value",
			"required-item": "required-value",
		}

		// Set the app config values using the client
		config, err := c.PatchKubernetesAppConfigValues(configValues)
		require.NoError(t, err, "PatchKubernetesAppConfigValues should succeed")

		// Verify the raw app config is returned (not the applied values)
		assert.Equal(t, "value", config.Groups[0].Items[0].Value.String(), "first item should return raw config schema value")
		assert.Equal(t, "", config.Groups[0].Items[1].Value.String(), "second item should return empty value since it has no default")
	})

	// Test PatchKubernetesAppConfigValues with missing required item
	t.Run("PatchKubernetesAppConfigValues missing required", func(t *testing.T) {
		// Create config values without the required item
		configValues := map[string]string{
			"test-item": "new-value",
			// required-item is intentionally omitted
		}

		// Set the app config values using the client
		_, err := c.PatchKubernetesAppConfigValues(configValues)
		require.Error(t, err, "PatchKubernetesAppConfigValues should fail with missing required item")

		// Check that the error is of correct type
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode, "Error should have BadRequest status code")
		assert.Len(t, apiErr.Errors, 1, "Should have one validation error")
		assert.Equal(t, "required-item", apiErr.Errors[0].Field, "Error should be for required-item field")
		assert.Equal(t, "item is required", apiErr.Errors[0].Message, "Error should indicate item is required")
	})

	// Test PatchKubernetesAppConfigValues with invalid state transition
	t.Run("PatchKubernetesAppConfigValues invalid state", func(t *testing.T) {
		// Create an install controller in a completed state
		completedInstallController, err := kubernetesinstall.NewInstallController(
			kubernetesinstall.WithStateMachine(kubernetesinstall.NewStateMachine(kubernetesinstall.WithCurrentState(kubernetesinstall.StateSucceeded))),
			kubernetesinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with the completed install controller
		completedAPIInstance := integration.NewAPIWithReleaseData(t,
			api.WithKubernetesInstallController(completedInstallController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		completedRouter := mux.NewRouter()
		completedAPIInstance.RegisterRoutes(completedRouter.PathPrefix("/api").Subrouter())

		// Create a test server using the router
		completedServer := httptest.NewServer(completedRouter)
		defer completedServer.Close()

		// Create client with the predefined token
		completedClient := apiclient.New(completedServer.URL, apiclient.WithToken("TOKEN"))

		// Create config values to set
		configValues := map[string]string{
			"test-item":     "new-value",
			"required-item": "required-value",
		}

		// Set the app config values using the client
		_, err = completedClient.PatchKubernetesAppConfigValues(configValues)
		require.Error(t, err, "PatchKubernetesAppConfigValues should fail with invalid state transition")

		// Check that the error is of correct type
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusConflict, apiErr.StatusCode, "Error should have Conflict status code")
		assert.Contains(t, apiErr.Message, "invalid transition", "Error should mention invalid transition")
	})
}

// TestInstallController_GetAppConfigValuesWithAPIClient tests the GetAppConfigValues endpoint using the API client
func TestInstallController_GetAppConfigValuesWithAPIClient(t *testing.T) {
	// Create an app config
	appConfig := kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
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
		},
	}

	// Create config values that should be applied to the config
	configValues := map[string]string{
		"test-item": "applied-value",
	}

	// Create an install controller with the config values
	installController, err := kubernetesinstall.NewInstallController(
		kubernetesinstall.WithConfigValues(configValues),
		kubernetesinstall.WithReleaseData(&release.ReleaseData{
			AppConfig: &appConfig,
		}),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance := integration.NewAPIWithReleaseData(t,
		api.WithKubernetesInstallController(installController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)
	require.NoError(t, err)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router.PathPrefix("/api").Subrouter())

	// Create a test server using the router
	server := httptest.NewServer(router)
	defer server.Close()

	// Create client with the predefined token
	c := apiclient.New(server.URL, apiclient.WithToken("TOKEN"))

	// Test GetKubernetesAppConfigValues
	t.Run("GetKubernetesAppConfigValues", func(t *testing.T) {
		// Get the app config values using the client
		values, err := c.GetKubernetesAppConfigValues()
		require.NoError(t, err, "GetKubernetesAppConfigValues should succeed")

		// Verify the app config values are returned from the store
		assert.Equal(t, configValues, values, "app config values should be returned from store")
	})

	// Test GetKubernetesAppConfigValues with invalid token
	t.Run("GetKubernetesAppConfigValues unauthorized", func(t *testing.T) {
		// Create client with invalid token
		invalidClient := apiclient.New(server.URL, apiclient.WithToken("INVALID_TOKEN"))

		// Get the app config values using the client
		_, err := invalidClient.GetKubernetesAppConfigValues()
		require.Error(t, err, "GetKubernetesAppConfigValues should fail with invalid token")

		// Check that the error is of correct type
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode, "Error should have Unauthorized status code")
	})
}
