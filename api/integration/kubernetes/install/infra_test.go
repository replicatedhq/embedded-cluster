package install

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	kubernetesinstall "github.com/replicatedhq/embedded-cluster/api/controllers/kubernetes/install"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	"github.com/replicatedhq/embedded-cluster/api/integration/assets"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	kubernetesinfra "github.com/replicatedhq/embedded-cluster/api/internal/managers/kubernetes/infra"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
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

// Test the kubernetes setupInfra endpoint runs infrastructure setup correctly
func TestKubernetesPostSetupInfra(t *testing.T) {
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

	t.Run("Success", func(t *testing.T) {
		hostname, err := os.Hostname()
		require.NoError(t, err)

		// Create mocks
		helmMock := &helm.MockClient{}
		fakeKcli := clientfake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(integration.NewTestControllerNode(hostname)).
			WithStatusSubresource(&ecv1beta1.Installation{}, &apiextensionsv1.CustomResourceDefinition{}).
			WithInterceptorFuncs(integration.NewTestInterceptorFuncs()).
			Build()
		fakeMcli := metadatafake.NewSimpleMetadataClient(metascheme)

		// Create a runtime config
		ki := kubernetesinstallation.New(nil)

		// Create infra manager with mocks
		infraManager := kubernetesinfra.NewInfraManager(
			kubernetesinfra.WithKubeClient(fakeKcli),
			kubernetesinfra.WithMetadataClient(fakeMcli),
			kubernetesinfra.WithHelmClient(helmMock),
			kubernetesinfra.WithLicense(assets.LicenseData),
			kubernetesinfra.WithKotsCLIInstaller(&MockKotsCLIInstaller{}),
			kubernetesinfra.WithReleaseData(&release.ReleaseData{
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

		mock.InOrder(
			helmMock.On("Install", mock.Anything, mock.Anything).Times(1).Return(nil, nil), // 1 addon
			helmMock.On("Close").Return(nil),
		)

		// Create an install controller with the mocked managers
		installController, err := kubernetesinstall.NewInstallController(
			kubernetesinstall.WithInstallation(ki),
			kubernetesinstall.WithStateMachine(kubernetesinstall.NewStateMachine(kubernetesinstall.WithCurrentState(kubernetesinstall.StateInstallationConfigured))),
			kubernetesinstall.WithInfraManager(infraManager),
			kubernetesinstall.WithReleaseData(&release.ReleaseData{
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
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance, err := api.New(
			types.APIConfig{
				Password: "password",
			},
			api.WithKubernetesInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		req := httptest.NewRequest(http.MethodPost, "/kubernetes/install/infra/setup", nil)
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
			req := httptest.NewRequest(http.MethodGet, "/kubernetes/install/infra/status", nil)
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
		helmMock.AssertExpectations(t)

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
		// assert.Contains(t, infra.Logs, "[metadata]") // record installation
		assert.Contains(t, infra.Logs, "[addons]")
		assert.Len(t, infra.Components, 1) // admin console addon
	})

	// Test authorization
	t.Run("Authorization error", func(t *testing.T) {
		// Create the API
		apiInstance, err := api.New(
			types.APIConfig{
				Password: "password",
			},
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		req := httptest.NewRequest(http.MethodPost, "/kubernetes/install/infra/setup", nil)
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

	// Addon install error
	t.Run("addon install error", func(t *testing.T) {
		hostname, err := os.Hostname()
		require.NoError(t, err)

		// Create mocks
		helmMock := &helm.MockClient{}
		fakeKcli := clientfake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(integration.NewTestControllerNode(hostname)).
			WithStatusSubresource(&ecv1beta1.Installation{}, &apiextensionsv1.CustomResourceDefinition{}).
			WithInterceptorFuncs(integration.NewTestInterceptorFuncs()).
			Build()
		fakeMcli := metadatafake.NewSimpleMetadataClient(metascheme)

		// Create a runtime config
		ki := kubernetesinstallation.New(nil)

		// Create infra manager with mocks
		infraManager := kubernetesinfra.NewInfraManager(
			kubernetesinfra.WithKubeClient(fakeKcli),
			kubernetesinfra.WithMetadataClient(fakeMcli),
			kubernetesinfra.WithHelmClient(helmMock),
			kubernetesinfra.WithLicense(assets.LicenseData),
			kubernetesinfra.WithKotsCLIInstaller(&MockKotsCLIInstaller{}),
			kubernetesinfra.WithReleaseData(&release.ReleaseData{
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

		mock.InOrder(
			helmMock.On("Install", mock.Anything, mock.Anything).Times(1).Return(nil, assert.AnError), // 1 addon
			helmMock.On("Close").Return(nil),
		)

		// Create an install controller with the mocked managers
		installController, err := kubernetesinstall.NewInstallController(
			kubernetesinstall.WithInstallation(ki),
			kubernetesinstall.WithStateMachine(kubernetesinstall.NewStateMachine(kubernetesinstall.WithCurrentState(kubernetesinstall.StateInstallationConfigured))),
			kubernetesinstall.WithInfraManager(infraManager),
			kubernetesinstall.WithReleaseData(&release.ReleaseData{
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
		require.NoError(t, err)

		// Create the API with the install controller
		apiInstance, err := api.New(
			types.APIConfig{
				Password: "password",
			},
			api.WithKubernetesInstallController(installController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		req := httptest.NewRequest(http.MethodPost, "/kubernetes/install/infra/setup", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)

		// The status should eventually be set to failed due to k0s install error
		assert.Eventually(t, func() bool {
			// Create a request to get infra status
			req := httptest.NewRequest(http.MethodGet, "/kubernetes/install/infra/status", nil)
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
			return infra.Status.State == types.StateFailed && strings.Contains(infra.Status.Description, assert.AnError.Error())
		}, 10*time.Second, 100*time.Millisecond, "Infrastructure setup did not fail in time")

		// Verify that the mock expectations were met
		helmMock.AssertExpectations(t)
	})
}

type MockKotsCLIInstaller struct {
}

func (m *MockKotsCLIInstaller) Install(opts kotscli.InstallOptions) error {
	return nil
}
