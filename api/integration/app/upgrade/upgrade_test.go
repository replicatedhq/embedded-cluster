package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	appcontroller "github.com/replicatedhq/embedded-cluster/api/controllers/app"
	kubernetesupgrade "github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/upgrade"
	linuxupgrade "github.com/replicatedhq/embedded-cluster/api/controllers/linux/upgrade"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	appconfig "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/config"
	appupgrademanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/upgrade"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	appupgradestore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/upgrade"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	helmcli "helm.sh/helm/v3/pkg/cli"
)

type AppUpgradeTestSuite struct {
	suite.Suite
	installType        string
	createStateMachine func(initialState statemachine.State) statemachine.Interface
	createAPI          func(t *testing.T, stateMachine statemachine.Interface, rc *release.ReleaseData, appController *appcontroller.AppController) *api.API
	router             *mux.Router
	baseURL            string
}

func (s *AppUpgradeTestSuite) TestPostUpgradeApp() {
	s.T().Run("Success", func(t *testing.T) {
		// Create mock app config manager
		mockAppConfigManager := &appconfig.MockAppConfigManager{}
		mockAppConfigManager.On("GetKotsadmConfigValues").Return(kotsv1beta1.ConfigValues{}, nil)

		// Create mock app upgrade manager with debug info
		mockAppUpgradeManager := &appupgrademanager.MockAppUpgradeManager{}
		mockAppUpgradeManager.On("Upgrade", mock.Anything, mock.Anything).Return(nil)
		mockAppUpgradeManager.On("GetStatus").Return(types.AppUpgrade{
			Status: types.Status{
				State:       types.StateRunning,
				Description: "Upgrading application",
			},
		}, nil)

		// Create mock store that will be shared between AppController and UpgradeController
		mockStore := &store.MockStore{}
		mockAppConfigManager.On("TemplateConfig", types.AppConfigValues{}, false, false).Return(types.AppConfig{}, nil)

		// Create state machine that will be shared between AppController and UpgradeController
		stateMachine := s.createStateMachine(states.StateAppPreflightsSucceeded)

		// Create app controller with mocked managers
		appController, err := appcontroller.NewAppController(
			appcontroller.WithAppConfigManager(mockAppConfigManager),
			appcontroller.WithAppUpgradeManager(mockAppUpgradeManager),
			appcontroller.WithStateMachine(stateMachine),
			appcontroller.WithStore(mockStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
			appcontroller.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		apiInstance := s.createAPI(t, stateMachine, integration.DefaultReleaseData(), appController)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create upgrade request
		upgradeRequest := types.UpgradeAppRequest{
			IgnoreAppPreflights: true,
		}

		reqBodyBytes, err := json.Marshal(upgradeRequest)
		require.NoError(t, err)

		// Create request
		req := httptest.NewRequest(http.MethodPost, s.baseURL+"/app/upgrade", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusOK, rec.Code, "expected status ok, got %d with body %s", rec.Code, rec.Body.String())

		// Parse the response body
		var response types.AppUpgrade
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify the response
		assert.Equal(t, types.StateRunning, response.Status.State)

		// Wait for the state machine to transition to Succeeded
		assert.Eventually(t, func() bool {
			return stateMachine.CurrentState() == states.StateSucceeded
		}, 10*time.Second, 100*time.Millisecond, "state should transition to Succeeded")

		// Verify mocks were called
		mockAppConfigManager.AssertExpectations(t)
		mockAppUpgradeManager.AssertExpectations(t)
	})

	s.T().Run("Authorization error", func(t *testing.T) {
		// Create state machine
		stateMachine := s.createStateMachine(states.StateAppPreflightsSucceeded)

		// Create mock store
		mockStore := &store.MockStore{}
		mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

		// Create simple app controller for auth test
		appController, err := appcontroller.NewAppController(
			appcontroller.WithStateMachine(stateMachine),
			appcontroller.WithStore(mockStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
			appcontroller.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		apiInstance := s.createAPI(t, stateMachine, integration.DefaultReleaseData(), appController)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create upgrade request
		upgradeRequest := types.UpgradeAppRequest{
			IgnoreAppPreflights: true,
		}

		reqBodyBytes, err := json.Marshal(upgradeRequest)
		require.NoError(t, err)

		// Create request with invalid token
		req := httptest.NewRequest(http.MethodPost, s.baseURL+"/app/upgrade", bytes.NewReader(reqBodyBytes))
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

	s.T().Run("Upgrade failure", func(t *testing.T) {
		// Create mock app config manager
		mockAppConfigManager := &appconfig.MockAppConfigManager{}
		mockAppConfigManager.On("GetKotsadmConfigValues").Return(kotsv1beta1.ConfigValues{}, nil)
		mockAppConfigManager.On("TemplateConfig", types.AppConfigValues{}, false, false).Return(types.AppConfig{}, nil)

		// Create mock app upgrade manager that fails
		mockAppUpgradeManager := &appupgrademanager.MockAppUpgradeManager{}
		mockAppUpgradeManager.On("Upgrade", mock.Anything, mock.Anything).Return(assert.AnError)
		mockAppUpgradeManager.On("GetStatus").Return(types.AppUpgrade{
			Status: types.Status{
				State:       types.StateRunning,
				Description: "Upgrading application",
			},
		}, nil)

		// Create mock store
		mockStore := &store.MockStore{}

		// Create state machine
		stateMachine := s.createStateMachine(states.StateAppPreflightsSucceeded)

		// Create app controller with mocked managers
		appController, err := appcontroller.NewAppController(
			appcontroller.WithAppConfigManager(mockAppConfigManager),
			appcontroller.WithAppUpgradeManager(mockAppUpgradeManager),
			appcontroller.WithStateMachine(stateMachine),
			appcontroller.WithStore(mockStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
			appcontroller.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		apiInstance := s.createAPI(t, stateMachine, integration.DefaultReleaseData(), appController)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create upgrade request
		upgradeRequest := types.UpgradeAppRequest{
			IgnoreAppPreflights: true,
		}

		reqBodyBytes, err := json.Marshal(upgradeRequest)
		require.NoError(t, err)

		// Create request
		req := httptest.NewRequest(http.MethodPost, s.baseURL+"/app/upgrade", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusOK, rec.Code, "expected status ok, got %d with body %s", rec.Code, rec.Body.String())

		// Parse the response body
		var response types.AppUpgrade
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify the response
		assert.Equal(t, types.StateRunning, response.Status.State)

		// Wait for the state machine to transition to AppUpgradeFailed
		assert.Eventually(t, func() bool {
			return stateMachine.CurrentState() == states.StateAppUpgradeFailed
		}, 10*time.Second, 100*time.Millisecond, "state should transition to AppUpgradeFailed")

		// Verify mock was called
		mockAppConfigManager.AssertExpectations(t)
		mockAppUpgradeManager.AssertExpectations(t)
	})
}

func (s *AppUpgradeTestSuite) TestGetAppUpgradeStatus() {
	s.T().Run("Success", func(t *testing.T) {
		// Create app upgrade status
		appUpgradeStatus := types.AppUpgrade{
			Status: types.Status{
				State:       types.StateRunning,
				Description: "Upgrading application",
			},
			Logs: "Upgrade in progress...",
		}

		// Create real app upgrade manager with in-memory store
		appUpgradeManager, err := appupgrademanager.NewAppUpgradeManager(
			appupgrademanager.WithAppUpgradeStore(
				appupgradestore.NewMemoryStore(appupgradestore.WithAppUpgrade(appUpgradeStatus)),
			),
		)
		require.NoError(t, err)

		stateMachine := s.createStateMachine(states.StateAppPreflightsSucceeded)

		// Create mock store
		mockStore := &store.MockStore{}
		mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

		// Create app controller with real upgrade manager
		appController, err := appcontroller.NewAppController(
			appcontroller.WithAppUpgradeManager(appUpgradeManager),
			appcontroller.WithStateMachine(stateMachine),
			appcontroller.WithStore(mockStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
			appcontroller.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		apiInstance := s.createAPI(t, stateMachine, integration.DefaultReleaseData(), appController)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create request
		req := httptest.NewRequest(http.MethodGet, s.baseURL+"/app/status", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var response types.AppUpgrade
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify the response matches our predefined status
		assert.Equal(t, types.StateRunning, response.Status.State)
		assert.Equal(t, "Upgrading application", response.Status.Description)
		assert.Equal(t, "Upgrade in progress...", response.Logs)
	})

	s.T().Run("Authorization error", func(t *testing.T) {
		// Create state machine
		stateMachine := s.createStateMachine(states.StateAppPreflightsSucceeded)

		// Create simple app controller for auth test

		// Create mock store
		mockStore := &store.MockStore{}
		mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

		appController, err := appcontroller.NewAppController(
			appcontroller.WithStateMachine(stateMachine),
			appcontroller.WithStore(mockStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
			appcontroller.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		apiInstance := s.createAPI(t, stateMachine, integration.DefaultReleaseData(), appController)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create request with invalid token
		req := httptest.NewRequest(http.MethodGet, s.baseURL+"/app/status", nil)
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
}

// Runner function that executes the suite for both install types
func TestAppUpgradeSuite(t *testing.T) {
	installTypes := []struct {
		name               string
		installType        string
		createStateMachine func(initialState statemachine.State) statemachine.Interface
		createAPI          func(t *testing.T, stateMachine statemachine.Interface, rd *release.ReleaseData, appController *appcontroller.AppController) *api.API
		baseURL            string
	}{
		{
			name:        "linux upgrade",
			installType: "linux",
			createStateMachine: func(initialState statemachine.State) statemachine.Interface {
				return linuxupgrade.NewStateMachine(linuxupgrade.WithCurrentState(initialState))
			},
			createAPI: func(t *testing.T, stateMachine statemachine.Interface, rd *release.ReleaseData, appController *appcontroller.AppController) *api.API {
				// Create mock infra manager to avoid filesystem access
				mockInfraManager := &infra.MockInfraManager{}
				mockInfraManager.On("RequiresUpgrade", mock.Anything, mock.Anything).Return(false, nil).Maybe()

				// Create Linux upgrade controller with app controller
				controller, err := linuxupgrade.NewUpgradeController(
					linuxupgrade.WithStateMachine(stateMachine),
					linuxupgrade.WithReleaseData(rd),
					linuxupgrade.WithHelmClient(&helm.MockClient{}),
					linuxupgrade.WithAppController(appController),
					linuxupgrade.WithInfraManager(mockInfraManager),
				)
				require.NoError(t, err)

				apiInstance, err := api.New(types.APIConfig{
					InstallTarget: types.InstallTargetLinux,
					Password:      "password",
					ReleaseData:   rd,
					Mode:          types.ModeUpgrade,
				},
					api.WithLinuxUpgradeController(controller),
					api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
					api.WithLogger(logger.NewDiscardLogger()),
					api.WithHelmClient(&helm.MockClient{}),
				)
				require.NoError(t, err)
				return apiInstance
			},
			baseURL: "/linux/upgrade",
		},
		{
			name:        "kubernetes upgrade",
			installType: "kubernetes",
			createStateMachine: func(initialState statemachine.State) statemachine.Interface {
				return kubernetesupgrade.NewStateMachine(kubernetesupgrade.WithCurrentState(initialState))
			},
			createAPI: func(t *testing.T, stateMachine statemachine.Interface, rd *release.ReleaseData, appController *appcontroller.AppController) *api.API {
				// Create Kubernetes upgrade controller with app controller
				controller, err := kubernetesupgrade.NewUpgradeController(
					kubernetesupgrade.WithStateMachine(stateMachine),
					kubernetesupgrade.WithReleaseData(rd),
					kubernetesupgrade.WithHelmClient(&helm.MockClient{}),
					kubernetesupgrade.WithAppController(appController),
					kubernetesupgrade.WithKubernetesEnvSettings(helmcli.New()),
				)
				require.NoError(t, err)
				return integration.NewTargetKubernetesAPIWithReleaseData(t, types.ModeUpgrade,
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
			testSuite := &AppUpgradeTestSuite{
				installType:        tt.installType,
				createStateMachine: tt.createStateMachine,
				createAPI:          tt.createAPI,
				baseURL:            tt.baseURL,
			}
			suite.Run(t, testSuite)
		})
	}
}
