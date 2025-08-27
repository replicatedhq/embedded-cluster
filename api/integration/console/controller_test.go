package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	"github.com/replicatedhq/embedded-cluster/api/controllers/console"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsoleListAvailableNetworkInterfaces(t *testing.T) {
	netUtils := &utils.MockNetUtils{}
	netUtils.On("ListValidNetworkInterfaces").Return([]string{"eth0", "eth1"}, nil).Once()

	// Create a console controller
	consoleController, err := console.NewConsoleController(
		console.WithNetUtils(netUtils),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance := integration.NewTargetLinuxAPIWithReleaseData(t,
		api.WithConsoleController(consoleController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)

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

func TestConsoleListAvailableNetworkInterfacesUnauthorized(t *testing.T) {
	netUtils := &utils.MockNetUtils{}
	netUtils.On("ListValidNetworkInterfaces").Return([]string{"eth0", "eth1"}, nil).Once()

	// Create a console controller
	consoleController, err := console.NewConsoleController(
		console.WithNetUtils(netUtils),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance := integration.NewTargetLinuxAPIWithReleaseData(t,
		api.WithConsoleController(consoleController),
		api.WithAuthController(auth.NewStaticAuthController("VALID_TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Create a request with an invalid token
	req := httptest.NewRequest(http.MethodGet, "/console/available-network-interfaces", nil)
	req.Header.Set("Authorization", "Bearer INVALID_TOKEN")
	rec := httptest.NewRecorder()

	// Serve the request
	router.ServeHTTP(rec, req)

	// Check the response is unauthorized
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	// Check that the API response is of type APIError
	var apiErr *types.APIError
	err = json.NewDecoder(rec.Body).Decode(&apiErr)
	require.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
}

func TestConsoleListAvailableNetworkInterfacesError(t *testing.T) {
	// Create a mock that returns an error
	netUtils := &utils.MockNetUtils{}
	netUtils.On("ListValidNetworkInterfaces").Return(nil, assert.AnError).Once()

	// Create a console controller
	consoleController, err := console.NewConsoleController(
		console.WithNetUtils(netUtils),
	)
	require.NoError(t, err)

	// Create the API with the install controller
	apiInstance := integration.NewTargetLinuxAPIWithReleaseData(t,
		api.WithConsoleController(consoleController),
		api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
		api.WithLogger(logger.NewDiscardLogger()),
	)

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
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	// Check that the API response is of type APIError
	var apiErr *types.APIError
	err = json.NewDecoder(rec.Body).Decode(&apiErr)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
}
