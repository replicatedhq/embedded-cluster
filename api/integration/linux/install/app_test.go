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
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
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

func TestLinuxPatchAppConfigValues(t *testing.T) {
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
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(linuxinstall.StateNew))),
			linuxinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
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
				"file-item":     types.AppConfigValue{Value: "SGVsbG8gV29ybGQ=", Filename: "new-file.txt"},
			},
		}

		reqBodyBytes, err := json.Marshal(patchRequest)
		require.NoError(t, err)

		// Create a request to patch config values
		req := httptest.NewRequest(http.MethodPatch, "/linux/install/app/config/values", bytes.NewReader(reqBodyBytes))
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
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(linuxinstall.StateNew))),
			linuxinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
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
		req := httptest.NewRequest(http.MethodPatch, "/linux/install/app/config/values", bytes.NewReader(reqBodyBytes))
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
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(linuxinstall.StateSucceeded))),
			linuxinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
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
		req := httptest.NewRequest(http.MethodPatch, "/linux/install/app/config/values", bytes.NewReader(reqBodyBytes))
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
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request to patch config values without the required item
		setRequest := types.PatchAppConfigValuesRequest{
			Values: types.AppConfigValues{
				"test-item": types.AppConfigValue{Value: "new-value"},
				// required-item is intentionally omitted
			},
		}

		reqBodyBytes, err := json.Marshal(setRequest)
		require.NoError(t, err)

		// Create a request
		req := httptest.NewRequest(http.MethodPatch, "/linux/install/app/config/values", bytes.NewReader(reqBodyBytes))
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
	installController, err := linuxinstall.NewInstallController(
		linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(linuxinstall.StateNew))),
		linuxinstall.WithReleaseData(&release.ReleaseData{
			AppConfig: &appConfig,
		}),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance := integration.NewAPIWithReleaseData(t,
		api.WithLinuxInstallController(installController),
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

	// Test PatchLinuxAppConfigValues
	t.Run("PatchLinuxAppConfigValues", func(t *testing.T) {
		// Create config values to set
		configValues := types.AppConfigValues{
			"test-item":     types.AppConfigValue{Value: "new-value"},
			"required-item": types.AppConfigValue{Value: "required-value"},
			"file-item":     types.AppConfigValue{Value: "SGVsbG8gV29ybGQ=", Filename: "new-file.txt"},
		}

		// Set the app config values using the client
		config, err := c.PatchLinuxAppConfigValues(configValues)
		require.NoError(t, err, "PatchLinuxAppConfigValues should succeed")

		// Verify the app config values are returned from the store
		assert.Equal(t, "new-value", config["test-item"].Value, "test-item should be updated")
		assert.Equal(t, "required-value", config["required-item"].Value, "required-item should be updated")
		assert.Equal(t, "SGVsbG8gV29ybGQ=", config["file-item"].Value, "file-item value should be updated")
		assert.Equal(t, "new-file.txt", config["file-item"].Filename, "file-item value should contain a filename")
	})

	// Test PatchLinuxAppConfigValues with missing required item
	t.Run("PatchLinuxAppConfigValues missing required", func(t *testing.T) {
		// Create config values without the required item
		configValues := types.AppConfigValues{
			"test-item": types.AppConfigValue{Value: "new-value"},
			// required-item is intentionally omitted
		}

		// Set the app config values using the client
		_, err := c.PatchLinuxAppConfigValues(configValues)
		require.Error(t, err, "PatchLinuxAppConfigValues should fail with missing required item")

		// Check that the error is of correct type
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode, "Error should have BadRequest status code")
		assert.Len(t, apiErr.Errors, 1, "Should have one validation error")
		assert.Equal(t, "required-item", apiErr.Errors[0].Field, "Error should be for required-item field")
		assert.Equal(t, "item is required", apiErr.Errors[0].Message, "Error should indicate item is required")
	})

	// Test PatchLinuxAppConfigValues with invalid state transition
	t.Run("PatchLinuxAppConfigValues invalid state", func(t *testing.T) {
		// Create an install controller in a completed state
		completedInstallController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(linuxinstall.StateSucceeded))),
			linuxinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with the completed install controller
		completedAPIInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(completedInstallController),
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
		_, err = completedClient.PatchLinuxAppConfigValues(configValues)
		require.Error(t, err, "PatchLinuxAppConfigValues should fail with invalid state transition")

		// Check that the error is of correct type
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusConflict, apiErr.StatusCode, "Error should have Conflict status code")
		assert.Contains(t, apiErr.Message, "invalid transition", "Error should mention invalid transition")
	})
}

func TestLinuxTemplateAppConfig(t *testing.T) {
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
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfigWithTemplates,
			}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
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
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/config/template", bytes.NewReader(reqBodyBytes))
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
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfigWithTemplates,
			}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
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
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/config/template", bytes.NewReader(reqBodyBytes))
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
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfigWithTemplates,
			}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
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
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/config/template", bytes.NewReader(reqBodyBytes))
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
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfigWithTemplates,
			}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
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
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/config/template", bytes.NewReader(reqBodyBytes))
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
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfigWithTemplates,
			}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request with invalid JSON
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/config/template", bytes.NewReader([]byte("invalid json")))
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
