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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
							Default: multitype.FromString("default"),
							Value:   multitype.FromString("value"),
						},
					},
				},
			},
		},
	}

	// Create an app config with templates
	appConfigWithTemplates := kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "deployment-config",
					Title: "{{repl print \"Deployment Configuration\" }}",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "cluster-name",
							Type:    "text",
							Title:   "Cluster Name",
							Default: multitype.FromString("production-cluster"),
							Value:   multitype.FromString("production-cluster"),
						},
						{
							Name:    "namespace",
							Type:    "text",
							Title:   "repl{{ upper \"target namespace\" }}",
							Default: multitype.FromString("{{repl printf \"default-%s\" \"namespace\" }}"),
							Value:   multitype.FromString("repl{{ printf \"%s-deployment\" (ConfigOption \"cluster-name\") }}"),
						},
					},
				},
			},
		},
	}

	// Create config values that should be applied to the config
	configValues := types.AppConfigValues{
		"test-item": types.AppConfigValue{Value: "applied-value"},
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

	// Test template processing
	t.Run("Template processing", func(t *testing.T) {
		// Create an install controller with the templated config
		installController, err := kubernetesinstall.NewInstallController(
			kubernetesinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfigWithTemplates,
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

		// Verify the templates were processed
		assert.Equal(t, "Deployment Configuration", response.Groups[0].Title, "group title should be processed from template")
		assert.Equal(t, "Cluster Name", response.Groups[0].Items[0].Title, "first item title should be non-templated")
		assert.Equal(t, "production-cluster", response.Groups[0].Items[0].Value.String(), "first item value should be non-templated")
		assert.Equal(t, "TARGET NAMESPACE", response.Groups[0].Items[1].Title, "templated item title should be processed from template")
		assert.Equal(t, "production-cluster-deployment", response.Groups[0].Items[1].Value.String(), "templated item value should be processed from ConfigOption template")
		assert.Equal(t, "default-namespace", response.Groups[0].Items[1].Default.String(), "templated item default should be processed from template")
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
							Default: multitype.FromString("default"),
							Value:   multitype.FromString("value"),
						},
						{
							Name:    "another-item",
							Type:    "text",
							Title:   "Another Item",
							Default: multitype.FromString("default2"),
							Value:   multitype.FromString("value2"),
						},
						{
							Name:     "required-item",
							Type:     "text",
							Title:    "Required Item",
							Required: true,
						},
						{
							Name:     "required-password",
							Type:     "password",
							Title:    "Required Password",
							Required: true,
						},
						{
							Name:     "file-item",
							Type:     "file",
							Title:    "File Item",
							Filename: "file.txt",
							Default:  multitype.FromString("SGVsbG8="),
							Value:    multitype.FromString("QQ=="),
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

		// Create a request to patch config values
		patchRequest := types.PatchAppConfigValuesRequest{
			Values: types.AppConfigValues{
				"test-item":         types.AppConfigValue{Value: "new-value"},
				"required-item":     types.AppConfigValue{Value: "required-value"},
				"required-password": types.AppConfigValue{Value: "required-password"},
				"file-item":         types.AppConfigValue{Value: "SGVsbG8gV29ybGQ=", Filename: "new-file.txt"},
			},
		}

		reqBodyBytes, err := json.Marshal(patchRequest)
		require.NoError(t, err)

		// Create a request to patch config values
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
		var response types.AppConfigValuesResponse
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		require.NotNil(t, response.Values, "response values should not be nil")

		// Verify the app config values are returned from the store
		assert.Equal(t, "new-value", response.Values["test-item"].Value, "test-item should be updated")
		assert.Equal(t, "required-value", response.Values["required-item"].Value, "required-item should be updated")
		assert.Equal(t, "SGVsbG8gV29ybGQ=", response.Values["file-item"].Value, "file-item value should be updated")
		assert.Equal(t, "new-file.txt", response.Values["file-item"].Filename, "file-item value should contain a filename")
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

		// Create a request to patch config values
		patchRequest := types.PatchAppConfigValuesRequest{
			Values: types.AppConfigValues{
				"test-item":     types.AppConfigValue{Value: "new-value"},
				"required-item": types.AppConfigValue{Value: "required-value"},
			},
		}

		reqBodyBytes, err := json.Marshal(patchRequest)
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

		// Create a request to patch config values
		patchRequest := types.PatchAppConfigValuesRequest{
			Values: types.AppConfigValues{
				"test-item":     types.AppConfigValue{Value: "new-value"},
				"required-item": types.AppConfigValue{Value: "required-value"},
			},
		}

		reqBodyBytes, err := json.Marshal(patchRequest)
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

		// Create a request to patch config values without the required item
		patchRequest := types.PatchAppConfigValuesRequest{
			Values: types.AppConfigValues{
				"test-item": types.AppConfigValue{Value: "new-value"},
				// required-item is intentionally omitted
			},
		}

		reqBodyBytes, err := json.Marshal(patchRequest)
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
		assert.Len(t, apiError.Errors, 2)
		assert.Equal(t, apiError.Errors[0].Field, "required-item")
		assert.Equal(t, apiError.Errors[0].Message, "Required Item is required")
		assert.Equal(t, apiError.Errors[1].Field, "required-password")
		assert.Equal(t, apiError.Errors[1].Message, "Required Password is required")
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
							Default: multitype.FromString("default"),
							Value:   multitype.FromString("value"),
						},
						{
							Name:     "file-item",
							Type:     "file",
							Title:    "File Item",
							Filename: "file.txt",
							Default:  multitype.FromString("SGVsbG8="),
							Value:    multitype.FromString("QQ=="),
						},
					},
				},
			},
		},
	}

	// Create config values that should be applied to the config
	configValues := types.AppConfigValues{
		"test-item": types.AppConfigValue{Value: "applied-value"},
		"file-item": types.AppConfigValue{Value: "SGVsbG8gV29ybGQ=", Filename: "new-file.txt"},
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
							Default: multitype.FromString("default"),
							Value:   multitype.FromString("value"),
						},
						{
							Name:     "required-item",
							Type:     "text",
							Title:    "Required Item",
							Required: true,
						},
						{
							Name:     "file-item",
							Type:     "file",
							Title:    "File Item",
							Filename: "file.txt",
							Default:  multitype.FromString("SGVsbG8="),
							Value:    multitype.FromString("QQ=="),
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
		configValues := types.AppConfigValues{
			"test-item":     types.AppConfigValue{Value: "new-value"},
			"required-item": types.AppConfigValue{Value: "required-value"},
			"file-item":     types.AppConfigValue{Value: "SGVsbG8gV29ybGQ=", Filename: "new-file.txt"},
		}

		// Set the app config values using the client
		config, err := c.PatchKubernetesAppConfigValues(configValues)
		require.NoError(t, err, "PatchKubernetesAppConfigValues should succeed")

		// Verify the config values are returned
		assert.Equal(t, "new-value", config["test-item"].Value, "new value for test-item should be returned")
		assert.Equal(t, "required-value", config["required-item"].Value, "new value for required-item should be returned")
		assert.Equal(t, "SGVsbG8gV29ybGQ=", config["file-item"].Value, "new value for file-item should be returned")
		assert.Equal(t, "new-file.txt", config["file-item"].Filename, "file-item value should contain a filename")
	})

	// Test PatchKubernetesAppConfigValues with missing required item
	t.Run("PatchKubernetesAppConfigValues missing required", func(t *testing.T) {
		// Create config values without the required item
		configValues := types.AppConfigValues{
			"test-item": types.AppConfigValue{Value: "new-value"},
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
		assert.Equal(t, "Required Item is required", apiErr.Errors[0].Message, "Error should indicate item is required")
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
		configValues := types.AppConfigValues{
			"test-item":     types.AppConfigValue{Value: "new-value"},
			"required-item": types.AppConfigValue{Value: "required-value"},
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
							Default: multitype.FromString("default"),
							Value:   multitype.FromString("value"),
						},
						{
							Name:     "file-item",
							Type:     "file",
							Title:    "File Item",
							Filename: "file.txt",
							Default:  multitype.FromString("SGVsbG8="),
							Value:    multitype.FromString("QQ=="),
						},
					},
				},
			},
		},
	}

	// Create config values that should be applied to the config
	configValues := types.AppConfigValues{
		"test-item": types.AppConfigValue{Value: "applied-value"},
		"file-item": types.AppConfigValue{Value: "SGVsbG8gV29ybGQ=", Filename: "new-file.txt"},
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

func TestKubernetesTemplateAppConfig(t *testing.T) {
	// Create an app config with realistic templates
	appConfigWithTemplates := kotsv1beta1.Config{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kots.io/v1beta1",
			Kind:       "Config",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "template-config",
		},
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "database",
					Title: "{{repl print \"Database Configuration\" }}",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "db_enabled",
							Type:    "bool",
							Title:   "Enable Database",
							Default: multitype.FromString("true"),
							Value:   multitype.FromString("true"),
						},
						{
							Name:    "db_type",
							Type:    "select_one",
							Title:   "Database Type",
							Default: multitype.FromString("mysql"),
							Value:   multitype.FromString("mysql"),
						},
						{
							Name:    "db_host",
							Type:    "text",
							Title:   "{{repl printf \"Database Host (%s)\" (ConfigOption \"db_type\") }}",
							Default: multitype.FromString("localhost"),
							Value:   multitype.FromString("{{repl ConfigOption \"db_type\" }}.example.com"),
							When:    "{{repl ConfigOptionEquals \"db_enabled\" \"true\" }}",
						},
						{
							Name:    "db_port",
							Type:    "text",
							Title:   "Database Port",
							Default: multitype.FromString("{{repl if ConfigOptionEquals \"db_type\" \"mysql\" }}3306{{repl else }}5432{{repl end }}"),
							Value:   multitype.FromString("{{repl if ConfigOptionEquals \"db_type\" \"mysql\" }}3306{{repl else }}5432{{repl end }}"),
							When:    "{{repl ConfigOptionEquals \"db_enabled\" \"true\" }}",
						},
					},
				},
				{
					Name:  "optional_features",
					Title: "Optional Features",
					When:  "{{repl ConfigOptionEquals \"db_enabled\" \"true\" }}",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "feature_enabled",
							Type:    "bool",
							Title:   "Enable Advanced Features",
							Default: multitype.FromString("false"),
							Value:   multitype.FromString("false"),
						},
					},
				},
			},
		},
	}

	// Test successful template processing with default values
	t.Run("Success with default values", func(t *testing.T) {
		// Create an install controller with the templated config
		installController, err := kubernetesinstall.NewInstallController(
			kubernetesinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfigWithTemplates,
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

		// Create template request with default values (empty user values)
		templateRequest := types.TemplateAppConfigRequest{
			Values: types.AppConfigValues{},
		}

		reqBodyBytes, err := json.Marshal(templateRequest)
		require.NoError(t, err)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/kubernetes/install/app/config/template", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var response types.AppConfig
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify the templates were processed with default values
		require.Len(t, response.Groups, 2, "both groups should be present when db_enabled is true")

		// Check database group
		dbGroup := response.Groups[0]
		assert.Equal(t, "Database Configuration", dbGroup.Title, "group title should be templated")
		require.Len(t, dbGroup.Items, 4, "all database items should be present")

		// Check db_enabled item
		assert.Equal(t, "db_enabled", dbGroup.Items[0].Name)
		assert.Equal(t, "true", dbGroup.Items[0].Value.String())

		// Check db_type item
		assert.Equal(t, "db_type", dbGroup.Items[1].Name)
		assert.Equal(t, "mysql", dbGroup.Items[1].Value.String())

		// Check db_host item (uses ConfigOption template)
		assert.Equal(t, "db_host", dbGroup.Items[2].Name)
		assert.Equal(t, "Database Host (mysql)", dbGroup.Items[2].Title, "title should be templated with db_type")
		assert.Equal(t, "mysql.example.com", dbGroup.Items[2].Value.String(), "value should be templated with db_type")

		// Check db_port item (uses conditional template)
		assert.Equal(t, "db_port", dbGroup.Items[3].Name)
		assert.Equal(t, "3306", dbGroup.Items[3].Value.String(), "should be mysql port")
		assert.Equal(t, "3306", dbGroup.Items[3].Default.String(), "default should be mysql port")

		// Check optional features group
		optionalGroup := response.Groups[1]
		assert.Equal(t, "Optional Features", optionalGroup.Title)
		require.Len(t, optionalGroup.Items, 1)
		assert.Equal(t, "feature_enabled", optionalGroup.Items[0].Name)
		assert.Equal(t, "false", optionalGroup.Items[0].Value.String())
	})

	// Test template processing with user-provided values
	t.Run("Success with user values", func(t *testing.T) {
		// Create an install controller with the templated config
		installController, err := kubernetesinstall.NewInstallController(
			kubernetesinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfigWithTemplates,
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

		// Create template request with user values
		templateRequest := types.TemplateAppConfigRequest{
			Values: types.AppConfigValues{
				"db_enabled": {Value: "true"},
				"db_type":    {Value: "postgresql"},
				"db_host":    {Value: "custom-host"},
				"db_port":    {Value: "9999"},
			},
		}

		reqBodyBytes, err := json.Marshal(templateRequest)
		require.NoError(t, err)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/kubernetes/install/app/config/template", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var response types.AppConfig
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify the templates were processed with user values
		require.Len(t, response.Groups, 2, "both groups should be present when db_enabled is true")

		// Check database group
		dbGroup := response.Groups[0]
		assert.Equal(t, "Database Configuration", dbGroup.Title, "group title should be templated")
		require.Len(t, dbGroup.Items, 4, "all database items should be present")

		// Check db_enabled item - config value remains unchanged
		assert.Equal(t, "db_enabled", dbGroup.Items[0].Name)
		assert.Equal(t, "true", dbGroup.Items[0].Value.String(), "config value should remain unchanged")

		// Check db_type item - config value remains unchanged
		assert.Equal(t, "db_type", dbGroup.Items[1].Name)
		assert.Equal(t, "mysql", dbGroup.Items[1].Value.String(), "config value should remain unchanged")

		// Check db_host item - template uses user-provided db_type value
		assert.Equal(t, "db_host", dbGroup.Items[2].Name)
		assert.Equal(t, "Database Host (postgresql)", dbGroup.Items[2].Title, "title should use user-provided db_type")
		assert.Equal(t, "postgresql.example.com", dbGroup.Items[2].Value.String(), "value should use user-provided db_type")

		// Check db_port item - conditional template uses user-provided db_type
		assert.Equal(t, "db_port", dbGroup.Items[3].Name)
		assert.Equal(t, "5432", dbGroup.Items[3].Value.String(), "should be postgresql port based on user db_type")
		assert.Equal(t, "5432", dbGroup.Items[3].Default.String(), "default should be postgresql port")
	})

	// Test with db_enabled=false to verify conditional filtering
	t.Run("Success with db disabled", func(t *testing.T) {
		// Create an install controller with the templated config
		installController, err := kubernetesinstall.NewInstallController(
			kubernetesinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfigWithTemplates,
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

		// Create template request with db disabled
		templateRequest := types.TemplateAppConfigRequest{
			Values: types.AppConfigValues{
				"db_enabled": {Value: "false"},
			},
		}

		reqBodyBytes, err := json.Marshal(templateRequest)
		require.NoError(t, err)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/kubernetes/install/app/config/template", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var response types.AppConfig
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify conditional filtering works
		require.Len(t, response.Groups, 1, "only one group should remain when db_enabled is false")

		// Check that only db_enabled and db_type items remain (no When conditions)
		dbGroup := response.Groups[0]
		assert.Equal(t, "Database Configuration", dbGroup.Title)
		require.Len(t, dbGroup.Items, 2, "only items without When conditions should remain")

		assert.Equal(t, "db_enabled", dbGroup.Items[0].Name)
		assert.Equal(t, "db_type", dbGroup.Items[1].Name)
	})

	// Test authorization error
	t.Run("Authorization error", func(t *testing.T) {
		// Create an install controller with the templated config
		installController, err := kubernetesinstall.NewInstallController(
			kubernetesinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfigWithTemplates,
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

		// Create template request
		templateRequest := types.TemplateAppConfigRequest{
			Values: types.AppConfigValues{},
		}

		reqBodyBytes, err := json.Marshal(templateRequest)
		require.NoError(t, err)

		// Create a request with invalid token
		req := httptest.NewRequest(http.MethodPost, "/kubernetes/install/app/config/template", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer INVALID_TOKEN")
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

	// Test invalid JSON request
	t.Run("Invalid JSON request", func(t *testing.T) {
		// Create an install controller with the templated config
		installController, err := kubernetesinstall.NewInstallController(
			kubernetesinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfigWithTemplates,
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

		// Create a request with invalid JSON
		req := httptest.NewRequest(http.MethodPost, "/kubernetes/install/app/config/template", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer TOKEN")
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
	})
}
