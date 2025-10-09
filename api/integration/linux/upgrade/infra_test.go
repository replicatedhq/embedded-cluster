package upgrade

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/replicatedhq/embedded-cluster/api"
	linuxupgrade "github.com/replicatedhq/embedded-cluster/api/controllers/linux/upgrade"
	"github.com/replicatedhq/embedded-cluster/api/integration"
	"github.com/replicatedhq/embedded-cluster/api/integration/assets"
	"github.com/replicatedhq/embedded-cluster/api/integration/auth"
	linuxinfra "github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/infra"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/upgrade"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	metadatafake "k8s.io/client-go/metadata/fake"
	nodeutil "k8s.io/component-helpers/node/util"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Test the linux requires-upgrade endpoint
func TestLinuxGetRequiresInfraUpgrade(t *testing.T) {
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
							Default: multitype.FromString("default"),
							Value:   multitype.FromString("value"),
						},
					},
				},
			},
		},
	}

	hostname, err := nodeutil.GetHostname("")
	require.NoError(t, err)

	// Create an existing installation that will be used for the upgrade
	existingInstallation := &ecv1beta1.Installation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ecv1beta1.GroupVersion.String(),
			Kind:       "Installation",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "20250101000000",
		},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "v1.0.0",
			},
		},
		Status: ecv1beta1.InstallationStatus{
			State: ecv1beta1.InstallationStateInstalled,
		},
	}

	t.Run("Requires upgrade returns true", func(t *testing.T) {
		// Create fake k8s clients
		fakeKcli := clientfake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(integration.NewTestControllerNode(hostname), existingInstallation).
			WithStatusSubresource(&ecv1beta1.Installation{}, &apiextensionsv1.CustomResourceDefinition{}).
			WithInterceptorFuncs(integration.NewTestInterceptorFuncs()).
			Build()
		fakeMcli := metadatafake.NewSimpleMetadataClient(metascheme)

		helmMock := &helm.MockClient{}

		// Create a runtime config
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		// Create real infra manager with mocked components
		infraManager := linuxinfra.NewInfraManager(
			linuxinfra.WithKubeClient(fakeKcli),
			linuxinfra.WithMetadataClient(fakeMcli),
			linuxinfra.WithHelmClient(helmMock),
			linuxinfra.WithLicense(assets.LicenseData),
			linuxinfra.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{
						Version: "v1.1.0", // Different version means upgrade required
					},
				},
				ChannelRelease: &release.ChannelRelease{},
				AppConfig:      &appConfig,
			}),
		)

		// Create an upgrade controller
		upgradeController, err := linuxupgrade.NewUpgradeController(
			linuxupgrade.WithRuntimeConfig(rc),
			linuxupgrade.WithInfraManager(infraManager),
			linuxupgrade.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{
						Version: "v1.1.0",
					},
				},
				ChannelRelease: &release.ChannelRelease{},
				AppConfig:      &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with Mode and Target set for upgrade routes
		password := "password"
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
		require.NoError(t, err)
		cfg := types.APIConfig{
			Password:     password,
			PasswordHash: passwordHash,
			ReleaseData:  integration.DefaultReleaseData(),
			Mode:         types.ModeUpgrade,
			Target:       types.TargetLinux,
		}
		apiInstance, err := api.New(cfg,
			api.WithLinuxUpgradeController(upgradeController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/linux/upgrade/infra/requires-upgrade", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)

		// Parse the response body
		var response types.RequiresInfraUpgradeResponse
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.True(t, response.RequiresUpgrade)
	})

	t.Run("Requires upgrade returns false", func(t *testing.T) {
		// Create fake k8s clients with same version
		fakeKcli := clientfake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(integration.NewTestControllerNode(hostname), existingInstallation).
			WithStatusSubresource(&ecv1beta1.Installation{}, &apiextensionsv1.CustomResourceDefinition{}).
			WithInterceptorFuncs(integration.NewTestInterceptorFuncs()).
			Build()
		fakeMcli := metadatafake.NewSimpleMetadataClient(metascheme)

		helmMock := &helm.MockClient{}

		// Create a runtime config
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		// Create real infra manager with same version
		infraManager := linuxinfra.NewInfraManager(
			linuxinfra.WithKubeClient(fakeKcli),
			linuxinfra.WithMetadataClient(fakeMcli),
			linuxinfra.WithHelmClient(helmMock),
			linuxinfra.WithLicense(assets.LicenseData),
			linuxinfra.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{
						Version: "v1.0.0", // Same version means no upgrade required
					},
				},
				ChannelRelease: &release.ChannelRelease{},
				AppConfig:      &appConfig,
			}),
		)

		// Create an upgrade controller
		upgradeController, err := linuxupgrade.NewUpgradeController(
			linuxupgrade.WithRuntimeConfig(rc),
			linuxupgrade.WithInfraManager(infraManager),
			linuxupgrade.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{
						Version: "v1.0.0",
					},
				},
				ChannelRelease: &release.ChannelRelease{},
				AppConfig:      &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with Mode and Target set for upgrade routes
		password := "password"
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
		require.NoError(t, err)
		cfg := types.APIConfig{
			Password:     password,
			PasswordHash: passwordHash,
			ReleaseData:  integration.DefaultReleaseData(),
			Mode:         types.ModeUpgrade,
			Target:       types.TargetLinux,
		}
		apiInstance, err := api.New(cfg,
			api.WithLinuxUpgradeController(upgradeController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodGet, "/linux/upgrade/infra/requires-upgrade", nil)
		req.Header.Set("Authorization", "Bearer TOKEN")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)

		// Parse the response body
		var response types.RequiresInfraUpgradeResponse
		err = json.NewDecoder(rec.Body).Decode(&response)
		require.NoError(t, err)
		assert.False(t, response.RequiresUpgrade)
	})
}

// Test the linux upgrade infra endpoint
func TestLinuxPostUpgradeInfra(t *testing.T) {
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
							Default: multitype.FromString("default"),
							Value:   multitype.FromString("value"),
						},
					},
				},
			},
		},
	}

	hostname, err := nodeutil.GetHostname("")
	require.NoError(t, err)

	// Create an existing installation
	existingInstallation := &ecv1beta1.Installation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ecv1beta1.GroupVersion.String(),
			Kind:       "Installation",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "20250101000000",
		},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "v1.0.0",
			},
		},
		Status: ecv1beta1.InstallationStatus{
			State: ecv1beta1.InstallationStateInstalled,
		},
	}

	t.Run("Success", func(t *testing.T) {
		// Create fake k8s clients
		fakeKcli := clientfake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(integration.NewTestControllerNode(hostname), existingInstallation).
			WithStatusSubresource(&ecv1beta1.Installation{}, &apiextensionsv1.CustomResourceDefinition{}).
			WithInterceptorFuncs(integration.NewTestInterceptorFuncs()).
			Build()
		fakeMcli := metadatafake.NewSimpleMetadataClient(metascheme)

		helmMock := &helm.MockClient{}
		upgraderMock := &upgrade.MockInfraUpgrader{}

		// Create a runtime config
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		// Setup mock expectations for successful upgrade
		upgraderMock.On("CreateInstallation", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			// CreateInstallation should actually create the Installation object in the fake k8s client
			in := args.Get(1).(*ecv1beta1.Installation)
			err := fakeKcli.Create(t.Context(), in)
			require.NoError(t, err)
		}).Return(nil)
		upgraderMock.On("CopyVersionMetadataToCluster", mock.Anything, mock.Anything).Return(nil)
		upgraderMock.On("DistributeArtifacts", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		upgraderMock.On("UpgradeK0s", mock.Anything, mock.Anything).Return(nil)
		upgraderMock.On("UpdateClusterConfig", mock.Anything, mock.Anything).Return(nil)
		upgraderMock.On("UpgradeAddons", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		upgraderMock.On("UpgradeExtensions", mock.Anything, mock.Anything).Return(nil)
		upgraderMock.On("CreateHostSupportBundle", mock.Anything).Return(nil)

		// Create real infra manager with mocked components
		infraManager := linuxinfra.NewInfraManager(
			linuxinfra.WithKubeClient(fakeKcli),
			linuxinfra.WithMetadataClient(fakeMcli),
			linuxinfra.WithHelmClient(helmMock),
			linuxinfra.WithInfraUpgrader(upgraderMock),
			linuxinfra.WithLicense(assets.LicenseData),
			linuxinfra.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{
						Version: "v1.1.0",
					},
				},
				ChannelRelease: &release.ChannelRelease{
					AppSlug:      "test-app",
					ChannelID:    "test-channel",
					VersionLabel: "1.1.0",
				},
				AppConfig: &appConfig,
			}),
		)

		// Create an upgrade controller with ApplicationConfigured state
		upgradeController, err := linuxupgrade.NewUpgradeController(
			linuxupgrade.WithRuntimeConfig(rc),
			linuxupgrade.WithInfraManager(infraManager),
			linuxupgrade.WithStateMachine(linuxupgrade.NewStateMachine(
				linuxupgrade.WithCurrentState(states.StateApplicationConfigured),
				linuxupgrade.WithRequiresInfraUpgrade(true),
			)),
			linuxupgrade.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{
						Version: "v1.1.0",
					},
				},
				ChannelRelease: &release.ChannelRelease{},
				AppConfig:      &appConfig,
			}),
			linuxupgrade.WithLicense(assets.LicenseData),
		)
		require.NoError(t, err)

		// Create the API with Mode and Target set for upgrade routes
		password := "password"
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
		require.NoError(t, err)
		cfg := types.APIConfig{
			Password:     password,
			PasswordHash: passwordHash,
			ReleaseData:  integration.DefaultReleaseData(),
			Mode:         types.ModeUpgrade,
			Target:       types.TargetLinux,
		}
		apiInstance, err := api.New(cfg,
			api.WithLinuxUpgradeController(upgradeController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/linux/upgrade/infra/upgrade", bytes.NewReader([]byte("{}")))
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

		// Verify that the status is not pending. We cannot check for an end state here because process is async
		// so the state might have moved from running to a final state before we get the response.
		assert.NotEqual(t, types.StatePending, infra.Status.State)

		// Helper function to get infra status
		getInfraStatus := func() types.Infra {
			// Create a request to get infra status
			req := httptest.NewRequest(http.MethodGet, "/linux/upgrade/infra/status", nil)
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
				t.Fatalf("Infrastructure upgrade failed: %s", infra.Status.Description)
			}

			return infra.Status.State == types.StateSucceeded
		}, 15*time.Second, 500*time.Millisecond, "Infrastructure upgrade did not succeed in time")

		// Verify that the mock expectations were met
		upgraderMock.AssertExpectations(t)
	})

	t.Run("Invalid state transition", func(t *testing.T) {
		// Create fake k8s clients
		fakeKcli := clientfake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(integration.NewTestControllerNode(hostname), existingInstallation).
			WithStatusSubresource(&ecv1beta1.Installation{}, &apiextensionsv1.CustomResourceDefinition{}).
			WithInterceptorFuncs(integration.NewTestInterceptorFuncs()).
			Build()
		fakeMcli := metadatafake.NewSimpleMetadataClient(metascheme)

		helmMock := &helm.MockClient{}

		// Create a runtime config
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		// Create real infra manager
		infraManager := linuxinfra.NewInfraManager(
			linuxinfra.WithKubeClient(fakeKcli),
			linuxinfra.WithMetadataClient(fakeMcli),
			linuxinfra.WithHelmClient(helmMock),
			linuxinfra.WithLicense(assets.LicenseData),
			linuxinfra.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{
						Version: "v1.1.0",
					},
				},
				ChannelRelease: &release.ChannelRelease{},
				AppConfig:      &appConfig,
			}),
		)

		// Create an upgrade controller with wrong state (already upgrading)
		upgradeController, err := linuxupgrade.NewUpgradeController(
			linuxupgrade.WithRuntimeConfig(rc),
			linuxupgrade.WithInfraManager(infraManager),
			linuxupgrade.WithStateMachine(linuxupgrade.NewStateMachine(
				linuxupgrade.WithCurrentState(states.StateInfrastructureUpgrading),
				linuxupgrade.WithRequiresInfraUpgrade(true),
			)),
			linuxupgrade.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{
						Version: "v1.1.0",
					},
				},
				ChannelRelease: &release.ChannelRelease{},
				AppConfig:      &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with Mode and Target set for upgrade routes
		password := "password"
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
		require.NoError(t, err)
		cfg := types.APIConfig{
			Password:     password,
			PasswordHash: passwordHash,
			ReleaseData:  integration.DefaultReleaseData(),
			Mode:         types.ModeUpgrade,
			Target:       types.TargetLinux,
		}
		apiInstance, err := api.New(cfg,
			api.WithLinuxUpgradeController(upgradeController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/linux/upgrade/infra/upgrade", bytes.NewReader([]byte("{}")))
		req.Header.Set("Authorization", "Bearer TOKEN")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Contains(t, rec.Body.String(), "invalid transition")
	})

	t.Run("Upgrade error", func(t *testing.T) {
		// Create fake k8s clients
		fakeKcli := clientfake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(integration.NewTestControllerNode(hostname), existingInstallation).
			WithStatusSubresource(&ecv1beta1.Installation{}, &apiextensionsv1.CustomResourceDefinition{}).
			WithInterceptorFuncs(integration.NewTestInterceptorFuncs()).
			Build()
		fakeMcli := metadatafake.NewSimpleMetadataClient(metascheme)

		helmMock := &helm.MockClient{}
		upgraderMock := &upgrade.MockInfraUpgrader{}

		// Create a runtime config
		rc := runtimeconfig.New(nil)
		rc.SetDataDir(t.TempDir())

		// Setup mock expectations for upgrade failure
		// CreateInstallation should actually create the Installation object in the fake k8s client
		upgraderMock.On("CreateInstallation", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			in := args.Get(1).(*ecv1beta1.Installation)
			err := fakeKcli.Create(t.Context(), in)
			require.NoError(t, err)
		}).Return(nil)
		upgraderMock.On("CopyVersionMetadataToCluster", mock.Anything, mock.Anything).Return(nil)
		upgraderMock.On("DistributeArtifacts", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		upgraderMock.On("UpgradeK0s", mock.Anything, mock.Anything).Return(errors.New("upgrade failed"))

		// Create real infra manager with mocked components
		infraManager := linuxinfra.NewInfraManager(
			linuxinfra.WithKubeClient(fakeKcli),
			linuxinfra.WithMetadataClient(fakeMcli),
			linuxinfra.WithHelmClient(helmMock),
			linuxinfra.WithInfraUpgrader(upgraderMock),
			linuxinfra.WithLicense(assets.LicenseData),
			linuxinfra.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{
						Version: "v1.1.0",
					},
				},
				ChannelRelease: &release.ChannelRelease{
					AppSlug:      "test-app",
					ChannelID:    "test-channel",
					VersionLabel: "1.1.0",
				},
				AppConfig: &appConfig,
			}),
		)

		// Create an upgrade controller
		upgradeController, err := linuxupgrade.NewUpgradeController(
			linuxupgrade.WithRuntimeConfig(rc),
			linuxupgrade.WithInfraManager(infraManager),
			linuxupgrade.WithStateMachine(linuxupgrade.NewStateMachine(
				linuxupgrade.WithCurrentState(states.StateApplicationConfigured),
				linuxupgrade.WithRequiresInfraUpgrade(true),
			)),
			linuxupgrade.WithReleaseData(&release.ReleaseData{
				EmbeddedClusterConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{
						Version: "v1.1.0",
					},
				},
				ChannelRelease: &release.ChannelRelease{},
				AppConfig:      &appConfig,
			}),
		)
		require.NoError(t, err)

		// Create the API with Mode and Target set for upgrade routes
		password := "password"
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
		require.NoError(t, err)
		cfg := types.APIConfig{
			Password:     password,
			PasswordHash: passwordHash,
			ReleaseData:  integration.DefaultReleaseData(),
			Mode:         types.ModeUpgrade,
			Target:       types.TargetLinux,
		}
		apiInstance, err := api.New(cfg,
			api.WithLinuxUpgradeController(upgradeController),
			api.WithAuthController(auth.NewStaticAuthController("TOKEN")),
			api.WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Create a router and register the API routes
		router := mux.NewRouter()
		apiInstance.RegisterRoutes(router)

		// Create a request
		req := httptest.NewRequest(http.MethodPost, "/linux/upgrade/infra/upgrade", bytes.NewReader([]byte("{}")))
		req.Header.Set("Authorization", "Bearer TOKEN")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Serve the request
		router.ServeHTTP(rec, req)

		// Check the response
		assert.Equal(t, http.StatusOK, rec.Code)

		// The status should eventually be set to failed due to upgrade error
		assert.Eventually(t, func() bool {
			// Create a request to get infra status
			req := httptest.NewRequest(http.MethodGet, "/linux/upgrade/infra/status", nil)
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
			return infra.Status.State == types.StateFailed && strings.Contains(infra.Status.Description, "upgrade failed")
		}, 10*time.Second, 100*time.Millisecond, "Infrastructure upgrade did not fail in time")

		// Verify that the mock expectations were met
		upgraderMock.AssertExpectations(t)
	})
}
