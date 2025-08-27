package install

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	appinstallstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/replicatedhq/embedded-cluster/api/types"
	kotscli "github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	helmrelease "helm.sh/helm/v3/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kyaml "sigs.k8s.io/yaml"
)

func TestAppInstallManager_Install(t *testing.T) {
	// Create test license
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			AppSlug: "test-app",
		},
	}
	licenseBytes, err := kyaml.Marshal(license)
	require.NoError(t, err)

	// Create valid helm chart archive
	mockChartArchive := createTestChartArchive(t, "test-chart", "0.1.0")

	// Create test release data with helm chart archives
	releaseData := &release.ReleaseData{
		HelmChartArchives: [][]byte{mockChartArchive},
		ChannelRelease: &release.ChannelRelease{
			DefaultDomains: release.Domains{
				ReplicatedAppDomain: "replicated.app",
			},
		},
	}

	t.Run("Success", func(t *testing.T) {
		kotsConfigValues := kotsv1beta1.ConfigValues{
			Spec: kotsv1beta1.ConfigValuesSpec{
				Values: map[string]kotsv1beta1.ConfigValue{
					"key1": {
						Value: "value1",
					},
					"key2": {
						Value: "value2",
					},
				},
			},
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
		})).Return(&helmrelease.Release{Name: "web-server"}, nil)

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
		})).Return(&helmrelease.Release{Name: "database"}, nil)

		// Verify installation order
		mock.InOrder(
			nginxCall,
			databaseCall,
		)

		// Create mock installer with detailed verification of config values
		mockInstaller := &MockKotsCLIInstaller{}
		mockInstaller.On("Install", mock.MatchedBy(func(opts kotscli.InstallOptions) bool {
			// Verify basic install options
			if opts.AppSlug != "test-app" {
				t.Logf("AppSlug mismatch: expected 'test-app', got '%s'", opts.AppSlug)
				return false
			}
			if opts.License == nil {
				t.Logf("License is nil")
				return false
			}
			if opts.Namespace != "kotsadm" {
				t.Logf("Namespace mismatch: expected 'kotsadm', got '%s'", opts.Namespace)
				return false
			}
			if opts.ClusterID != "test-cluster" {
				t.Logf("ClusterID mismatch: expected 'test-cluster', got '%s'", opts.ClusterID)
				return false
			}
			if opts.AirgapBundle != "test-airgap.tar.gz" {
				t.Logf("AirgapBundle mismatch: expected 'test-airgap.tar.gz', got '%s'", opts.AirgapBundle)
				return false
			}
			if opts.ReplicatedAppEndpoint == "" {
				t.Logf("ReplicatedAppEndpoint is empty")
				return false
			}
			if opts.ConfigValuesFile == "" {
				t.Logf("ConfigValuesFile is empty")
				return false
			}

			// Verify config values file content
			b, err := os.ReadFile(opts.ConfigValuesFile)
			if err != nil {
				t.Logf("Failed to read config values file: %v", err)
				return false
			}
			var cv kotsv1beta1.ConfigValues
			if err := kyaml.Unmarshal(b, &cv); err != nil {
				t.Logf("Failed to unmarshal config values: %v", err)
				return false
			}
			if cv.Spec.Values["key1"].Value != "value1" {
				t.Logf("Config value key1 mismatch: expected 'value1', got '%s'", cv.Spec.Values["key1"].Value)
				return false
			}
			if cv.Spec.Values["key2"].Value != "value2" {
				t.Logf("Config value key2 mismatch: expected 'value2', got '%s'", cv.Spec.Values["key2"].Value)
				return false
			}
			return true
		})).Return(nil)

		// Create manager
		manager, err := NewAppInstallManager(
			WithLicense(licenseBytes),
			WithClusterID("test-cluster"),
			WithAirgapBundle("test-airgap.tar.gz"),
			WithReleaseData(releaseData),
			WithK8sVersion("v1.33.0"),
			WithKotsCLI(mockInstaller),
			WithHelmClient(mockHelmClient),
			WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Run installation with InstallableHelmCharts and config values
		err = manager.Install(context.Background(), installableCharts, kotsConfigValues)
		require.NoError(t, err)

		mockInstaller.AssertExpectations(t)
		mockHelmClient.AssertExpectations(t)
	})

	t.Run("Install updates status correctly", func(t *testing.T) {
		installableCharts := []types.InstallableHelmChart{
			createTestInstallableHelmChart(t, "monitoring-chart", "1.0.0", "prometheus", "monitoring", 0, map[string]any{"key": "value"}),
		}

		// Create mock helm client
		mockHelmClient := &helm.MockClient{}
		mockHelmClient.On("Install", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			return opts.ChartPath != "" && opts.ReleaseName == "prometheus" && opts.Namespace == "monitoring"
		})).Return(&helmrelease.Release{Name: "prometheus"}, nil)

		// Create mock installer that succeeds
		mockInstaller := &MockKotsCLIInstaller{}
		mockInstaller.On("Install", mock.Anything).Return(nil)

		// Create manager with initialized store
		store := appinstallstore.NewMemoryStore(appinstallstore.WithAppInstall(types.AppInstall{
			Status: types.Status{State: types.StatePending},
		}))
		manager, err := NewAppInstallManager(
			WithLicense(licenseBytes),
			WithClusterID("test-cluster"),
			WithReleaseData(releaseData),
			WithK8sVersion("v1.33.0"),
			WithKotsCLI(mockInstaller),
			WithHelmClient(mockHelmClient),
			WithLogger(logger.NewDiscardLogger()),
			WithAppInstallStore(store),
		)
		require.NoError(t, err)

		// Verify initial status
		appInstall, err := manager.GetStatus()
		require.NoError(t, err)
		assert.Equal(t, types.StatePending, appInstall.Status.State)

		// Run installation
		err = manager.Install(context.Background(), installableCharts, kotsv1beta1.ConfigValues{})
		require.NoError(t, err)

		// Verify final status
		appInstall, err = manager.GetStatus()
		require.NoError(t, err)
		assert.Equal(t, types.StateSucceeded, appInstall.Status.State)
		assert.Equal(t, "Installation complete", appInstall.Status.Description)

		mockInstaller.AssertExpectations(t)
		mockHelmClient.AssertExpectations(t)
	})

	t.Run("Install handles errors correctly", func(t *testing.T) {
		installableCharts := []types.InstallableHelmChart{
			createTestInstallableHelmChart(t, "logging-chart", "1.0.0", "fluentd", "logging", 0, map[string]any{"key": "value"}),
		}

		// Create mock helm client that fails
		mockHelmClient := &helm.MockClient{}
		mockHelmClient.On("Install", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			return opts.ChartPath != "" && opts.ReleaseName == "fluentd" && opts.Namespace == "logging"
		})).Return((*helmrelease.Release)(nil), assert.AnError)

		// Create manager with initialized store (no need for KOTS installer mock since Helm fails first)
		store := appinstallstore.NewMemoryStore(appinstallstore.WithAppInstall(types.AppInstall{
			Status: types.Status{State: types.StatePending},
		}))
		manager, err := NewAppInstallManager(
			WithLicense(licenseBytes),
			WithClusterID("test-cluster"),
			WithReleaseData(releaseData),
			WithK8sVersion("v1.33.0"),
			WithHelmClient(mockHelmClient),
			WithLogger(logger.NewDiscardLogger()),
			WithAppInstallStore(store),
		)
		require.NoError(t, err)

		// Run installation (should fail)
		err = manager.Install(context.Background(), installableCharts, kotsv1beta1.ConfigValues{})
		assert.Error(t, err)

		// Verify final status
		appInstall, err := manager.GetStatus()
		require.NoError(t, err)
		assert.Equal(t, types.StateFailed, appInstall.Status.State)
		assert.Contains(t, appInstall.Status.Description, "install helm charts")

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
			WithK8sVersion("v1.33.0"),
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

func TestAppInstallManager_createConfigValuesFile(t *testing.T) {
	manager := &appInstallManager{}

	kotsConfigValues := kotsv1beta1.ConfigValues{
		Spec: kotsv1beta1.ConfigValuesSpec{
			Values: map[string]kotsv1beta1.ConfigValue{
				"testKey": {
					Value: "testValue",
				},
			},
		},
	}

	filename, err := manager.createConfigValuesFile(kotsConfigValues)
	assert.NoError(t, err)
	assert.NotEmpty(t, filename)

	// Verify file exists and contains correct content
	data, err := os.ReadFile(filename)
	assert.NoError(t, err)

	var unmarshaled kotsv1beta1.ConfigValues
	err = kyaml.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, "testValue", unmarshaled.Spec.Values["testKey"].Value)

	// Clean up
	os.Remove(filename)
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
	t.Run("Components are registered and status is tracked", func(t *testing.T) {
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
		})).Return(&helmrelease.Release{Name: "postgres"}, nil).Once()

		// Web chart installation (should be second due to higher weight)
		mockHelmClient.On("Install", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			return opts.ReleaseName == "nginx" && opts.Namespace == "web"
		})).Return(&helmrelease.Release{Name: "nginx"}, nil).Once()

		// Create mock KOTS installer
		mockInstaller := &MockKotsCLIInstaller{}
		mockInstaller.On("Install", mock.Anything).Return(nil)

		// Create manager with in-memory store
		appInstallStore := appinstallstore.NewMemoryStore(appinstallstore.WithAppInstall(types.AppInstall{
			Status: types.Status{State: types.StatePending},
		}))
		manager, err := NewAppInstallManager(
			WithAppInstallStore(appInstallStore),
			WithReleaseData(&release.ReleaseData{}),
			WithK8sVersion("v1.33.0"),
			WithLicense([]byte(`{"spec":{"appSlug":"test-app"}}`)),
			WithClusterID("test-cluster"),
			WithKotsCLI(mockInstaller),
			WithHelmClient(mockHelmClient),
		)
		require.NoError(t, err)

		// Install the charts
		err = manager.Install(context.Background(), installableCharts, kotsv1beta1.ConfigValues{})
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

		// Overall status should be succeeded
		assert.Equal(t, types.StateSucceeded, appInstall.Status.State)
		assert.Equal(t, "Installation complete", appInstall.Status.Description)

		mockInstaller.AssertExpectations(t)
		mockHelmClient.AssertExpectations(t)
	})

	t.Run("Component failure is tracked correctly", func(t *testing.T) {
		// Create test chart
		installableCharts := []types.InstallableHelmChart{
			createTestInstallableHelmChart(t, "failing-chart", "1.0.0", "failing-app", "default", 0, map[string]any{"key": "value"}),
		}

		// Create mock helm client that fails
		mockHelmClient := &helm.MockClient{}
		mockHelmClient.On("Install", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			return opts.ReleaseName == "failing-app"
		})).Return((*helmrelease.Release)(nil), errors.New("helm install failed"))

		// Create manager with in-memory store
		appInstallStore := appinstallstore.NewMemoryStore(appinstallstore.WithAppInstall(types.AppInstall{
			Status: types.Status{State: types.StatePending},
		}))
		manager, err := NewAppInstallManager(
			WithAppInstallStore(appInstallStore),
			WithReleaseData(&release.ReleaseData{}),
			WithK8sVersion("v1.33.0"),
			WithLicense([]byte(`{"spec":{"appSlug":"test-app"}}`)),
			WithClusterID("test-cluster"),
			WithHelmClient(mockHelmClient),
		)
		require.NoError(t, err)

		// Install the charts (should fail)
		err = manager.Install(context.Background(), installableCharts, kotsv1beta1.ConfigValues{})
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

		// Overall status should be failed
		assert.Equal(t, types.StateFailed, appInstall.Status.State)

		mockHelmClient.AssertExpectations(t)
	})
}
