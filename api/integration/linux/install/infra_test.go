package install

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/api"
	linuxinstall "github.com/replicatedhq/embedded-cluster/api/controllers/linux/install"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	"github.com/replicatedhq/embedded-cluster/api/integration/assets"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	linuxinfra "github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/preflight"
	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
	linuxpreflightstore "github.com/replicatedhq/embedded-cluster/api/internal/store/linux/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	metadatafake "k8s.io/client-go/metadata/fake"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Test the linux setupInfra endpoint runs infrastructure setup correctly
func TestLinuxPostSetupInfra(t *testing.T) {
	// Create schemes
	scheme := runtime.NewScheme()
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apiextensionsv1.AddToScheme(scheme))

	metascheme := metadatafake.NewTestScheme()
	require.NoError(t, metav1.AddMetaToScheme(metascheme))
	require.NoError(t, corev1.AddToScheme(metascheme))

	appConfig := kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "network-config",
					Title: "{{ print \"Network Configuration\" }}",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "service-cidr",
							Type:    "text",
							Title:   "{{ upper \"service cidr\" }}",
							Default: multitype.FromString("{{ print \"10.96.0.0/12\" }}"),
							Value:   multitype.FromString("{{ print \"10.96.0.0/12\" }}"),
						},
						{
							Name:    "pod-cidr",
							Type:    "text",
							Title:   "{{ upper \"pod cidr\" }}",
							Default: multitype.FromString("{{ print \"10.244.0.0/16\" }}"),
							Value:   multitype.FromString("{{ print \"10.244.0.0/16\" }}"),
						},
					},
				},
			},
		},
	}

	t.Run("Success", func(t *testing.T) {
		hostname, err := os.Hostname()
		require.NoError(t, err)

		// Create mocks
		k0sMock := &k0s.MockK0s{}
		helmMock := &helm.MockClient{}
		hostutilsMock := &hostutils.MockHostUtils{}
		fakeKcli := clientfake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(integration.NewTestControllerNode(hostname)).
			WithStatusSubresource(&ecv1beta1.Installation{}, &apiextensionsv1.CustomResourceDefinition{}).
			WithInterceptorFuncs(integration.NewTestInterceptorFuncs()).
			Build()
		fakeMcli := metadatafake.NewSimpleMetadataClient(metascheme)

		// Create a runtime config
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())
		rc.SetNetworkSpec(ecv1beta1.NetworkSpec{
			NetworkInterface: "eth0",
			ServiceCIDR:      "10.96.0.0/12",
			PodCIDR:          "10.244.0.0/16",
		})

		// Create host preflights with successful status
		hpf := types.HostPreflights{}
		hpf.Status = types.Status{
			State:       types.StateSucceeded,
			Description: "Host preflights succeeded",
		}

		// Create host preflights manager
		pfManager := preflight.NewHostPreflightManager(
			preflight.WithHostPreflightStore(linuxpreflightstore.NewMemoryStore(linuxpreflightstore.WithHostPreflight(hpf))),
		)

		// Create infra manager with mocks
		infraManager := linuxinfra.NewInfraManager(
			linuxinfra.WithK0s(k0sMock),
			linuxinfra.WithKubeClient(fakeKcli),
			linuxinfra.WithMetadataClient(fakeMcli),
			linuxinfra.WithHelmClient(helmMock),
			linuxinfra.WithLicense(assets.LicenseData),
			linuxinfra.WithHostUtils(hostutilsMock),
			linuxinfra.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease: &release.ChannelRelease{
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.example.com",
						ProxyRegistryDomain: "some-proxy.example.com",
					},
				},
				AppConfig: &appConfig,
			}),
		)

		// Setup mock expectations
		k0sConfig := &k0sv1beta1.ClusterConfig{
			Spec: &k0sv1beta1.ClusterSpec{
				Network: &k0sv1beta1.Network{
					PodCIDR:     "10.244.0.0/16",
					ServiceCIDR: "10.96.0.0/12",
				},
			},
		}
		mock.InOrder(
			k0sMock.On("IsInstalled").Return(false, nil),
			k0sMock.On("WriteK0sConfig", mock.Anything, "eth0", "", "10.244.0.0/16", "10.96.0.0/12", mock.Anything, mock.Anything).Return(k0sConfig, nil),
			hostutilsMock.On("CreateSystemdUnitFiles", mock.Anything, mock.Anything, rc, false).Return(nil),
			k0sMock.On("Install", rc).Return(nil),
			k0sMock.On("WaitForK0s").Return(nil),
			hostutilsMock.On("AddInsecureRegistry", mock.Anything).Return(nil),
			helmMock.On("Install", mock.Anything, mock.Anything).Times(4).Return(nil, nil), // 4 addons
		)

		// Create an install controller with the mocked managers
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithRuntimeConfig(rc),
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(states.StateHostPreflightsSucceeded))),
			linuxinstall.WithHostPreflightManager(pfManager),
			linuxinstall.WithInfraManager(infraManager),
			linuxinstall.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease: &release.ChannelRelease{
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.example.com",
						ProxyRegistryDomain: "some-proxy.example.com",
					},
				},
				AppConfig: &appConfig,
			}),
			linuxinstall.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewTargetLinuxAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request with proper JSON body
		requestBody := types.LinuxInfraSetupRequest{
			IgnoreHostPreflights: false,
		}
		reqBodyBytes, err := json.Marshal(requestBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/linux/install/infra/setup", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Authorization", "Bearer TOKEN")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)

		t.Logf("Response body: %s", rec.Body.String())

		// Parse the response body
		var infra types.Infra
		err = json.NewDecoder(rec.Body).Decode(&infra)
		require.NoError(t, err)

		// Verify that the status is not pending. We cannot check for an end state here because the hots config is async
		// so the state might have moved from running to a final state before we get the response.
		assert.NotEqual(t, types.StatePending, infra.Status.State)

		// Helper function to get infra status
		getInfraStatus := func() types.Infra {
			// Create a request to get infra status
			req := httptest.NewRequest(http.MethodGet, "/linux/install/infra/status", nil)
			req.Header.Set("Authorization", "Bearer TOKEN")
			rec := httptest.NewRecorder()

			// Serve the request
			router.ServeHTTP(rec, req)

			// Check the response
			assert.Equal(t, http.StatusOK, rec.Code)

			// Parse the response body
			var infra types.Infra
			err = json.NewDecoder(rec.Body).Decode(&infra)
			require.NoError(t, err)

			// Log the infra status
			t.Logf("Infra Status: %s, Description: %s", infra.Status.State, infra.Status.Description)

			return infra
		}

		// The status should eventually be set to succeeded in a goroutine
		assert.Eventually(t, func() bool {
			infra := getInfraStatus()

			// Fail the test if the status is Failed
			if infra.Status.State == types.StateFailed {
				t.Fatalf("Infrastructure setup failed: %s", infra.Status.Description)
			}

			return infra.Status.State == types.StateSucceeded
		}, 30*time.Second, 500*time.Millisecond, "Infrastructure setup did not succeed in time")

		// Verify that the mock expectations were met
		k0sMock.AssertExpectations(t)
		hostutilsMock.AssertExpectations(t)
		helmMock.AssertExpectations(t)

		// Verify installation was created
		gotInst, err := kubeutils.GetLatestInstallation(t.Context(), fakeKcli)
		require.NoError(t, err)
		assert.Equal(t, ecv1beta1.InstallationStateInstalled, gotInst.Status.State)

		// Verify version metadata configmap was created
		var gotConfigmap corev1.ConfigMap
		err = fakeKcli.Get(t.Context(), client.ObjectKey{Namespace: "embedded-cluster", Name: "version-metadata-0-0-0"}, &gotConfigmap)
		require.NoError(t, err)

		// Verify kotsadm namespace and kotsadm-password secret were created
		var gotKotsadmNamespace corev1.Namespace
		err = fakeKcli.Get(t.Context(), client.ObjectKey{Name: constants.KotsadmNamespace}, &gotKotsadmNamespace)
		require.NoError(t, err)

		var gotKotsadmPasswordSecret corev1.Secret
		err = fakeKcli.Get(t.Context(), client.ObjectKey{Namespace: constants.KotsadmNamespace, Name: "kotsadm-password"}, &gotKotsadmPasswordSecret)
		require.NoError(t, err)
		assert.NotEmpty(t, gotKotsadmPasswordSecret.Data["passwordBcrypt"])

		// Get infra status again and verify more details
		infra = getInfraStatus()
		assert.Contains(t, infra.Logs, "[k0s]")
		assert.Contains(t, infra.Logs, "[metadata]")
		assert.Contains(t, infra.Logs, "[addons]")
		assert.Contains(t, infra.Logs, "[extensions]")
		assert.Len(t, infra.Components, 6)
	})

	// Test authorization
	t.Run("Authorization error", func(t *testing.T) {
		// Create the API
		apiInstance := integration.NewTargetLinuxAPIWithReleaseData(t,
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request with proper JSON body
		requestBody := types.LinuxInfraSetupRequest{
			IgnoreHostPreflights: false,
		}
		reqBodyBytes, err := json.Marshal(requestBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/linux/install/infra/setup", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Authorization", "Bearer NOT_A_TOKEN")
		req.Header.Set("Content-Type", "application/json")
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

	// Test preflight bypass with CLI flag allowing it - should succeed
	t.Run("Preflight bypass allowed by CLI flag", func(t *testing.T) {
		// Create host preflights with failed status
		hpf := types.HostPreflights{}
		hpf.Status = types.Status{
			State:       types.StateFailed,
			Description: "Host preflights failed",
		}

		// Create managers
		pfManager := preflight.NewHostPreflightManager(
			preflight.WithHostPreflightStore(linuxpreflightstore.NewMemoryStore(linuxpreflightstore.WithHostPreflight(hpf))),
		)

		// Create an install controller with CLI flag allowing bypass
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(states.StateHostPreflightsFailed))),
			linuxinstall.WithHostPreflightManager(pfManager),
			linuxinstall.WithAllowIgnoreHostPreflights(true), // CLI flag allows bypass
			linuxinstall.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease:        &release.ChannelRelease{},
				AppConfig:             &appConfig,
			}),
			linuxinstall.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewTargetLinuxAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request with ignoreHostPreflights=true
		requestBody := types.LinuxInfraSetupRequest{
			IgnoreHostPreflights: true,
		}
		reqBodyBytes, err := json.Marshal(requestBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/linux/install/infra/setup", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Authorization", "Bearer TOKEN")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response - should succeed because CLI flag allows bypass
		assert.Equal(t, http.StatusOK, rec.Code)

		t.Logf("Response body: %s", rec.Body.String())
	})

	// Test preflight bypass with CLI flag NOT allowing it - should fail
	t.Run("Preflight bypass denied by CLI flag", func(t *testing.T) {
		// Create host preflights with failed status
		hpf := types.HostPreflights{}
		hpf.Status = types.Status{
			State:       types.StateFailed,
			Description: "Host preflights failed",
		}

		// Create managers
		pfManager := preflight.NewHostPreflightManager(
			preflight.WithHostPreflightStore(linuxpreflightstore.NewMemoryStore(linuxpreflightstore.WithHostPreflight(hpf))),
		)

		// Create an install controller with CLI flag NOT allowing bypass
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(states.StateHostPreflightsFailed))),
			linuxinstall.WithHostPreflightManager(pfManager),
			linuxinstall.WithAllowIgnoreHostPreflights(false), // CLI flag does NOT allow bypass
			linuxinstall.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease:        &release.ChannelRelease{},
				AppConfig:             &appConfig,
			}),
			linuxinstall.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewTargetLinuxAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request with ignoreHostPreflights=true
		requestBody := types.LinuxInfraSetupRequest{
			IgnoreHostPreflights: true,
		}
		reqBodyBytes, err := json.Marshal(requestBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/linux/install/infra/setup", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Authorization", "Bearer TOKEN")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response - should fail because CLI flag does NOT allow bypass
		assert.Equal(t, http.StatusBadRequest, rec.Code)

		t.Logf("Response body: %s", rec.Body.String())

		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, apiError.StatusCode)
		assert.Contains(t, apiError.Message, "preflight checks failed")
	})

	// Test client not requesting bypass but preflights failed - should fail
	t.Run("Client not requesting bypass with failed preflights", func(t *testing.T) {
		// Create host preflights with failed status
		hpf := types.HostPreflights{}
		hpf.Status = types.Status{
			State:       types.StateFailed,
			Description: "Host preflights failed",
		}

		// Create managers
		pfManager := preflight.NewHostPreflightManager(
			preflight.WithHostPreflightStore(linuxpreflightstore.NewMemoryStore(linuxpreflightstore.WithHostPreflight(hpf))),
		)

		// Create an install controller with CLI flag allowing bypass
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(states.StateHostPreflightsFailed))),
			linuxinstall.WithHostPreflightManager(pfManager),
			linuxinstall.WithAllowIgnoreHostPreflights(true), // CLI flag allows bypass
			linuxinstall.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease:        &release.ChannelRelease{},
				AppConfig:             &appConfig,
			}),
			linuxinstall.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewTargetLinuxAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request with ignoreHostPreflights=false (client not requesting bypass)
		requestBody := types.LinuxInfraSetupRequest{
			IgnoreHostPreflights: false,
		}
		reqBodyBytes, err := json.Marshal(requestBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/linux/install/infra/setup", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Authorization", "Bearer TOKEN")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response - should fail because client is not requesting bypass
		assert.Equal(t, http.StatusBadRequest, rec.Code)

		t.Logf("Response body: %s", rec.Body.String())

		// Parse the response body
		var apiError types.APIError
		err = json.NewDecoder(rec.Body).Decode(&apiError)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, apiError.StatusCode)
		assert.Contains(t, apiError.Message, "preflight checks failed")
	})

	// Test preflight checks not completed
	t.Run("Preflight checks not completed", func(t *testing.T) {
		// Create host preflights with running status (not completed)
		hpf := types.HostPreflights{}
		hpf.Status = types.Status{
			State:       types.StateRunning,
			Description: "Host preflights running",
		}

		// Create managers
		pfManager := preflight.NewHostPreflightManager(
			preflight.WithHostPreflightStore(linuxpreflightstore.NewMemoryStore(linuxpreflightstore.WithHostPreflight(hpf))),
		)

		// Create an install controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(states.StateHostPreflightsRunning))),
			linuxinstall.WithHostPreflightManager(pfManager),
			linuxinstall.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease:        &release.ChannelRelease{},
				AppConfig:             &appConfig,
			}),
			linuxinstall.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewTargetLinuxAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request with proper JSON body
		requestBody := types.LinuxInfraSetupRequest{
			IgnoreHostPreflights: false,
		}
		reqBodyBytes, err := json.Marshal(requestBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/linux/install/infra/setup", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Authorization", "Bearer TOKEN")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Contains(t, rec.Body.String(), "invalid transition")
	})

	// Test k0s already installed error
	t.Run("K0s already installed", func(t *testing.T) {
		// Create a runtime config
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())
		rc.SetNetworkSpec(ecv1beta1.NetworkSpec{
			NetworkInterface: "eth0",
		})

		// Create host preflights with successful status
		hpf := types.HostPreflights{}
		hpf.Status = types.Status{
			State:       types.StateSucceeded,
			Description: "Host preflights succeeded",
		}

		// Create managers
		pfManager := preflight.NewHostPreflightManager(
			preflight.WithHostPreflightStore(linuxpreflightstore.NewMemoryStore(linuxpreflightstore.WithHostPreflight(hpf))),
		)

		// Create an install controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithRuntimeConfig(rc),
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(states.StateSucceeded))),
			linuxinstall.WithHostPreflightManager(pfManager),
			linuxinstall.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease:        &release.ChannelRelease{},
				AppConfig:             &appConfig,
			}),
			linuxinstall.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewTargetLinuxAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request with proper JSON body
		requestBody := types.LinuxInfraSetupRequest{
			IgnoreHostPreflights: false,
		}
		reqBodyBytes, err := json.Marshal(requestBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/linux/install/infra/setup", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Authorization", "Bearer TOKEN")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Contains(t, rec.Body.String(), "invalid transition")
	})

	// Test k0s install error
	t.Run("K0s install error", func(t *testing.T) {
		// Create mocks
		k0sMock := &k0s.MockK0s{}
		hostutilsMock := &hostutils.MockHostUtils{}

		// Create a runtime config
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())
		rc.SetNetworkSpec(ecv1beta1.NetworkSpec{
			NetworkInterface: "eth0",
			ServiceCIDR:      "10.96.0.0/12",
			PodCIDR:          "10.244.0.0/16",
		})

		// Create host preflights with successful status
		hpf := types.HostPreflights{}
		hpf.Status = types.Status{
			State:       types.StateSucceeded,
			Description: "Host preflights succeeded",
		}

		// Create managers
		pfManager := preflight.NewHostPreflightManager(
			preflight.WithHostPreflightStore(linuxpreflightstore.NewMemoryStore(linuxpreflightstore.WithHostPreflight(hpf))),
		)
		infraManager := linuxinfra.NewInfraManager(
			linuxinfra.WithK0s(k0sMock),
			linuxinfra.WithHostUtils(hostutilsMock),
			linuxinfra.WithLicense(assets.LicenseData),
		)

		// Setup k0s mock expectations with failure
		k0sConfig := &k0sv1beta1.ClusterConfig{}
		mock.InOrder(
			k0sMock.On("IsInstalled").Return(false, nil),
			k0sMock.On("WriteK0sConfig", mock.Anything, "eth0", "", "10.244.0.0/16", "10.96.0.0/12", mock.Anything, mock.Anything).Return(k0sConfig, nil),
			hostutilsMock.On("CreateSystemdUnitFiles", mock.Anything, mock.Anything, rc, false).Return(nil),
			k0sMock.On("Install", mock.Anything).Return(errors.New("failed to install k0s")),
		)

		// Create an install controller
		installController, err := linuxinstall.NewInstallController(
			linuxinstall.WithHostPreflightManager(pfManager),
			linuxinstall.WithInfraManager(infraManager),
			linuxinstall.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{},
				ChannelRelease:        &release.ChannelRelease{},
				AppConfig:             &appConfig,
			}),
			linuxinstall.WithRuntimeConfig(rc),
			linuxinstall.WithStateMachine(linuxinstall.NewStateMachine(linuxinstall.WithCurrentState(states.StateHostPreflightsSucceeded))),
			linuxinstall.WithHelmClient(&helm.MockClient{}),
		)
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance := integration.NewTargetLinuxAPIWithReleaseData(t,
			api.WithLinuxInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)

		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request with proper JSON body
		requestBody := types.LinuxInfraSetupRequest{
			IgnoreHostPreflights: false,
		}
		reqBodyBytes, err := json.Marshal(requestBody)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/linux/install/infra/setup", bytes.NewReader(reqBodyBytes))
		req.Header.Set("Authorization", "Bearer TOKEN")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)

		// The status should eventually be set to failed due to k0s install error
		assert.Eventually(t, func() bool {
			// Create a request to get infra status
			req := httptest.NewRequest(http.MethodGet, "/linux/install/infra/status", nil)
			req.Header.Set("Authorization", "Bearer TOKEN")
			rec := httptest.NewRecorder()

			// Serve the request
			router.ServeHTTP(rec, req)

			// Check the response
			assert.Equal(t, http.StatusOK, rec.Code)

			// Parse the response body
			var infra types.Infra
			err = json.NewDecoder(rec.Body).Decode(&infra)
			require.NoError(t, err)

			t.Logf("Infra Status: %s, Description: %s", infra.Status.State, infra.Status.Description)
			return infra.Status.State == types.StateFailed && strings.Contains(infra.Status.Description, "failed to install k0s")
		}, 10*time.Second, 100*time.Millisecond, "Infrastructure setup did not fail in time")

		// Verify that the mock expectations were met
		k0sMock.AssertExpectations(t)
		hostutilsMock.AssertExpectations(t)
	})
}
