package integration

import (
	_ "embed"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	apiclient "github.com/replicatedhq/embedded-cluster/api/client"
	linuxupgrade "github.com/replicatedhq/embedded-cluster/api/controllers/linux/upgrade"
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

// TestUpgradeController_PatchAppConfigValuesWithAPIClient tests the PatchUpgradeAppConfigValues endpoint using the API client
func TestUpgradeController_PatchAppConfigValuesWithAPIClient(t *testing.T) {
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

	// Create an upgrade controller with the app config and initial values
	upgradeController, err := linuxupgrade.NewUpgradeController(
		linuxupgrade.WithStateMachine(linuxupgrade.NewStateMachine(linuxupgrade.WithCurrentState(states.StateNew))),
		linuxupgrade.WithConfigValues(initialConfigValues),
		linuxupgrade.WithReleaseData(&release.ReleaseData{
			AppConfig: &appConfig,
		}),
	)
	require.NoError(t, err)

	// Create the API with the upgrade controller
	apiInstance := integration.NewAPIWithReleaseData(t,
		api.WithLinuxUpgradeController(upgradeController),
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

	// Test PatchLinuxUpgradeAppConfigValues with partial update (not all fields)
	t.Run("PatchLinuxUpgradeAppConfigValues partial update", func(t *testing.T) {
		// Create config values with only some fields updated (required field not included, should keep existing value)
		configValues := types.AppConfigValues{
			"test-item": types.AppConfigValue{Value: "partially-updated-value"},
		}

		// Set the app config values using the client
		config, err := c.PatchLinuxUpgradeAppConfigValues(configValues)
		require.NoError(t, err, "PatchLinuxUpgradeAppConfigValues should succeed with partial update")

		// Verify that updated fields are changed and unspecified required field retains its existing value
		assert.Equal(t, "partially-updated-value", config["test-item"].Value, "test-item should be updated")
		assert.Equal(t, "initial-required", config["required-item"].Value, "required-item should retain initial value when not specified in patch")
		assert.Equal(t, "aW5pdGlhbA==", config["file-item"].Value, "file-item should retain initial value when not specified in patch")
		assert.Equal(t, "file.txt", config["file-item"].Filename, "file-item filename should retain initial value when not specified in patch")
	})

	// Test PatchLinuxUpgradeAppConfigValues with clearing required item
	t.Run("PatchLinuxUpgradeAppConfigValues clear required", func(t *testing.T) {
		// Try to clear the required item by setting it to empty
		configValues := types.AppConfigValues{
			"test-item":     types.AppConfigValue{Value: "updated-value"},
			"required-item": types.AppConfigValue{Value: ""}, // explicitly clear required field
		}

		// Set the app config values using the client
		_, err := c.PatchLinuxUpgradeAppConfigValues(configValues)
		require.Error(t, err, "PatchLinuxUpgradeAppConfigValues should fail when clearing required item")

		// Check that the error is of correct type
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode, "Error should have BadRequest status code")
		assert.Len(t, apiErr.Errors, 1, "Should have one validation error")
		assert.Equal(t, "required-item", apiErr.Errors[0].Field, "Error should be for required-item field")
		assert.Equal(t, "Required Item is required", apiErr.Errors[0].Message, "Error should indicate item is required")
	})

	// Test PatchLinuxUpgradeAppConfigValues with invalid state transition
	t.Run("PatchLinuxUpgradeAppConfigValues invalid state", func(t *testing.T) {
		// Create an upgrade controller in a completed state
		completedUpgradeController, err := linuxupgrade.NewUpgradeController(
			linuxupgrade.WithStateMachine(linuxupgrade.NewStateMachine(linuxupgrade.WithCurrentState(states.StateSucceeded))),
			linuxupgrade.WithConfigValues(initialConfigValues),
			linuxupgrade.WithReleaseData(&release.ReleaseData{
				AppConfig: &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with the completed upgrade controller
		completedAPIInstance := integration.NewAPIWithReleaseData(t,
			api.WithLinuxUpgradeController(completedUpgradeController),
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
			"test-item":     types.AppConfigValue{Value: "updated-value"},
			"required-item": types.AppConfigValue{Value: "updated-required"},
		}

		// Set the app config values using the client
		_, err = completedClient.PatchLinuxUpgradeAppConfigValues(configValues)
		require.Error(t, err, "PatchLinuxUpgradeAppConfigValues should fail with invalid state transition")

		// Check that the error is of correct type
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusConflict, apiErr.StatusCode, "Error should have Conflict status code")
		assert.Contains(t, apiErr.Message, "invalid transition", "Error should mention invalid transition")
	})

	// Test PatchLinuxUpgradeAppConfigValues with valid required field values
	t.Run("PatchLinuxUpgradeAppConfigValues success", func(t *testing.T) {
		// Create config values to update from initial values (keep required field populated)
		configValues := types.AppConfigValues{
			"test-item":     types.AppConfigValue{Value: "updated-value"},
			"required-item": types.AppConfigValue{Value: "updated-required"},
			"file-item":     types.AppConfigValue{Value: "dXBkYXRlZEZpbGU=", Filename: "updated-file.txt"},
		}

		// Set the app config values using the client
		config, err := c.PatchLinuxUpgradeAppConfigValues(configValues)
		require.NoError(t, err, "PatchLinuxUpgradeAppConfigValues should succeed when required field is populated")

		// Verify the app config values are returned from the store and updated from initial values
		assert.Equal(t, "updated-value", config["test-item"].Value, "test-item should be updated from initial-value")
		assert.Equal(t, "updated-required", config["required-item"].Value, "required-item should be updated from initial-required")
		assert.Equal(t, "dXBkYXRlZEZpbGU=", config["file-item"].Value, "file-item value should be updated from initial")
		assert.Equal(t, "updated-file.txt", config["file-item"].Filename, "file-item filename should be updated")
	})
}

// TestUpgradeController_GetAppConfigValuesWithAPIClient tests the GetUpgradeAppConfigValues endpoint using the API client
func TestUpgradeController_GetAppConfigValuesWithAPIClient(t *testing.T) {
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

	// Create an upgrade controller with the existing config values
	upgradeController, err := linuxupgrade.NewUpgradeController(
		linuxupgrade.WithStateMachine(linuxupgrade.NewStateMachine(linuxupgrade.WithCurrentState(states.StateNew))),
		linuxupgrade.WithConfigValues(existingConfigValues),
		linuxupgrade.WithReleaseData(&release.ReleaseData{
			AppConfig: &appConfig,
		}),
	)
	require.NoError(t, err)

	// Create the API with the upgrade controller
	apiInstance := integration.NewAPIWithReleaseData(t,
		api.WithLinuxUpgradeController(upgradeController),
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

	// Test GetLinuxUpgradeAppConfigValues
	t.Run("GetLinuxUpgradeAppConfigValues", func(t *testing.T) {
		// Get the app config values using the client
		values, err := c.GetLinuxUpgradeAppConfigValues()
		require.NoError(t, err, "GetLinuxUpgradeAppConfigValues should succeed")

		// Verify the app config values are returned from the store (existing values, not defaults)
		assert.Equal(t, existingConfigValues, values, "app config values should be returned from store with existing values")
	})

	// Test GetLinuxUpgradeAppConfigValues with invalid token
	t.Run("GetLinuxUpgradeAppConfigValues unauthorized", func(t *testing.T) {
		// Create client with invalid token
		invalidClient := apiclient.New(server.URL, apiclient.WithToken("INVALID_TOKEN"))

		// Get the app config values using the client
		_, err := invalidClient.GetLinuxUpgradeAppConfigValues()
		require.Error(t, err, "GetLinuxUpgradeAppConfigValues should fail with invalid token")

		// Check that the error is of correct type
		apiErr, ok := err.(*types.APIError)
		require.True(t, ok, "Error should be of type *types.APIError")
		assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode, "Error should have Unauthorized status code")
	})
}
