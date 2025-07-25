package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	kubernetesinstall "github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/install"
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AppInstallTestSuite struct {
	suite.Suite
	installType string
	createAPI   func(t *testing.T, initialState statemachine.State, rc *release.ReleaseData, configValues types.AppConfigValues) *api.API
	router      *mux.Router
	baseURL     string
}

func (s *AppInstallTestSuite) TestGetAppConfigValues() {
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

	// Create an install controller with the app config
	apiInstance := s.createAPI(s.T(), states.StateNew, &release.ReleaseData{
		AppConfig: &appConfig,
	}, configValues)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test successful get
	s.T().Run("Success", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, s.baseURL+"/app/config/values", nil)
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var response types.AppConfigValuesResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify the app config values are returned from the store
		assert.Equal(t, configValues, response.Values, "app config values should be returned from store")
	})

	// Test authorization
	s.T().Run("Authorization error", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, s.baseURL+"/app/config/values", nil)
		req.Header.Set("Authorization", "Bearer "+"NOT_A_TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var apiError types.APIError
		err := json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, apiError.StatusCode)
	})
}

func (s *AppInstallTestSuite) TestPatchAppConfigValues() {
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
	s.T().Run("Success", func(t *testing.T) {
		// Create an install controller with the app config
		apiInstance := s.createAPI(t, states.StateNew, &release.ReleaseData{
			AppConfig: &appConfig,
		}, nil)

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
		req := httptest.NewRequest(http.MethodPatch, s.baseURL+"/app/config/values", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)

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
	s.T().Run("Authorization error", func(t *testing.T) {
		// Create an install controller with the app config
		apiInstance := s.createAPI(t, states.StateNew, &release.ReleaseData{
			AppConfig: &appConfig,
		}, nil)

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
		req := httptest.NewRequest(http.MethodPatch, s.baseURL+"/app/config/values", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+"NOT_A_TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusUnauthorized, rec.Code)

		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, apiError.StatusCode)
	})

	// Test invalid state transition
	s.T().Run("Invalid state transition", func(t *testing.T) {
		// Create an install controller with the app config
		apiInstance := s.createAPI(t, states.StateSucceeded, &release.ReleaseData{
			AppConfig: &appConfig,
		}, nil)

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
		req := httptest.NewRequest(http.MethodPatch, s.baseURL+"/app/config/values", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusConflict, rec.Code)

		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusConflict, apiError.StatusCode)
		assert.Contains(t, apiError.Message, "invalid transition")
	})

	// Test missing required item
	s.T().Run("Missing required item", func(t *testing.T) {
		// Create an install controller with the app config
		apiInstance := s.createAPI(t, states.StateNew, &release.ReleaseData{
			AppConfig: &appConfig,
		}, nil)

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
		req := httptest.NewRequest(http.MethodPatch, s.baseURL+"/app/config/values", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+"TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusBadRequest, rec.Code)

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

func (s *AppInstallTestSuite) TestTemplateAppConfig() {
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
	s.T().Run("Success with default values", func(t *testing.T) {
		// Create an install controller with the app config
		apiInstance := s.createAPI(t, states.StateNew, &release.ReleaseData{
			AppConfig: &appConfigWithTemplates,
		}, nil)

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
		req := httptest.NewRequest(http.MethodPost, s.baseURL+"/app/config/template", bytes.NewReader(reqBodyBytes))
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
	s.T().Run("Success with user values", func(t *testing.T) {
		// Create an install controller with the app config
		apiInstance := s.createAPI(t, states.StateNew, &release.ReleaseData{
			AppConfig: &appConfigWithTemplates,
		}, nil)

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
		req := httptest.NewRequest(http.MethodPost, s.baseURL+"/app/config/template", bytes.NewReader(reqBodyBytes))
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

		// Check db_type item - user value takes precedence over config value
		assert.Equal(t, "db_type", dbGroup.Items[1].Name)
		assert.Equal(t, "postgresql", dbGroup.Items[1].Value.String(), "user value should take precedence over config value")

		// Check db_host item - user value takes precedence over templated config value
		assert.Equal(t, "db_host", dbGroup.Items[2].Name)
		assert.Equal(t, "Database Host (postgresql)", dbGroup.Items[2].Title, "title should use user-provided db_type")
		assert.Equal(t, "custom-host", dbGroup.Items[2].Value.String(), "user value should take precedence over templated config value")

		// Check db_port item - user value takes precedence over templated config value
		assert.Equal(t, "db_port", dbGroup.Items[3].Name)
		assert.Equal(t, "9999", dbGroup.Items[3].Value.String(), "user value should take precedence over templated config value")
		assert.Equal(t, "5432", dbGroup.Items[3].Default.String(), "default should be postgresql port")
	})

	// Test with db_enabled=false to verify conditional filtering
	s.T().Run("Success with db disabled", func(t *testing.T) {
		// Create an install controller with the app config
		apiInstance := s.createAPI(t, states.StateNew, &release.ReleaseData{
			AppConfig: &appConfigWithTemplates,
		}, nil)

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
		req := httptest.NewRequest(http.MethodPost, s.baseURL+"/app/config/template", bytes.NewReader(reqBodyBytes))
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
	s.T().Run("Authorization error", func(t *testing.T) {
		// Create an install controller with the app config
		apiInstance := s.createAPI(t, states.StateNew, &release.ReleaseData{
			AppConfig: &appConfigWithTemplates,
		}, nil)

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
		req := httptest.NewRequest(http.MethodPost, s.baseURL+"/app/config/template", bytes.NewReader(reqBodyBytes))
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
	s.T().Run("Invalid JSON request", func(t *testing.T) {
		// Create an install controller with the app config
		apiInstance := s.createAPI(t, states.StateNew, &release.ReleaseData{
			AppConfig: &appConfigWithTemplates,
		}, nil)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request with invalid JSON
		req := httptest.NewRequest(http.MethodPost, s.baseURL+"/app/config/template", bytes.NewReader([]byte("invalid json")))
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
		err := json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, apiError.StatusCode)
	})
}

// Runner function that executes the suite for both install types
func TestAppInstallSuite(t *testing.T) {
	installTypes := []struct {
		name        string
		installType string
		createAPI   func(t *testing.T, initialState statemachine.State, rc *release.ReleaseData, configValues types.AppConfigValues) *api.API
		baseURL     string
	}{
		{
			name:        "linux install",
			installType: "linux",
			createAPI: func(t *testing.T, initialState statemachine.State, rc *release.ReleaseData, configValues types.AppConfigValues) *api.API {
				controller, err := linuxinstall.NewInstallController(
					linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(initialState))),
					linuxinstall.WithReleaseData(rc),
					linuxinstall.WithConfigValues(configValues),
				)
				require.NoError(t, err)
				// Create the API with the install controller
				return integration.NewAPIWithReleaseData(t,
					api.WithLinuxInstallController(controller),
					api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
					api.WithLogger(logger.NewDiscardLogger()),
				)
			},
			baseURL: "/linux/install",
		},
		{
			name:        "kubernetes install",
			installType: "kubernetes",
			createAPI: func(t *testing.T, initialState statemachine.State, rc *release.ReleaseData, configValues types.AppConfigValues) *api.API {
				controller, err := kubernetesinstall.NewInstallController(
					kubernetesinstall.WithStateMachine(kubernetesinstall.NewStateMachine(kubernetesinstall.WithCurrentState(initialState))),
					kubernetesinstall.WithReleaseData(rc),
					kubernetesinstall.WithConfigValues(configValues),
				)
				require.NoError(t, err)
				// Create the API with the install controller
				return integration.NewAPIWithReleaseData(t,
					api.WithKubernetesInstallController(controller),
					api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
					api.WithLogger(logger.NewDiscardLogger()),
				)
			},
			baseURL: "/kubernetes/install",
		},
	}

	for _, tt := range installTypes {
		t.Run(tt.name, func(t *testing.T) {
			testSuite := &AppInstallTestSuite{
				installType: tt.installType,
				createAPI:   tt.createAPI,
				baseURL:     tt.baseURL,
			}
			suite.Run(t, testSuite)
		})
	}
}
