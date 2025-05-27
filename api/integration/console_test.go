package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	"github.com/replicatedhq/embedded-cluster/api/controllers/console"
	"github.com/replicatedhq/embedded-cluster/api/controllers/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/installation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsoleListAvailableNetworkInterfaces(t *testing.T) {
	netutils := &mockNetUtils{ifaces: []string{"eth0", "eth1"}}

	// Create a console controller
	consoleController, err := console.NewConsoleController(
		console.WithNetUtils(netutils),
	)
	require.NoError(t, err)

	// Create an install controller
	installController, err := install.NewInstallController(
		install.WithInstallationManager(installation.NewInstallationManager(
			installation.WithNetUtils(netutils),
		)),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance, err := api.New(
		"password",
		api.WithInstallController(installController),
		api.WithConsoleController(consoleController),
		api.WithAuthController(&staticAuthController{"TOKEN"}),
		api.WithLogger(api.NewDiscardLogger()),
	)
	require.NoError(t, err)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Create a request to the network interfaces endpoint
	req := httptest.NewRequest(http.MethodGet, "/console/available-network-interfaces", nil)
	req.Header.Set("Authorization", "Bearer TOKEN")
	rec := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(rec, req)

	// Check the response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	// Parse the response body
	var response struct {
		NetworkInterfaces []string `json:"networkInterfaces"`
	}
	err = json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	// Verify the response contains the expected network interfaces
	assert.Equal(t, []string{"eth0", "eth1"}, response.NetworkInterfaces)
}
