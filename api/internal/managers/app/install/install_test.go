package install

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	helmrelease "helm.sh/helm/v3/pkg/release"
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

	t.Run("Config values should be passed to the installer", func(t *testing.T) {
		configValues := kotsv1beta1.ConfigValues{
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

		// Create mock helm client
		mockHelmClient := &helm.MockClient{}
		mockHelmClient.On("Install", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			return opts.ChartPath != "" && opts.ReleaseName == "test-chart" && opts.Namespace == "kotsadm"
		})).Return(&helmrelease.Release{Name: "test-chart"}, nil)

		// Create mock installer with detailed verification
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
			WithKotsCLI(mockInstaller),
			WithHelmClient(mockHelmClient),
			WithLogger(logger.NewDiscardLogger()),
		)
		require.NoError(t, err)

		// Run installation
		err = manager.Install(context.Background(), configValues)
		require.NoError(t, err)

		mockInstaller.AssertExpectations(t)
		mockHelmClient.AssertExpectations(t)
	})

	t.Run("Install updates status correctly", func(t *testing.T) {
		// Create mock helm client
		mockHelmClient := &helm.MockClient{}
		mockHelmClient.On("Install", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			return opts.ChartPath != "" && opts.ReleaseName == "test-chart" && opts.Namespace == "kotsadm"
		})).Return(&helmrelease.Release{Name: "test-chart"}, nil)

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
		err = manager.Install(context.Background(), kotsv1beta1.ConfigValues{})
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
		// Create mock helm client that fails
		mockHelmClient := &helm.MockClient{}
		mockHelmClient.On("Install", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			return opts.ChartPath != "" && opts.ReleaseName == "test-chart" && opts.Namespace == "kotsadm"
		})).Return((*helmrelease.Release)(nil), assert.AnError)

		// Create manager with initialized store (no need for KOTS installer mock since Helm fails first)
		store := appinstallstore.NewMemoryStore(appinstallstore.WithAppInstall(types.AppInstall{
			Status: types.Status{State: types.StatePending},
		}))
		manager, err := NewAppInstallManager(
			WithLicense(licenseBytes),
			WithClusterID("test-cluster"),
			WithReleaseData(releaseData),
			WithHelmClient(mockHelmClient),
			WithLogger(logger.NewDiscardLogger()),
			WithAppInstallStore(store),
		)
		require.NoError(t, err)

		// Run installation (should fail)
		err = manager.Install(context.Background(), kotsv1beta1.ConfigValues{})
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

	configValues := kotsv1beta1.ConfigValues{
		Spec: kotsv1beta1.ConfigValuesSpec{
			Values: map[string]kotsv1beta1.ConfigValue{
				"testKey": {
					Value: "testValue",
				},
			},
		},
	}

	filename, err := manager.createConfigValuesFile(configValues)
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
