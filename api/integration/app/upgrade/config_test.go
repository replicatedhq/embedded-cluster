package integration

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	kubernetesupgrade "github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/upgrade"
	linuxupgrade "github.com/replicatedhq/embedded-cluster/api/controllers/linux/upgrade"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type AppConfigTestSuite struct {
	suite.Suite
	installType string
	createAPI   func(t *testing.T, configValues types.AppConfigValues, state statemachine.State, appConfig *kotsv1beta1.Config) *api.API
	baseURL     string
}

func (s *AppConfigTestSuite) TestPatchAppConfigValues() {
	t := s.T()

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
							Value:   multitype.FromString("initial-value"),
						},
						{
							Name:     "required-item",
							Type:     "text",
							Title:    "Required Item",
							Required: true,
							Value:    multitype.FromString("initial-required"),
						},
						{
							Name:     "file-item",
							Type:     "file",
							Title:    "File Item",
							Filename: "file.txt",
							Default:  multitype.FromString("SGVsbG8="),
							Value:    multitype.FromString("aW5pdGlhbA=="),
						},
					},
				},
			},
		},
	}

	// Create initial config values that simulate an existing configuration
	initialConfigValues := types.AppConfigValues{
		"test-item":     types.AppConfigValue{Value: "initial-value"},
		"required-item": types.AppConfigValue{Value: "initial-required"},
		"file-item":     types.AppConfigValue{Value: "aW5pdGlhbA==", Filename: "file.txt"},
	}

	// Create the API with the app config and initial values
	apiInstance := s.createAPI(t, initialConfigValues, states.StateNew, &appConfig)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test PatchAppConfigValues with partial update (not all fields)
	t.Run("PatchAppConfigValues partial update", func(t *testing.T) {
		// Create config values with only some fields updated (required field not included, should keep existing value)
		configValues := types.AppConfigValues{
			"test-item": types.AppConfigValue{Value: "partially-updated-value"},
		}

		request := types.PatchAppConfigValuesRequest{
			Values: configValues,
		}

		reqBodyBytes, err := json.Marshal(request)
		require.NoError(t, err)
		t.Logf("Request body: %s", string(reqBodyBytes))

		// Create request
		req := httptest.NewRequest(http.MethodPatch, s.baseURL+"/app/config/values", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		t.Logf("Response status: %d, body: %s", rec.Code, rec.Body.String())
		require.Equal(t, http.StatusOK, rec.Code, "expected status ok, got %d with body %s", rec.Code, rec.Body.String())

		// Parse the response body
		var response types.AppConfigValuesResponse
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify that updated fields are changed and unspecified required field retains its existing value
		assert.Equal(t, "partially-updated-value", response.Values["test-item"].Value, "test-item should be updated")
		assert.Equal(t, "initial-required", response.Values["required-item"].Value, "required-item should retain initial value when not specified in patch")
		assert.Equal(t, "aW5pdGlhbA==", response.Values["file-item"].Value, "file-item should retain initial value when not specified in patch")
		assert.Equal(t, "file.txt", response.Values["file-item"].Filename, "file-item filename should retain initial value when not specified in patch")
	})

	// Test PatchAppConfigValues with clearing required item
	t.Run("PatchAppConfigValues clear required", func(t *testing.T) {
		// Try to clear the required item by setting it to empty
		configValues := types.AppConfigValues{
			"test-item":     types.AppConfigValue{Value: "updated-value"},
			"required-item": types.AppConfigValue{Value: ""}, // explicitly clear required field
		}

		request := types.PatchAppConfigValuesRequest{
			Values: configValues,
		}

		reqBodyBytes, err := json.Marshal(request)
		require.NoError(t, err)

		// Create request
		req := httptest.NewRequest(http.MethodPatch, s.baseURL+"/app/config/values", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusBadRequest, rec.Code, "expected status BadRequest, got %d", rec.Code)

		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, apiError.StatusCode, "Error should have BadRequest status code")
		assert.Len(t, apiError.Errors, 1, "Should have one validation error")
		assert.Equal(t, "required-item", apiError.Errors[0].Field, "Error should be for required-item field")
		assert.Equal(t, "Required Item is required", apiError.Errors[0].Message, "Error should indicate item is required")
	})

	// Test PatchAppConfigValues with invalid state transition
	t.Run("PatchAppConfigValues invalid state", func(t *testing.T) {
		// Create the API with the completed upgrade controller
		completedAPIInstance := s.createAPI(t, initialConfigValues, states.StateSucceeded, &appConfig)

		// Create a router and register the API routes
		completedRouter := mux.NewRouter()
		completedAPIInstance.RegisterRoutes(completedRouter)

		// Create config values to set
		configValues := types.AppConfigValues{
			"test-item":     types.AppConfigValue{Value: "updated-value"},
			"required-item": types.AppConfigValue{Value: "updated-required"},
		}

		request := types.PatchAppConfigValuesRequest{
			Values: configValues,
		}

		reqBodyBytes, err := json.Marshal(request)
		require.NoError(t, err)

		// Create request
		req := httptest.NewRequest(http.MethodPatch, s.baseURL+"/app/config/values", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		completedRouter.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusConflict, rec.Code, "expected status Conflict, got %d", rec.Code)

		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusConflict, apiError.StatusCode, "Error should have Conflict status code")
		assert.Contains(t, apiError.Message, "invalid transition", "Error should mention invalid transition")
	})

	// Test PatchAppConfigValues with valid required field values
	t.Run("PatchAppConfigValues success", func(t *testing.T) {
		// Create config values to update from initial values (keep required field populated)
		configValues := types.AppConfigValues{
			"test-item":     types.AppConfigValue{Value: "updated-value"},
			"required-item": types.AppConfigValue{Value: "updated-required"},
			"file-item":     types.AppConfigValue{Value: "dXBkYXRlZEZpbGU=", Filename: "updated-file.txt"},
		}

		request := types.PatchAppConfigValuesRequest{
			Values: configValues,
		}

		reqBodyBytes, err := json.Marshal(request)
		require.NoError(t, err)

		// Create request
		req := httptest.NewRequest(http.MethodPatch, s.baseURL+"/app/config/values", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusOK, rec.Code, "expected status ok, got %d with body %s", rec.Code, rec.Body.String())

		// Parse the response body
		var response types.AppConfigValuesResponse
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify the app config values are returned from the store and updated from initial values
		assert.Equal(t, "updated-value", response.Values["test-item"].Value, "test-item should be updated from initial-value")
		assert.Equal(t, "updated-required", response.Values["required-item"].Value, "required-item should be updated from initial-required")
		assert.Equal(t, "dXBkYXRlZEZpbGU=", response.Values["file-item"].Value, "file-item value should be updated from initial")
		assert.Equal(t, "updated-file.txt", response.Values["file-item"].Filename, "file-item filename should be updated")
	})
}

func (s *AppConfigTestSuite) TestGetAppConfigValues() {
	t := s.T()

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
							Value:   multitype.FromString("initial-value"),
						},
						{
							Name:     "file-item",
							Type:     "file",
							Title:    "File Item",
							Filename: "file.txt",
							Default:  multitype.FromString("SGVsbG8="),
							Value:    multitype.FromString("aW5pdGlhbA=="),
						},
					},
				},
			},
		},
	}

	// Create existing config values that should be returned (simulating previous configuration)
	existingConfigValues := types.AppConfigValues{
		"test-item": types.AppConfigValue{Value: "existing-value"},
		"file-item": types.AppConfigValue{Value: "ZXhpc3RpbmdGaWxl", Filename: "existing-file.txt"},
	}

	// Create the API with the existing config values
	apiInstance := s.createAPI(t, existingConfigValues, states.StateNew, &appConfig)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test GetAppConfigValues
	t.Run("GetAppConfigValues", func(t *testing.T) {
		// Create request
		req := httptest.NewRequest(http.MethodGet, s.baseURL+"/app/config/values", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusOK, rec.Code, "expected status ok, got %d with body %s", rec.Code, rec.Body.String())

		// Parse the response body
		var response types.AppConfigValuesResponse
		err := json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify the app config values are returned from the store (existing values, not defaults)
		assert.Equal(t, existingConfigValues, response.Values, "app config values should be returned from store with existing values")
	})

	// Test GetAppConfigValues with invalid token
	t.Run("GetAppConfigValues unauthorized", func(t *testing.T) {
		// Create request with invalid token
		req := httptest.NewRequest(http.MethodGet, s.baseURL+"/app/config/values", nil)
		req.Header.Set("Authorization", "Bearer INVALID_TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusUnauthorized, rec.Code, "expected status Unauthorized, got %d", rec.Code)

		// Parse the response body
		var apiError types.APIError
		err := json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, apiError.StatusCode, "Error should have Unauthorized status code")
	})
}

func TestAppConfigSuite(t *testing.T) {
	installTypes := []struct {
		name        string
		installType string
		createAPI   func(t *testing.T, configValues types.AppConfigValues, state statemachine.State, appConfig *kotsv1beta1.Config) *api.API
		baseURL     string
	}{
		{
			name:        "linux upgrade config",
			installType: "linux",
			createAPI: func(t *testing.T, configValues types.AppConfigValues, state statemachine.State, appConfig *kotsv1beta1.Config) *api.API {
				controller, err := linuxupgrade.NewUpgradeController(
					linuxupgrade.WithStateMachine(linuxupgrade.NewStateMachine(linuxupgrade.WithCurrentState(state))),
					linuxupgrade.WithConfigValues(configValues),
					linuxupgrade.WithReleaseData(&release.ReleaseData{
						AppConfig: appConfig,
					}),
				)
				require.NoError(t, err)
				return integration.NewAPIWithReleaseData(t,
					api.WithLinuxUpgradeController(controller),
					api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
					api.WithLogger(logger.NewDiscardLogger()),
				)
			},
			baseURL: "/linux/upgrade",
		},
		{
			name:        "kubernetes upgrade config",
			installType: "kubernetes",
			createAPI: func(t *testing.T, configValues types.AppConfigValues, state statemachine.State, appConfig *kotsv1beta1.Config) *api.API {
				controller, err := kubernetesupgrade.NewUpgradeController(
					kubernetesupgrade.WithStateMachine(kubernetesupgrade.NewStateMachine(kubernetesupgrade.WithCurrentState(state))),
					kubernetesupgrade.WithConfigValues(configValues),
					kubernetesupgrade.WithReleaseData(&release.ReleaseData{
						AppConfig: appConfig,
					}),
				)
				require.NoError(t, err)
				return integration.NewAPIWithReleaseData(t,
					api.WithKubernetesUpgradeController(controller),
					api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
					api.WithLogger(logger.NewDiscardLogger()),
				)
			},
			baseURL: "/kubernetes/upgrade",
		},
	}

	for _, tt := range installTypes {
		t.Run(tt.name, func(t *testing.T) {
			testSuite := &AppConfigTestSuite{
				installType: tt.installType,
				createAPI:   tt.createAPI,
				baseURL:     tt.baseURL,
			}
			suite.Run(t, testSuite)
		})
	}
}
