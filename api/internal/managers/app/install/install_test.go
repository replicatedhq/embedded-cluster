package install

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	appinstallstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	metadatafake "k8s.io/client-go/metadata/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	kyaml "sigs.k8s.io/yaml"
)

func TestAppInstallManager_Install(t *testing.T) {
	// Setup environment variable for V3
	t.Setenv("ENABLE_V3", "1")

	// Create valid helm chart archive
	mockChartArchive := createTestChartArchive(t, "test-chart", "0.1.0")

	// Create test license
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			AppSlug: "test-app",
		},
	}
	licenseBytes, err := kyaml.Marshal(license)
	require.NoError(t, err)

	// Create test release data with helm chart archives
	releaseData := &release.ReleaseData{
		HelmChartArchives: [][]byte{mockChartArchive},
		ChannelRelease: &release.ChannelRelease{
			DefaultDomains: release.Domains{
				ReplicatedAppDomain: "replicated.app",
			},
		},
	}

	// Set up release data globally so AppSlug() returns the correct value for v3
	err = release.SetReleaseDataForTests(map[string][]byte{
		"channelrelease.yaml": []byte("# channel release object\nappSlug: test-app"),
	})
	require.NoError(t, err)

	// create fake kube client with kotsadm namespace
	kotsadmNamespace := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-app",
		},
	}
	fakeKcli := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(kotsadmNamespace).Build()

	t.Run("Success", func(t *testing.T) {
		configValues := types.AppConfigValues{
			"key1": {Value: "value1"},
			"key2": {Value: "value2"},
			"key3": {Value: "value3"},
		}

		// Create InstallableHelmCharts with weights - should already be sorted at this stage
		installableCharts := []types.InstallableHelmChart{
			createTestInstallableHelmChart(t, "nginx-chart", "1.0.0", "web-server", "web", 10, map[string]any{
				"image": map[string]any{
					"repository": "nginx",
					"tag":        "latest",
				},
				"replicas": 3,
			}),
			createTestInstallableHelmChart(t, "postgres-chart", "2.0.0", "database", "data", 20, map[string]any{
				"database": map[string]any{
					"host":     "postgres.example.com",
					"password": "secret",
				},
			}),
		}

		// Create registry settings for testing image pull secret creation
		dockerConfigJSON := `{"auths":{"registry.example.com":{"auth":"dXNlcjpwYXNz"}}}`
		registrySettings := &types.RegistrySettings{
			ImagePullSecretName:  "test-pull-secret",
			ImagePullSecretValue: base64.StdEncoding.EncodeToString([]byte(dockerConfigJSON)),
		}

		// Create a temp CA bundle file for testing
		caContent := "-----BEGIN CERTIFICATE-----\ntest-ca-content\n-----END CERTIFICATE-----"
		tmpCAFile, err := os.CreateTemp("", "ca-bundle-*.crt")
		require.NoError(t, err)
		defer os.Remove(tmpCAFile.Name())
		_, err = tmpCAFile.WriteString(caContent)
		require.NoError(t, err)
		tmpCAFile.Close()

		// Create fake metadata client for CA configmap creation
		fakeMcli := metadatafake.NewSimpleMetadataClient(metadatafake.NewTestScheme())

		// Create mock helm client that validates pre-processed values
		mockHelmClient := &helm.MockClient{}

		// Chart 1 installation (nginx chart)
		nginxCall := mockHelmClient.On("Install", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			if opts.ReleaseName != "web-server" {
				return false
			}
			if opts.Namespace != "web" {
				return false
			}
			// Check if values contain expected pre-processed data for nginx chart
			if vals, ok := opts.Values["image"].(map[string]any); ok {
				return vals["repository"] == "nginx" && vals["tag"] == "latest" && opts.Values["replicas"] == 3
			}
			return false
		})).Return("Release \"web-server\" has been installed.", nil)

		// Chart 2 installation (database chart)
		databaseCall := mockHelmClient.On("Install", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			if opts.ReleaseName != "database" {
				return false
			}
			if opts.Namespace != "data" {
				return false
			}
			// Check if values contain expected pre-processed database data
			if vals, ok := opts.Values["database"].(map[string]any); ok {
				return vals["host"] == "postgres.example.com" && vals["password"] == "secret"
			}
			return false
		})).Return("Release \"database\" has been installed.", nil)

		// Verify installation order
		mock.InOrder(
			nginxCall,
			databaseCall,
		)

		// Create manager
		manager, err := NewAppInstallManager(
			WithClusterID("test-cluster"),
			WithAirgapBundle("test-airgap.tar.gz"),
			WithReleaseData(releaseData),
			WithLicense(licenseBytes),
			WithHelmClient(mockHelmClient),
			WithLogger(logger.NewDiscardLogger()),
			WithKubeClient(fakeKcli),
			WithMetadataClient(fakeMcli),
		)
		require.NoError(t, err)

		// Run installation with registry settings and host CA bundle path
		err = manager.Install(t.Context(), installableCharts, configValues, registrySettings, tmpCAFile.Name())
		require.NoError(t, err)

		mockHelmClient.AssertExpectations(t)

		// Verify image pull secret was created in the app namespace
		secret := &corev1.Secret{}
		err = fakeKcli.Get(t.Context(), client.ObjectKey{
			Namespace: "test-app",
			Name:      "test-pull-secret",
		}, secret)
		require.NoError(t, err)
		assert.Equal(t, corev1.SecretTypeDockerConfigJson, secret.Type)
		assert.Equal(t, dockerConfigJSON, string(secret.Data[".dockerconfigjson"]))

		// Verify CA configmap was created in the app namespace
		configMap := &corev1.ConfigMap{}
		err = fakeKcli.Get(t.Context(), client.ObjectKey{
			Namespace: "test-app",
			Name:      "kotsadm-private-cas",
		}, configMap)
		require.NoError(t, err)
		assert.Contains(t, configMap.Data["ca_0.crt"], "test-ca-content")
	})

	t.Run("Install updates status correctly", func(t *testing.T) {
		configValues := types.AppConfigValues{
			"key1": {Value: "value1"},
		}

		installableCharts := []types.InstallableHelmChart{
			createTestInstallableHelmChart(t, "monitoring-chart", "1.0.0", "prometheus", "monitoring", 0, map[string]any{"key": "value"}),
		}

		// Create mock helm client
		mockHelmClient := &helm.MockClient{}
		mockHelmClient.On("Install", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			return opts.ChartPath != "" && opts.ReleaseName == "prometheus" && opts.Namespace == "monitoring"
		})).Return("Release \"prometheus\" has been installed.", nil)

		// Create manager with initialized store
		store := appinstallstore.NewMemoryStore(appinstallstore.WithAppInstall(types.AppInstall{
			Status: types.Status{State: types.StatePending},
		}))
		manager, err := NewAppInstallManager(
			WithClusterID("test-cluster"),
			WithReleaseData(releaseData),
			WithLicense(licenseBytes),
			WithHelmClient(mockHelmClient),
			WithLogger(logger.NewDiscardLogger()),
			WithAppInstallStore(store),
			WithKubeClient(fakeKcli),
		)
		require.NoError(t, err)

		// Verify initial status
		appInstall, err := manager.GetStatus()
		require.NoError(t, err)
		assert.Equal(t, types.StatePending, appInstall.Status.State)

		// Run installation
		err = manager.Install(t.Context(), installableCharts, configValues, nil, "")
		require.NoError(t, err)

		// Verify components status
		appInstall, err = manager.GetStatus()
		require.NoError(t, err)
		assert.NotEmpty(t, appInstall.Components)

		mockHelmClient.AssertExpectations(t)
	})

	t.Run("Install handles errors correctly", func(t *testing.T) {
		configValues := types.AppConfigValues{
			"key1": {Value: "value1"},
		}

		installableCharts := []types.InstallableHelmChart{
			createTestInstallableHelmChart(t, "logging-chart", "1.0.0", "fluentd", "logging", 0, map[string]any{"key": "value"}),
		}

		// Create mock helm client that fails
		mockHelmClient := &helm.MockClient{}
		mockHelmClient.On("Install", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			return opts.ChartPath != "" && opts.ReleaseName == "fluentd" && opts.Namespace == "logging"
		})).Return("", assert.AnError)

		// Create manager with initialized store
		store := appinstallstore.NewMemoryStore(appinstallstore.WithAppInstall(types.AppInstall{
			Status: types.Status{State: types.StatePending},
		}))
		manager, err := NewAppInstallManager(
			WithClusterID("test-cluster"),
			WithReleaseData(releaseData),
			WithLicense(licenseBytes),
			WithHelmClient(mockHelmClient),
			WithLogger(logger.NewDiscardLogger()),
			WithAppInstallStore(store),
			WithKubeClient(fakeKcli),
		)
		require.NoError(t, err)

		// Run installation (should fail)
		err = manager.Install(t.Context(), installableCharts, configValues, nil, "")
		assert.Error(t, err)

		mockHelmClient.AssertExpectations(t)
	})

	t.Run("GetStatus returns current app install state", func(t *testing.T) {
		// Create test store with known status
		store := appinstallstore.NewMemoryStore(appinstallstore.WithAppInstall(types.AppInstall{
			Status: types.Status{
				State:       types.StateRunning,
				Description: "Installing application",
				LastUpdated: time.Now(),
			},
			Logs: "Installation started\n",
		}))

		// Create manager with test store
		manager, err := NewAppInstallManager(
			WithLogger(logger.NewDiscardLogger()),
			WithAppInstallStore(store),
			WithHelmClient(&helm.MockClient{}),
			WithKubeClient(fakeKcli),
		)
		require.NoError(t, err)

		// Test GetStatus
		appInstall, err := manager.GetStatus()
		require.NoError(t, err)
		assert.Equal(t, types.StateRunning, appInstall.Status.State)
		assert.Equal(t, "Installing application", appInstall.Status.Description)
		assert.Equal(t, "Installation started\n", appInstall.Logs)
	})
}

// createTarGzArchive creates a tar.gz archive with the given files
func createTarGzArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for filename, content := range files {
		header := &tar.Header{
			Name: filename,
			Mode: 0600,
			Size: int64(len(content)),
		}
		require.NoError(t, tw.WriteHeader(header))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	return buf.Bytes()
}

func createTestChartArchive(t *testing.T, name, version string) []byte {
	chartYaml := fmt.Sprintf(`apiVersion: v2
name: %s
version: %s
description: A test Helm chart
type: application
`, name, version)

	return createTarGzArchive(t, map[string]string{
		fmt.Sprintf("%s/Chart.yaml", name): chartYaml,
	})
}

// Helper functions to create test data that can be reused across test cases
func createTestHelmChartCR(name, releaseName, namespace string, weight int64) *kotsv1beta2.HelmChart {
	return &kotsv1beta2.HelmChart{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kots.io/v1beta2",
			Kind:       "HelmChart",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: kotsv1beta2.HelmChartSpec{
			Chart: kotsv1beta2.ChartIdentifier{
				Name:         name,
				ChartVersion: "1.0.0",
			},
			ReleaseName: releaseName,
			Namespace:   namespace,
			Weight:      weight,
		},
	}
}

func createTestInstallableHelmChart(t *testing.T, chartName, chartVersion, releaseName, namespace string, weight int64, values map[string]any) types.InstallableHelmChart {
	return types.InstallableHelmChart{
		Archive: createTestChartArchive(t, chartName, chartVersion),
		Values:  values,
		CR:      createTestHelmChartCR(chartName, releaseName, namespace, weight),
	}
}

// TestComponentStatusTracking tests that components are properly initialized and tracked
func TestComponentStatusTracking(t *testing.T) {
	// Create test license
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			AppSlug: "test-app",
		},
	}
	licenseBytes, err := kyaml.Marshal(license)
	require.NoError(t, err)

	// create fake kube client with kotsadm namespace
	kotsadmNamespace := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kotsadm",
		},
	}
	fakeKcli := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(kotsadmNamespace).Build()

	t.Run("Components are registered and status is tracked", func(t *testing.T) {
		configValues := types.AppConfigValues{
			"key1": {Value: "value1"},
			"key2": {Value: "value2"},
			"key3": {Value: "value3"},
		}

		// Create test charts with different weights
		installableCharts := []types.InstallableHelmChart{
			createTestInstallableHelmChart(t, "database-chart", "1.0.0", "postgres", "data", 10, map[string]any{"key": "value1"}),
			createTestInstallableHelmChart(t, "web-chart", "2.0.0", "nginx", "web", 20, map[string]any{"key": "value2"}),
		}

		// Create mock helm client
		mockHelmClient := &helm.MockClient{}

		// Database chart installation (should be first due to lower weight)
		mockHelmClient.On("Install", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			return opts.ReleaseName == "postgres" && opts.Namespace == "data"
		})).Return("Release \"postgres\" has been installed.", nil).Once()

		// Web chart installation (should be second due to higher weight)
		mockHelmClient.On("Install", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			return opts.ReleaseName == "nginx" && opts.Namespace == "web"
		})).Return("Release \"nginx\" has been installed.", nil).Once()

		// Create manager with in-memory store
		appInstallStore := appinstallstore.NewMemoryStore(appinstallstore.WithAppInstall(types.AppInstall{
			Status: types.Status{State: types.StatePending},
		}))
		manager, err := NewAppInstallManager(
			WithAppInstallStore(appInstallStore),
			WithReleaseData(&release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					VersionLabel: "v1.0.0",
				},
			}),
			WithLicense(licenseBytes),
			WithClusterID("test-cluster"),
			WithHelmClient(mockHelmClient),
			WithKubeClient(fakeKcli),
		)
		require.NoError(t, err)

		// Install the charts
		err = manager.Install(t.Context(), installableCharts, configValues, nil, "")
		require.NoError(t, err)

		// Verify that components were registered and have correct status
		appInstall, err := manager.GetStatus()
		require.NoError(t, err)

		// Should have 2 components
		assert.Len(t, appInstall.Components, 2, "Should have 2 components")

		// Components should be sorted by weight (database first with weight 10, web second with weight 20)
		assert.Equal(t, "database-chart", appInstall.Components[0].Name)
		assert.Equal(t, types.StateSucceeded, appInstall.Components[0].Status.State)

		assert.Equal(t, "web-chart", appInstall.Components[1].Name)
		assert.Equal(t, types.StateSucceeded, appInstall.Components[1].Status.State)

		mockHelmClient.AssertExpectations(t)
	})

	t.Run("Component failure is tracked correctly", func(t *testing.T) {
		configValues := types.AppConfigValues{
			"key1": {Value: "value1"},
		}

		// Create test chart
		installableCharts := []types.InstallableHelmChart{
			createTestInstallableHelmChart(t, "failing-chart", "1.0.0", "failing-app", "default", 0, map[string]any{"key": "value"}),
		}

		// Create mock helm client that fails
		mockHelmClient := &helm.MockClient{}
		mockHelmClient.On("Install", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			return opts.ReleaseName == "failing-app"
		})).Return("", errors.New("helm install failed"))

		// Create manager with in-memory store
		appInstallStore := appinstallstore.NewMemoryStore(appinstallstore.WithAppInstall(types.AppInstall{
			Status: types.Status{State: types.StatePending},
		}))
		manager, err := NewAppInstallManager(
			WithAppInstallStore(appInstallStore),
			WithReleaseData(&release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					VersionLabel: "v1.0.0",
				},
			}),
			WithLicense(licenseBytes),
			WithClusterID("test-cluster"),
			WithHelmClient(mockHelmClient),
			WithKubeClient(fakeKcli),
		)
		require.NoError(t, err)

		// Install the charts (should fail)
		err = manager.Install(t.Context(), installableCharts, configValues, nil, "")
		require.Error(t, err)

		// Verify that component failure is tracked
		appInstall, err := manager.GetStatus()
		require.NoError(t, err)

		// Should have 1 component
		assert.Len(t, appInstall.Components, 1, "Should have 1 component")

		// Component should be marked as failed
		failedComponent := appInstall.Components[0]
		assert.Equal(t, "failing-chart", failedComponent.Name)
		assert.Equal(t, types.StateFailed, failedComponent.Status.State)
		assert.Contains(t, failedComponent.Status.Description, "helm install failed")

		mockHelmClient.AssertExpectations(t)
	})
}

func TestAppInstallManager_Install_ConfigValuesSecret(t *testing.T) {
	// Set up environment and release data for all tests
	t.Setenv("ENABLE_V3", "1")
	err := release.SetReleaseDataForTests(map[string][]byte{
		"channelrelease.yaml": []byte("# channel release object\nappSlug: test-app"),
	})
	require.NoError(t, err)

	tests := []struct {
		name                  string
		releaseData           *release.ReleaseData
		configValues          types.AppConfigValues
		setupClient           func(t *testing.T) client.Client
		expectError           bool
		expectedErrorContains string
		validateSecret        func(t *testing.T, kcli client.Client)
	}{
		{
			name: "first install creates secret with multiple config values",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					VersionLabel: "v1.0.0",
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.app",
					},
				},
			},
			configValues: types.AppConfigValues{
				"key1": {Value: "value1"},
				"key2": {Value: "value2"},
				"key3": {Value: "value3"},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))
				return fake.NewClientBuilder().WithScheme(sch).Build()
			},
			expectError: false,
			validateSecret: func(t *testing.T, kcli client.Client) {
				// Get and verify secret
				secret := &corev1.Secret{}
				err := kcli.Get(t.Context(), client.ObjectKey{
					Name:      "test-app-config-values",
					Namespace: "test-app",
				}, secret)
				require.NoError(t, err)

				// Verify labels
				assert.Equal(t, "test-app", secret.Labels["app.kubernetes.io/name"])
				assert.Equal(t, "v1.0.0", secret.Labels["app.kubernetes.io/version"])
				assert.Equal(t, "config", secret.Labels["app.kubernetes.io/component"])
				assert.Equal(t, "embedded-cluster", secret.Labels["app.kubernetes.io/part-of"])
				assert.Equal(t, "embedded-cluster-installer", secret.Labels["app.kubernetes.io/managed-by"])

				// Verify type
				assert.Equal(t, corev1.SecretTypeOpaque, secret.Type)

				// Verify data
				data, ok := secret.Data["config-values.yaml"]
				require.True(t, ok)

				// Unmarshal and verify values
				var cv types.AppConfigValues
				err = kyaml.Unmarshal(data, &cv)
				require.NoError(t, err)
				assert.Equal(t, "value1", cv["key1"].Value)
				assert.Equal(t, "value2", cv["key2"].Value)
				assert.Equal(t, "value3", cv["key3"].Value)
			},
		},
		{
			name: "existing secret is fetched and updated",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					VersionLabel: "v1.0.0",
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.app",
					},
				},
			},
			configValues: types.AppConfigValues{
				"newkey": {Value: "newvalue"},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))

				// Create existing secret with old version
				existingSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app-config-values",
						Namespace: "test-app",
						Labels: map[string]string{
							"app.kubernetes.io/version": "v0.9.0",
						},
					},
					Data: map[string][]byte{
						"config-values.yaml": []byte("old: data"),
					},
				}

				return fake.NewClientBuilder().
					WithScheme(sch).
					WithObjects(existingSecret).
					Build()
			},
			expectError: false,
			validateSecret: func(t *testing.T, kcli client.Client) {
				// Get and verify secret was recreated
				secret := &corev1.Secret{}
				err := kcli.Get(t.Context(), client.ObjectKey{
					Name:      "test-app-config-values",
					Namespace: "test-app",
				}, secret)
				require.NoError(t, err)

				// Verify updated version label
				assert.Equal(t, "v1.0.0", secret.Labels["app.kubernetes.io/version"])

				// Verify new data
				data, ok := secret.Data["config-values.yaml"]
				require.True(t, ok)

				var cv types.AppConfigValues
				err = kyaml.Unmarshal(data, &cv)
				require.NoError(t, err)
				assert.Equal(t, "newvalue", cv["newkey"].Value)
			},
		},
		{
			name:        "fails when release data is missing",
			releaseData: nil,
			configValues: types.AppConfigValues{
				"key1": {Value: "value1"},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))
				return fake.NewClientBuilder().WithScheme(sch).Build()
			},
			expectError:           true,
			expectedErrorContains: "release data is required",
		},
		{
			name: "fails when channel release is missing",
			releaseData: &release.ReleaseData{
				ChannelRelease: nil,
			},
			configValues: types.AppConfigValues{
				"key1": {Value: "value1"},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))
				return fake.NewClientBuilder().WithScheme(sch).Build()
			},
			expectError:           true,
			expectedErrorContains: "release data is required",
		},
		{
			name: "fails when get returns error",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					VersionLabel: "v1.0.0",
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.app",
					},
				},
			},
			configValues: types.AppConfigValues{
				"key1": {Value: "value1"},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))

				// Create existing secret
				existingSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app-config-values",
						Namespace: "test-app",
					},
				}

				return fake.NewClientBuilder().
					WithScheme(sch).
					WithObjects(existingSecret).
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
							if key.Name == "test-app-config-values" {
								return fmt.Errorf("simulated get error")
							}
							return c.Get(ctx, key, obj, opts...)
						},
					}).
					Build()
			},
			expectError:           true,
			expectedErrorContains: "get existing config values secret",
		},
		{
			name: "fails when update returns error",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					VersionLabel: "v1.0.0",
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.app",
					},
				},
			},
			configValues: types.AppConfigValues{
				"key1": {Value: "value1"},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))

				// Create existing secret
				existingSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app-config-values",
						Namespace: "test-app",
					},
				}

				return fake.NewClientBuilder().
					WithScheme(sch).
					WithObjects(existingSecret).
					WithInterceptorFuncs(interceptor.Funcs{
						Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
							if secret, ok := obj.(*corev1.Secret); ok && secret.Name == "test-app-config-values" {
								return fmt.Errorf("simulated update error")
							}
							return c.Update(ctx, obj, opts...)
						},
					}).
					Build()
			},
			expectError:           true,
			expectedErrorContains: "update config values secret",
		},
		{
			name: "fails when create returns non-AlreadyExists error",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					VersionLabel: "v1.0.0",
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.app",
					},
				},
			},
			configValues: types.AppConfigValues{
				"key1": {Value: "value1"},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))

				return fake.NewClientBuilder().
					WithScheme(sch).
					WithInterceptorFuncs(interceptor.Funcs{
						Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
							if _, ok := obj.(*corev1.Secret); ok {
								return fmt.Errorf("simulated create error")
							}
							return c.Create(ctx, obj, opts...)
						},
					}).
					Build()
			},
			expectError:           true,
			expectedErrorContains: "create config values secret",
		},
		{
			name: "handles empty config values",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					VersionLabel: "v1.0.0",
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.app",
					},
				},
			},
			configValues: types.AppConfigValues{
				"key1": {Value: "value1"},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))
				return fake.NewClientBuilder().WithScheme(sch).Build()
			},
			expectError: false,
			validateSecret: func(t *testing.T, kcli client.Client) {
				// Get and verify secret was created even with empty values
				secret := &corev1.Secret{}
				err := kcli.Get(t.Context(), client.ObjectKey{
					Name:      "test-app-config-values",
					Namespace: "test-app",
				}, secret)
				require.NoError(t, err)

				// Verify data exists
				data, ok := secret.Data["config-values.yaml"]
				require.True(t, ok)
				require.NotEmpty(t, data)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			license := &kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{AppSlug: "test-app"},
			}
			licenseBytes, err := kyaml.Marshal(license)
			require.NoError(t, err)

			kcli := tt.setupClient(t)

			// Create mock helm client
			mockHelmClient := &helm.MockClient{}

			manager, err := NewAppInstallManager(
				WithLicense(licenseBytes),
				WithClusterID("test-cluster"),
				WithAirgapBundle("test-airgap.tar.gz"),
				WithReleaseData(tt.releaseData),
				WithLogger(logger.NewDiscardLogger()),
				WithKubeClient(kcli),
				WithHelmClient(mockHelmClient),
			)
			require.NoError(t, err)

			// Execute
			err = manager.Install(t.Context(), nil, tt.configValues, nil, "")

			// Verify
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorContains)
			} else {
				require.NoError(t, err)
				if tt.validateSecret != nil {
					tt.validateSecret(t, kcli)
				}
			}
		})
	}
}
