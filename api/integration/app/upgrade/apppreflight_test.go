package integration

import (
	"context"
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
	apppreflightmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/preflight"
	appreleasemanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/release"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/internal/store"
	apppreflightstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	helmcli "helm.sh/helm/v3/pkg/cli"
)

type AppPreflightTestSuite struct {
	suite.Suite
	installType        string
	createStateMachine func(initialState statemachine.State) statemachine.Interface
	createAPI          func(t *testing.T, stateMachine statemachine.Interface, rd *release.ReleaseData, appController *appcontroller.AppController) *api.API
	baseURL            string
}

func (s *AppPreflightTestSuite) TestGetAppPreflightsStatus() {
	apf := types.AppPreflights{
		Output: &types.PreflightsOutput{
			Pass: []types.PreflightsRecord{
				{
					Title:   "Some App Preflight",
					Message: "All good",
				},
			},
			Fail: []types.PreflightsRecord{
				{
					Title:   "Another App Preflight",
					Message: "Oh no!",
				},
			},
		},
		Titles: []string{
			"Some App Preflight",
			"Another App Preflight",
		},
		Status: types.Status{
			State:       types.StateFailed,
			Description: "An app preflight failed",
		},
	}

	// Create real app preflight manager with in-memory store
	appPreflightManager := apppreflightmanager.NewAppPreflightManager(
		apppreflightmanager.WithAppPreflightStore(
			apppreflightstore.NewMemoryStore(apppreflightstore.WithAppPreflight(apf)),
		),
	)

	// Create mock store
	mockStore := &store.MockStore{}
	mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

	// Create state machine
	stateMachine := s.createStateMachine(states.StateApplicationConfigured)

	// Create real app controller
	appController, err := appcontroller.NewAppController(
		appcontroller.WithAppPreflightManager(appPreflightManager),
		appcontroller.WithStateMachine(stateMachine),
		appcontroller.WithStore(mockStore),
		appcontroller.WithReleaseData(integration.DefaultReleaseData()),
		appcontroller.WithHelmClient(&helm.MockClient{}),
	)
	require.NoError(s.T(), err)

	// Verify the appController is not nil
	require.NotNil(s.T(), appController, "appController should not be nil")

	// Try to call a method to see if it works
	titles, err := appController.GetAppPreflightTitles(context.Background())
	require.NoError(s.T(), err, "GetAppPreflightTitles should not error")
	require.Equal(s.T(), apf.Titles, titles, "titles should match")

	apiInstance := s.createAPI(s.T(), stateMachine, integration.DefaultReleaseData(), appController)

	// Create a router and register the API routes
	router := mux.NewRouter()
	apiInstance.RegisterRoutes(router)

	// Test successful get
	s.T().Run("Success", func(t *testing.T) {
		// Create a request
		req := httptest.NewRequest(http.MethodGet, s.baseURL+"/app-preflights/status", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusOK, rec.Code, "expected status ok, got %d with body %s", rec.Code, rec.Body.String())
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var status types.UpgradeAppPreflightsStatusResponse
		err = json.NewDecoder(rec.Body).Decode(&status)
		require.NoError(t, err)

		// Check the parsed response
		assert.Equal(t, apf.Titles, status.Titles)
		assert.Equal(t, apf.Output, status.Output)
		assert.Equal(t, apf.Status, status.Status)
		assert.False(t, status.HasStrictAppPreflightFailures)
	})

	// Test API endpoint returns hasStrictAppPreflightFailures: true when strict failures exist
	s.T().Run("Success with strict failures", func(t *testing.T) {
		apfStrict := types.AppPreflights{
			Output: &types.PreflightsOutput{
				Pass: []types.PreflightsRecord{
					{
						Title:   "Some Passing Check",
						Message: "All good",
						Strict:  false,
					},
				},
				Fail: []types.PreflightsRecord{
					{
						Title:   "Critical App Requirement",
						Message: "This is a strict failure that blocks installation",
						Strict:  true, // This is the key - strict failure
					},
					{
						Title:   "Non-critical Check",
						Message: "This can be bypassed",
						Strict:  false,
					},
				},
			},
			Titles: []string{
				"Some Passing Check",
				"Critical App Requirement",
				"Non-critical Check",
			},
			Status: types.Status{
				State:       types.StateFailed,
				Description: "App preflights failed with strict failures",
			},
		}

		// Create real app preflight manager with strict failures
		strictAppPreflightManager := apppreflightmanager.NewAppPreflightManager(
			apppreflightmanager.WithAppPreflightStore(
				apppreflightstore.NewMemoryStore(apppreflightstore.WithAppPreflight(apfStrict)),
			),
		)

		// Create mock store with proper app config store
		mockStrictStore := &store.MockStore{}
		mockStrictStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

		// Create state machine
		strictStateMachine := s.createStateMachine(states.StateAppPreflightsFailed)

		// Create real app controller
		strictAppController, err := appcontroller.NewAppController(
			appcontroller.WithAppPreflightManager(strictAppPreflightManager),
			appcontroller.WithStateMachine(strictStateMachine),
			appcontroller.WithStore(mockStrictStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
			appcontroller.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		strictAPIInstance := s.createAPI(t, strictStateMachine, integration.DefaultReleaseData(), strictAppController)

		// Create a router and register the API routes
		strictRouter := mux.NewRouter()
		strictAPIInstance.RegisterRoutes(strictRouter)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, s.baseURL+"/app-preflights/status", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		strictRouter.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusOK, rec.Code, "expected status ok, got %d with body %s", rec.Code, rec.Body.String())
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body
		var strictStatus types.UpgradeAppPreflightsStatusResponse
		err = json.NewDecoder(rec.Body).Decode(&strictStatus)
		require.NoError(t, err)

		// Check the parsed response
		assert.Equal(t, apfStrict.Titles, strictStatus.Titles)
		assert.Equal(t, apfStrict.Output, strictStatus.Output)
		assert.Equal(t, apfStrict.Status, strictStatus.Status)
		assert.True(t, strictStatus.HasStrictAppPreflightFailures)
		assert.True(t, strictStatus.AllowIgnoreAppPreflights) // Hardcoded to true in API handlers
	})

	// Test authorization error
	s.T().Run("Authorization error", func(t *testing.T) {
		// Create a request without authorization header
		req := httptest.NewRequest(http.MethodGet, s.baseURL+"/app-preflights/status", nil)
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

func (s *AppPreflightTestSuite) TestPostRunAppPreflights() {
	s.T().Run("Success", func(t *testing.T) {
		// Mock preflight runner (external dependency)
		runner := &preflights.MockPreflightRunner{}

		// Mock the preflight runner expectations
		runner.On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).Return(&types.PreflightsOutput{
			Pass: []types.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
		}, "", nil)

		// Create real app preflight manager with mock runner
		appPreflightManager := apppreflightmanager.NewAppPreflightManager(
			apppreflightmanager.WithAppPreflightStore(
				apppreflightstore.NewMemoryStore(),
			),
			apppreflightmanager.WithPreflightRunner(runner),
		)

		// Create mock store with proper app config store
		mockStore := &store.MockStore{}
		mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

		// Create mock app release manager
		mockAppReleaseManager := &appreleasemanager.MockAppReleaseManager{}
		mockAppReleaseManager.On("ExtractAppPreflightSpec", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&troubleshootv1beta2.PreflightSpec{
			Analyzers: []*troubleshootv1beta2.Analyze{
				{
					ClusterVersion: &troubleshootv1beta2.ClusterVersion{
						Outcomes: []*troubleshootv1beta2.Outcome{
							{
								Pass: &troubleshootv1beta2.SingleOutcome{
									Message: "Kubernetes version is supported",
								},
							},
						},
					},
				},
			},
		}, nil)

		// Create state machine with ApplicationConfigured state
		stateMachine := s.createStateMachine(states.StateApplicationConfigured)

		// Create app controller
		appController, err := appcontroller.NewAppController(
			appcontroller.WithAppPreflightManager(appPreflightManager),
			appcontroller.WithAppReleaseManager(mockAppReleaseManager),
			appcontroller.WithStateMachine(stateMachine),
			appcontroller.WithStore(mockStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
			appcontroller.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		// Use the test suite's createAPI to handle both Linux and Kubernetes properly
		apiInstance := s.createAPI(t, stateMachine, integration.DefaultReleaseData(), appController)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request (no body needed)
		req := httptest.NewRequest(http.MethodPost, s.baseURL+"/app-preflights/run", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusOK, rec.Code, "expected status ok, got %d with body %s", rec.Code, rec.Body.String())
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		// Parse the response body (should be the status response)
		var response types.UpgradeAppPreflightsStatusResponse
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)

		// Verify the mocks were called
		mockStore.AppConfigMockStore.AssertExpectations(t)
		mockAppReleaseManager.AssertExpectations(t)

		// Wait for the state machine to reach success state
		// This ensures the goroutine has completed before we check mock expectations
		assert.Eventually(t, func() bool {
			return stateMachine.CurrentState() == states.StateAppPreflightsSucceeded
		}, 5*time.Second, 100*time.Millisecond, "state machine should reach StateAppPreflightsSucceeded")

		// Now verify the runner mock was called
		runner.AssertExpectations(t)
	})

	s.T().Run("Invalid state", func(t *testing.T) {
		// Create state machine with wrong state
		stateMachine := s.createStateMachine(states.StateNew)

		// Create mock store with GetConfigValues expectation
		mockStore := &store.MockStore{}
		mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

		// Create simple app controller
		appController, err := appcontroller.NewAppController(
			appcontroller.WithStateMachine(stateMachine),
			appcontroller.WithStore(mockStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
			appcontroller.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		// Use the test suite's createAPI to handle both Linux and Kubernetes properly
		apiInstance := s.createAPI(t, stateMachine, integration.DefaultReleaseData(), appController)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, s.baseURL+"/app-preflights/run", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		require.Equal(t, http.StatusConflict, rec.Code)

		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusConflict, apiError.StatusCode)
	})

	s.T().Run("Authorization error", func(t *testing.T) {
		// Create state machine
		stateMachine := s.createStateMachine(states.StateApplicationConfigured)

		// Create mock store with GetConfigValues expectation
		mockStore := &store.MockStore{}
		mockStore.AppConfigMockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)

		// Create simple app controller
		appController, err := appcontroller.NewAppController(
			appcontroller.WithStateMachine(stateMachine),
			appcontroller.WithStore(mockStore),
			appcontroller.WithReleaseData(integration.DefaultReleaseData()),
			appcontroller.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		// Use the test suite's createAPI to handle both Linux and Kubernetes properly
		apiInstance := s.createAPI(t, stateMachine, integration.DefaultReleaseData(), appController)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request with invalid token
		req := httptest.NewRequest(http.MethodPost, s.baseURL+"/app-preflights/run", nil)
		req.Header.Set("Authorization", "Bearer INVALID_TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}

// Runner function that executes the suite for both install types
func TestAppPreflightSuite(t *testing.T) {
	installTypes := []struct {
		name               string
		installType        string
		createStateMachine func(initialState statemachine.State) statemachine.Interface
		createAPI          func(t *testing.T, stateMachine statemachine.Interface, rd *release.ReleaseData, appController *appcontroller.AppController) *api.API
		baseURL            string
	}{
		{
			name:        "linux upgrade app preflights",
			installType: "linux",
			createStateMachine: func(initialState statemachine.State) statemachine.Interface {
				return linuxupgrade.NewStateMachine(linuxupgrade.WithCurrentState(initialState))
			},
			createAPI: func(t *testing.T, stateMachine statemachine.Interface, rd *release.ReleaseData, appController *appcontroller.AppController) *api.API {
				// Create RuntimeConfig with temp directory for Linux
				rc := runtimeconfig.New(nil)
				rc.SetDataDir(t.TempDir())

				// Create mock infra manager to avoid filesystem access
				mockInfraManager := &infra.MockInfraManager{}
				mockInfraManager.On("RequiresUpgrade", mock.Anything, mock.Anything).Return(false, nil)

				// Create Linux upgrade controller with app controller
				controller, err := linuxupgrade.NewUpgradeController(
					linuxupgrade.WithRuntimeConfig(rc),
					linuxupgrade.WithStateMachine(stateMachine),
					linuxupgrade.WithReleaseData(&release.ReleaseData{
						EmbeddedClusterConfig: &ecv1beta1.Config{},
						ChannelRelease: &release.ChannelRelease{
							DefaultDomains: release.Domains{
								ReplicatedAppDomain: "replicated.example.com",
								ProxyRegistryDomain: "some-proxy.example.com",
							},
						},
						AppConfig: &kotsv1beta1.Config{},
					}),
					linuxupgrade.WithHelmClient(&helm.MockClient{}),
					linuxupgrade.WithAppController(appController),
					linuxupgrade.WithInfraManager(mockInfraManager),
				)
				require.NoError(t, err)

				// Create the API with runtime config in the API config
				apiInstance, err := api.New(types.APIConfig{
					InstallTarget: types.InstallTargetLinux,
					Password:      "password",
					LinuxConfig: types.LinuxConfig{
						RuntimeConfig: rc,
					},
					ReleaseData: rd,
					Mode:        types.ModeUpgrade,
				},
					api.WithLinuxUpgradeController(controller),
					api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
					api.WithLogger(logger.NewDiscardLogger()), // Prevent permission errors from log file creation
					api.WithHelmClient(&helm.MockClient{}),
				)
				require.NoError(t, err)
				return apiInstance
			},
			baseURL: "/linux/upgrade",
		},
		{
			name:        "kubernetes upgrade app preflights",
			installType: "kubernetes",
			createStateMachine: func(initialState statemachine.State) statemachine.Interface {
				return kubernetesupgrade.NewStateMachine(kubernetesupgrade.WithCurrentState(initialState))
			},
			createAPI: func(t *testing.T, stateMachine statemachine.Interface, rd *release.ReleaseData, appController *appcontroller.AppController) *api.API {
				// Create mock installation for Kubernetes
				mockInstallation := &kubernetesinstallation.MockInstallation{}
				mockInstallation.On("ProxySpec").Return((*ecv1beta1.ProxySpec)(nil))
				mockInstallation.On("PathToEmbeddedBinary", "kubectl-preflight").Return("/tmp/kubectl-preflight", nil)

				// Create Kubernetes upgrade controller with app controller
				controller, err := kubernetesupgrade.NewUpgradeController(
					kubernetesupgrade.WithStateMachine(stateMachine),
					kubernetesupgrade.WithReleaseData(rd),
					kubernetesupgrade.WithHelmClient(&helm.MockClient{}),
					kubernetesupgrade.WithAppController(appController),
					kubernetesupgrade.WithKubernetesEnvSettings(helmcli.New()),
				)
				require.NoError(t, err)

				// Create the API with kubernetes config in the API config
				apiInstance, err := api.New(types.APIConfig{
					InstallTarget: types.InstallTargetKubernetes,
					Password:      "password",
					KubernetesConfig: types.KubernetesConfig{
						Installation: mockInstallation,
					},
					ReleaseData: rd,
					Mode:        types.ModeUpgrade,
				},
					api.WithKubernetesUpgradeController(controller),
					api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
					api.WithLogger(logger.NewDiscardLogger()),
					api.WithHelmClient(&helm.MockClient{}),
				)
				require.NoError(t, err)
				return apiInstance
			},
			baseURL: "/kubernetes/upgrade",
		},
	}

	for _, tt := range installTypes {
		t.Run(tt.name, func(t *testing.T) {
			testSuite := &AppPreflightTestSuite{
				installType:        tt.installType,
				createStateMachine: tt.createStateMachine,
				createAPI:          tt.createAPI,
				baseURL:            tt.baseURL,
			}
			suite.Run(t, testSuite)
		})
	}
}
