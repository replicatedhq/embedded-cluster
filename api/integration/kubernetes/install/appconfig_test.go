package install

import (
	"context"
	_ "embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	apiclient "github.com/replicatedhq/embedded-cluster/api/client"
	kubernetesinstall "github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/install"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
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
	installController, err := kubernetesinstall.NewInstallController(
		kubernetesinstall.WithStateMachine(kubernetesinstall.NewStateMachine(kubernetesinstall.WithCurrentState(states.StateNew))),
		kubernetesinstall.WithReleaseData(&release.ReleaseData{
			AppConfig: &appConfig,
		}),
		kubernetesinstall.WithHelmClient(&helm.MockClient{}),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance := integration.NewTargetKubernetesAPIWithReleaseData(t, types.ModeInstall,
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

	// Test PatchKubernetesAppConfigValues with missing required item
	t.Run("PatchKubernetesAppConfigValues missing required", func(t *testing.T) {
		// Create config values without the required item
		configValues := types.AppConfigValues{
			"test-item": types.AppConfigValue{Value: "new-value"},
			// required-item is intentionally omitted
		}

		// Set the app config values using the client
		_, err := c.PatchKubernetesInstallAppConfigValues(context.Background(), configValues)
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
			kubernetesinstall.WithStateMachine(kubernetesinstall.NewStateMachine(kubernetesinstall.WithCurrentState(states.StateSucceeded))),
			kubernetesinstall.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfig,
			}),
			kubernetesinstall.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		// Create the API with the completed install controller
		completedAPIInstance := integration.NewTargetKubernetesAPIWithReleaseData(t, types.ModeInstall,
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
		_, err = completedClient.PatchKubernetesInstallAppConfigValues(context.Background(), configValues)
		require.Error(t, err, "PatchKubernetesAppConfigValues should fail with invalid state transition")

		// Check that the error is of correct type
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusConflict, apiErr.StatusCode, "Error should have Conflict status code")
		assert.Contains(t, apiErr.Message, "invalid transition", "Error should mention invalid transition")
	})

	// Test PatchKubernetesAppConfigValues
	t.Run("PatchKubernetesAppConfigValues", func(t *testing.T) {
		// Create config values to set
		configValues := types.AppConfigValues{
			"test-item":     types.AppConfigValue{Value: "new-value"},
			"required-item": types.AppConfigValue{Value: "required-value"},
			"file-item":     types.AppConfigValue{Value: "SGVsbG8gV29ybGQ=", Filename: "new-file.txt"},
		}

		// Set the app config values using the client
		config, err := c.PatchKubernetesInstallAppConfigValues(context.Background(), configValues)
		require.NoError(t, err, "PatchKubernetesAppConfigValues should succeed")

		// Verify the config values are returned
		assert.Equal(t, "new-value", config["test-item"].Value, "new value for test-item should be returned")
		assert.Equal(t, "required-value", config["required-item"].Value, "new value for required-item should be returned")
		assert.Equal(t, "SGVsbG8gV29ybGQ=", config["file-item"].Value, "new value for file-item should be returned")
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
	installController, err := kubernetesinstall.NewInstallController(
		kubernetesinstall.WithConfigValues(configValues),
		kubernetesinstall.WithReleaseData(&release.ReleaseData{
			AppConfig: &appConfig,
		}),
		kubernetesinstall.WithHelmClient(&helm.MockClient{}),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance := integration.NewTargetKubernetesAPIWithReleaseData(t, types.ModeInstall,
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
		values, err := c.GetKubernetesInstallAppConfigValues(context.Background())
		require.NoError(t, err, "GetKubernetesAppConfigValues should succeed")

		// Verify the app config values are returned from the store
		assert.Equal(t, configValues, values, "app config values should be returned from store")
	})

	// Test GetKubernetesAppConfigValues with invalid token
	t.Run("GetKubernetesAppConfigValues unauthorized", func(t *testing.T) {
		// Create client with invalid token
		invalidClient := apiclient.New(server.URL, apiclient.WithToken("INVALID_TOKEN"))

		// Get the app config values using the client
		_, err := invalidClient.GetKubernetesInstallAppConfigValues(context.Background())
		require.Error(t, err, "GetKubernetesAppConfigValues should fail with invalid token")

		// Check that the error is of correct type
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode, "Error should have Unauthorized status code")
	})
}
