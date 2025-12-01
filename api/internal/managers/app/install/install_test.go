package install

import (
	"context"
	"os"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	kotscli "github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	kyaml "sigs.k8s.io/yaml"
)

func TestAppInstallManager_Install(t *testing.T) {
	// Setup environment variable for V3
	t.Setenv("ENABLE_V3", "1")

	// Create test license with proper Kubernetes resource format
	licenseYAML := `apiVersion: kots.io/v1beta1
kind: License
spec:
  appSlug: test-app
`
	licenseBytes := []byte(licenseYAML)

	// Create test release data
	releaseData := &release.ReleaseData{
		ChannelRelease: &release.ChannelRelease{
			DefaultDomains: release.Domains{
				ReplicatedAppDomain: "replicated.app",
			},
		},
	}

	// Set up release data globally so AppSlug() returns the correct value for v3
	err := release.SetReleaseDataForTests(map[string][]byte{
		"channelrelease.yaml": []byte("# channel release object\nappSlug: test-app"),
	})
	require.NoError(t, err)

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

		// Create mock installer with detailed verification
		mockKotsCLI := &kotscli.MockKotsCLI{}
		mockKotsCLI.On("Install", mock.MatchedBy(func(opts kotscli.InstallOptions) bool {
			// Verify basic install options
			if opts.AppSlug != "test-app" {
				t.Logf("AppSlug mismatch: expected 'test-app', got '%s'", opts.AppSlug)
				return false
			}
			if opts.License == nil {
				t.Logf("License is nil")
				return false
			}
			if opts.Namespace != "test-app" {
				t.Logf("Namespace mismatch: expected 'test-app', got '%s'", opts.Namespace)
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
			if !opts.DisableImagePush {
				t.Logf("DisableImagePush is false")
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

		// Create fake kube client
		sch := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(sch))
		require.NoError(t, scheme.AddToScheme(sch))
		fakeKcli := clientfake.NewClientBuilder().WithScheme(sch).Build()

		// Create manager
		manager, err := NewAppInstallManager(
			WithLicense(licenseBytes),
			WithClusterID("test-cluster"),
			WithAirgapBundle("test-airgap.tar.gz"),
			WithReleaseData(releaseData),
			WithKotsCLI(mockKotsCLI),
			WithLogger(logger.NewDiscardLogger()),
			WithKubeClient(fakeKcli),
		)
		require.NoError(t, err)

		// Run installation
		err = manager.Install(context.Background(), configValues)
		require.NoError(t, err)

		// Verify mock was called
		mockKotsCLI.AssertExpectations(t)
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
