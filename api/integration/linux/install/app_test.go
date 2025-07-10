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
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	appconfigstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/config"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinuxGetAppConfig(t *testing.T) {
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
	installController, err := linuxinstall.NewInstallController(
		linuxinstall.WithStore(
			store.NewMemoryStore(store.WithAppConfigStore(appconfigstore.NewMemoryStore(appconfigstore.WithConfigValues(configValues)))),
		),
		linuxinstall.WithReleaseData(&release.ReleaseData{
			AppConfig: &appConfig,
		}),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance, err := api.New(
		types.APIConfig{
			Password: "password",
		},
		api.WithLinuxInstallController(installController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)
	require.NoError(t, err)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test successful get
	t.Run("Success", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/linux/install/app/config", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/linux/install/app/config", nil)
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

func TestLinuxSetAppConfigValues(t *testing.T) {
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
		apiInstance, err := api.New(
			types.APIConfig{
				Password: "password",
			},
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request to set config values
		setRequest := types.SetAppConfigValuesRequest{
			Values: map[string]string{
				"test-item": "new-value",
			},
		}

		reqBodyBytes, err := json.Marshal(setRequest)
		require.NoError(t, err)

		// Create a request to set config values
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/config/values", bytes.NewReader(reqBodyBytes))
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
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(linuxinstall.StateNew))),
			linuxinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance, err := api.New(
			types.APIConfig{
				Password: "password",
			},
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request to set config values
		setRequest := types.SetAppConfigValuesRequest{
			Values: map[string]string{
				"test-item": "new-value",
			},
		}

		reqBodyBytes, err := json.Marshal(setRequest)
		require.NoError(t, err)

		// Create a request with invalid token
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/config/values", bytes.NewReader(reqBodyBytes))
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
		apiInstance, err := api.New(
			types.APIConfig{
				Password: "password",
			},
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request to set config values
		setRequest := types.SetAppConfigValuesRequest{
			Values: map[string]string{
				"test-item": "new-value",
			},
		}

		reqBodyBytes, err := json.Marshal(setRequest)
		require.NoError(t, err)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/linux/install/app/config/values", bytes.NewReader(reqBodyBytes))
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
}

func TestLinuxGetAppConfigValues(t *testing.T) {
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
	installController, err := linuxinstall.NewInstallController(
		linuxinstall.WithStore(
			store.NewMemoryStore(store.WithAppConfigStore(appconfigstore.NewMemoryStore(appconfigstore.WithConfigValues(configValues)))),
		),
		linuxinstall.WithReleaseData(&release.ReleaseData{
			AppConfig: &appConfig,
		}),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance, err := api.New(
		types.APIConfig{
			Password: "password",
		},
		api.WithLinuxInstallController(installController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)
	require.NoError(t, err)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test successful get
	t.Run("Success", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/linux/install/app/config/values", nil)
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
		req := httptest.NewRequest(http.MethodGet, "/linux/install/app/config/values", nil)
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
