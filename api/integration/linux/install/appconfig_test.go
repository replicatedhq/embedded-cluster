package install

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	apiclient "github.com/replicatedhq/embedded-cluster/api/client"
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(states.StateNew))),
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
		assert.Equal(t, "Required Item is required", apiErr.Errors[0].Message, "Error should indicate item is required")
	})

	// Test PatchLinuxAppConfigValues with invalid state transition
	t.Run("PatchLinuxAppConfigValues invalid state", func(t *testing.T) {
		// Create an install controller in a completed state
		completedInstallController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(states.StateSucceeded))),
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
	installController, err := linuxinstall.NewInstallController(
		linuxinstall.WithConfigValues(configValues),
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

	// Test GetLinuxAppConfigValues
	t.Run("GetLinuxAppConfigValues", func(t *testing.T) {
		// Get the app config values using the client
		values, err := c.GetLinuxAppConfigValues()
		require.NoError(t, err, "GetLinuxAppConfigValues should succeed")

		// Verify the app config values are returned from the store
		assert.Equal(t, configValues, values, "app config values should be returned from store")
	})

	// Test GetLinuxAppConfigValues with invalid token
	t.Run("GetLinuxAppConfigValues unauthorized", func(t *testing.T) {
		// Create client with invalid token
		invalidClient := apiclient.New(server.URL, apiclient.WithToken("INVALID_TOKEN"))

		// Get the app config values using the client
		_, err := invalidClient.GetLinuxAppConfigValues()
		require.Error(t, err, "GetLinuxAppConfigValues should fail with invalid token")

		// Check that the error is of correct type
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode, "Error should have Unauthorized status code")
	})
}
