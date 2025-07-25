package integration

//
// import (
// 	"bytes"
// 	"encoding/json"
// 	"net/http"
// 	"net/http/httptest"
// 	"testing"
//
// 	"github.com/gorilla/mux"
// 	"github.com/replicatedhq/embedded-cluster/api"
// 	"github.com/replicatedhq/embedded-cluster/api/integration"
// 	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
// 	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
// 	"github.com/replicatedhq/embedded-cluster/api/types"
// 	"github.com/replicatedhq/embedded-cluster/pkg/release"
// 	"github.com/replicatedhq/kotskinds/multitype"
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// 	"github.com/stretchr/testify/suite"
// )
//
// type AppInstallTestSuite struct {
// 	suite.Suite
// 	installType string
// 	apiInstance *api.API
// 	router      *mux.Router
// 	baseURL     string
// }
//
// func (suite *AppInstallTestSuite) SetupTest() {
// 	suite.router = mux.NewRouter()
// 	suite.apiInstance.RegisterRoutes(suite.router)
// }
//
// func (suite *AppInstallTestSuite) TestGetAppConfig() {
// 	req := httptest.NewRequest(http.MethodGet, suite.baseURL+"/app/config", nil)
// 	req.Header.Set("Authorization", "Bearer TOKEN")
// 	rec := httptest.NewRecorder()
//
// 	suite.router.ServeHTTP(rec, req)
//
// 	suite.Equal(http.StatusOK, rec.Code)
// 	// ... shared test assertions
// }
//
// func (s *AppInstallTestSuite) TestPatchAppConfigValues() {
// 	// Create an app config
// 	appConfig := kotsv1beta1.Config{
// 		Spec: kotsv1beta1.ConfigSpec{
// 			Groups: []kotsv1beta1.ConfigGroup{
// 				{
// 					Name:  "test-group",
// 					Title: "Test Group",
// 					Items: []kotsv1beta1.ConfigItem{
// 						{
// 							Name:    "test-item",
// 							Type:    "text",
// 							Title:   "Test Item",
// 							Default: multitype.FromString("default"),
// 							Value:   multitype.FromString("value"),
// 						},
// 						{
// 							Name:    "another-item",
// 							Type:    "text",
// 							Title:   "Another Item",
// 							Default: multitype.FromString("default2"),
// 							Value:   multitype.FromString("value2"),
// 						},
// 						{
// 							Name:     "required-item",
// 							Type:     "text",
// 							Title:    "Required Item",
// 							Required: true,
// 						},
// 						{
// 							Name:     "required-password",
// 							Type:     "password",
// 							Title:    "Required Password",
// 							Required: true,
// 						},
// 						{
// 							Name:     "file-item",
// 							Type:     "file",
// 							Title:    "File Item",
// 							Filename: "file.txt",
// 							Default:  multitype.FromString("SGVsbG8="),
// 							Value:    multitype.FromString("QQ=="),
// 						},
// 					},
// 				},
// 			},
// 		},
// 	}
//
// 	// Test successful set and get
// 	s.T().Run("Success", func(t *testing.T) {
// 		// Create an install controller with the app config
// 		installController, err := linuxinstall.NewInstallController(
// 			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(linuxinstall.StateNew))),
// 			linuxinstall.WithReleaseData(&release.ReleaseData{
// 				AppConfig: &appConfig,
// 			}),
// 		)
// 		require.NoError(t, err)
//
// 		// Create the API with the install controller
// 		apiInstance := integration.NewAPIWithReleaseData(t,
// 			api.WithLinuxInstallController(installController),
// 			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
// 			api.WithLogger(logger.NewDiscardLogger()),
// 		)
//
// 		// Create a router and register the API routes
// 		router := mux.NewRouter()
// 		apiInstance.RegisterRoutes(router)
//
// 		// Create a request to patch config values
// 		patchRequest := types.PatchAppConfigValuesRequest{
// 			Values: types.AppConfigValues{
// 				"test-item":         types.AppConfigValue{Value: "new-value"},
// 				"required-item":     types.AppConfigValue{Value: "required-value"},
// 				"required-password": types.AppConfigValue{Value: "required-password"},
// 				"file-item":         types.AppConfigValue{Value: "SGVsbG8gV29ybGQ=", Filename: "new-file.txt"},
// 			},
// 		}
//
// 		reqBodyBytes, err := json.Marshal(patchRequest)
// 		require.NoError(t, err)
//
// 		// Create a request to patch config values
// 		req := httptest.NewRequest(http.MethodPatch, "/linux/install/app/config/values", bytes.NewReader(reqBodyBytes))
// 		req.Header.Set("Content-Type", "application/json")
// 		req.Header.Set("Authorization", "Bearer "+"TOKEN")
// 		rec := httptest.NewRecorder()
//
// 		// Serve the request
// 		router.ServeHTTP(rec, req)
//
// 		// Check the response
// 		assert.Equal(t, http.StatusOK, rec.Code)
//
// 		// Parse the response body
// 		var response types.AppConfigValuesResponse
// 		err = json.NewDecoder(rec.Body).Decode(&response)
// 		require.NoError(t, err)
// 		require.NotNil(t, response.Values, "response values should not be nil")
//
// 		// Verify the app config values are returned from the store
// 		assert.Equal(t, "new-value", response.Values["test-item"].Value, "test-item should be updated")
// 		assert.Equal(t, "required-value", response.Values["required-item"].Value, "required-item should be updated")
// 		assert.Equal(t, "SGVsbG8gV29ybGQ=", response.Values["file-item"].Value, "file-item value should be updated")
// 		assert.Equal(t, "new-file.txt", response.Values["file-item"].Filename, "file-item value should contain a filename")
// 	})
//
// 	// Test authorization
// 	t.Run("Authorization error", func(t *testing.T) {
// 		// Create an install controller with the app config
// 		installController, err := linuxinstall.NewInstallController(
// 			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(linuxinstall.StateNew))),
// 			linuxinstall.WithReleaseData(&release.ReleaseData{
// 				AppConfig: &appConfig,
// 			}),
// 		)
// 		require.NoError(t, err)
//
// 		// Create the API with the install controller
// 		apiInstance := integration.NewAPIWithReleaseData(t,
// 			api.WithLinuxInstallController(installController),
// 			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
// 			api.WithLogger(logger.NewDiscardLogger()),
// 		)
//
// 		// Create a router and register the API routes
// 		router := mux.NewRouter()
// 		apiInstance.RegisterRoutes(router)
//
// 		// Create a request to patch config values
// 		patchRequest := types.PatchAppConfigValuesRequest{
// 			Values: types.AppConfigValues{
// 				"test-item":     types.AppConfigValue{Value: "new-value"},
// 				"required-item": types.AppConfigValue{Value: "required-value"},
// 			},
// 		}
//
// 		reqBodyBytes, err := json.Marshal(patchRequest)
// 		require.NoError(t, err)
//
// 		// Create a request with invalid token
// 		req := httptest.NewRequest(http.MethodPatch, "/linux/install/app/config/values", bytes.NewReader(reqBodyBytes))
// 		req.Header.Set("Content-Type", "application/json")
// 		req.Header.Set("Authorization", "Bearer "+"NOT_A_TOKEN")
// 		rec := httptest.NewRecorder()
//
// 		// Serve the request
// 		router.ServeHTTP(rec, req)
//
// 		// Check the response
// 		assert.Equal(t, http.StatusUnauthorized, rec.Code)
//
// 		// Parse the response body
// 		var apiError types.APIError
// 		err = json.NewDecoder(rec.Body).Decode(&apiError)
// 		require.NoError(t, err)
// 		assert.Equal(t, http.StatusUnauthorized, apiError.StatusCode)
// 	})
//
// 	// Test invalid state transition
// 	t.Run("Invalid state transition", func(t *testing.T) {
// 		// Create an install controller with the app config
// 		installController, err := linuxinstall.NewInstallController(
// 			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(linuxinstall.StateSucceeded))),
// 			linuxinstall.WithReleaseData(&release.ReleaseData{
// 				AppConfig: &appConfig,
// 			}),
// 		)
// 		require.NoError(t, err)
//
// 		// Create the API with the install controller
// 		apiInstance := integration.NewAPIWithReleaseData(t,
// 			api.WithLinuxInstallController(installController),
// 			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
// 			api.WithLogger(logger.NewDiscardLogger()),
// 		)
//
// 		// Create a router and register the API routes
// 		router := mux.NewRouter()
// 		apiInstance.RegisterRoutes(router)
//
// 		// Create a request to patch config values
// 		patchRequest := types.PatchAppConfigValuesRequest{
// 			Values: types.AppConfigValues{
// 				"test-item":     types.AppConfigValue{Value: "new-value"},
// 				"required-item": types.AppConfigValue{Value: "required-value"},
// 			},
// 		}
//
// 		reqBodyBytes, err := json.Marshal(patchRequest)
// 		require.NoError(t, err)
//
// 		// Create a request
// 		req := httptest.NewRequest(http.MethodPatch, "/linux/install/app/config/values", bytes.NewReader(reqBodyBytes))
// 		req.Header.Set("Content-Type", "application/json")
// 		req.Header.Set("Authorization", "Bearer "+"TOKEN")
// 		rec := httptest.NewRecorder()
//
// 		// Serve the request
// 		router.ServeHTTP(rec, req)
//
// 		// Check the response
// 		assert.Equal(t, http.StatusConflict, rec.Code)
//
// 		// Parse the response body
// 		var apiError types.APIError
// 		err = json.NewDecoder(rec.Body).Decode(&apiError)
// 		require.NoError(t, err)
// 		assert.Equal(t, http.StatusConflict, apiError.StatusCode)
// 		assert.Contains(t, apiError.Message, "invalid transition")
// 	})
//
// 	// Test missing required item
// 	t.Run("Missing required item", func(t *testing.T) {
// 		// Create an install controller with the app config
// 		installController, err := linuxinstall.NewInstallController(
// 			linuxinstall.WithReleaseData(&release.ReleaseData{
// 				AppConfig: &appConfig,
// 			}),
// 		)
// 		require.NoError(t, err)
//
// 		// Create the API with the install controller
// 		apiInstance := integration.NewAPIWithReleaseData(t,
// 			api.WithLinuxInstallController(installController),
// 			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
// 			api.WithLogger(logger.NewDiscardLogger()),
// 		)
// 		require.NoError(t, err)
//
// 		// Create a router and register the API routes
// 		router := mux.NewRouter()
// 		apiInstance.RegisterRoutes(router)
//
// 		// Create a request to patch config values without the required item
// 		setRequest := types.PatchAppConfigValuesRequest{
// 			Values: types.AppConfigValues{
// 				"test-item": types.AppConfigValue{Value: "new-value"},
// 				// required-item is intentionally omitted
// 			},
// 		}
//
// 		reqBodyBytes, err := json.Marshal(setRequest)
// 		require.NoError(t, err)
//
// 		// Create a request
// 		req := httptest.NewRequest(http.MethodPatch, "/linux/install/app/config/values", bytes.NewReader(reqBodyBytes))
// 		req.Header.Set("Content-Type", "application/json")
// 		req.Header.Set("Authorization", "Bearer "+"TOKEN")
// 		rec := httptest.NewRecorder()
//
// 		// Serve the request
// 		router.ServeHTTP(rec, req)
//
// 		// Check the response
// 		assert.Equal(t, http.StatusBadRequest, rec.Code)
//
// 		// Parse the response body
// 		var apiError types.APIError
// 		err = json.NewDecoder(rec.Body).Decode(&apiError)
// 		require.NoError(t, err)
// 		assert.Equal(t, http.StatusBadRequest, apiError.StatusCode)
// 		assert.Len(t, apiError.Errors, 2)
// 		assert.Equal(t, apiError.Errors[0].Field, "required-item")
// 		assert.Equal(t, apiError.Errors[0].Message, "item is required")
// 		assert.Equal(t, apiError.Errors[1].Field, "required-password")
// 		assert.Equal(t, apiError.Errors[1].Message, "item is required")
// 	})
// }
//
// func (suite *AppInstallTestSuite) TestGetAppConfigValues() {
// 	// Another test method that automatically runs for both install types
// }
//
// // Runner function that executes the suite for both install types
// func TestAppInstallSuite(t *testing.T) {
// 	installTypes := []struct {
// 		name        string
// 		installType string
// 		createAPI   func() *api.API
// 		baseURL     string
// 	}{
// 		{
// 			name:        "linux install",
// 			installType: "linux",
// 			createAPI: func() *api.API {
// 				controller, _ := linuxinstall.NewController( /* ... */ )
// 				return integration.NewAPIWithReleaseData(t,
// 					api.WithLinuxInstallController(controller),
// 					api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
// 				)
// 			},
// 			baseURL: "/linux/install",
// 		},
// 		{
// 			name:        "kubernetes install",
// 			installType: "kubernetes",
// 			createAPI: func() *api.API {
// 				controller, _ := kubernetesinstall.NewController( /* ... */ )
// 				return integration.NewAPIWithReleaseData(t,
// 					api.WithKubernetesInstallController(controller),
// 					api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
// 				)
// 			},
// 			baseURL: "/kubernetes/install",
// 		},
// 	}
//
// 	for _, tt := range installTypes {
// 		t.Run(tt.name, func(t *testing.T) {
// 			testSuite := &AppInstallTestSuite{
// 				installType: tt.installType,
// 				apiInstance: tt.createAPI(),
// 				baseURL:     tt.baseURL,
// 			}
// 			suite.Run(t, testSuite)
// 		})
// 	}
// }
