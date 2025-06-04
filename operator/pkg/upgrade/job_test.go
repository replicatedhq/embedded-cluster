package upgrade

import (
	"context"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateUpgradeJob_NodeAffinity(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	// Version used for testing
	testVersion := "1.2.3"

	// Create a minimal installation CR
	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-installation",
			Namespace: "default",
		},
		Spec: ecv1beta1.InstallationSpec{
			BinaryName: "test-binary",
			Config: &ecv1beta1.ConfigSpec{
				Version: testVersion,
				Domains: ecv1beta1.Domains{
					ProxyRegistryDomain: "registry.example.com",
				},
			},
		},
	}

	// Create a cached metadata for the test version
	// This avoids having to properly create a ConfigMap
	testMeta := types.ReleaseMetadata{
		Images: []string{"registry.example.com/embedded-cluster-operator-image:1.2.3"},
	}
	release.CacheMeta(testVersion, testMeta)

	// Create a fake client with the installation
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(installation).
		Build()

	rc := runtimeconfig.New(nil)

	// Call the function under test
	err := CreateUpgradeJob(
		context.Background(), cli, rc, installation,
		"registry.example.com/local-artifact-mirror:1.2.3",
		"license-id", "app-slug", "channel-id", testVersion,
		"1.2.2",
	)
	require.NoError(t, err)

	// Get the job that was created
	job := &batchv1.Job{}
	err = cli.Get(context.Background(), client.ObjectKey{
		Namespace: upgradeJobNamespace,
		Name:      "embedded-cluster-upgrade-test-installation",
	}, job)
	require.NoError(t, err)

	// Verify that the job has the expected node affinity
	require.NotNil(t, job.Spec.Template.Spec.Affinity, "Job should have affinity set")
	require.NotNil(t, job.Spec.Template.Spec.Affinity.NodeAffinity, "Job should have node affinity set")

	// Verify the preferred scheduling term
	preferredTerms := job.Spec.Template.Spec.Affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution
	require.NotEmpty(t, preferredTerms, "Job should have preferred scheduling terms")

	// Verify the weight and key
	assert.Equal(t, int32(100), preferredTerms[0].Weight, "Node affinity weight should be 100")
	assert.Equal(t, "node-role.kubernetes.io/control-plane", preferredTerms[0].Preference.MatchExpressions[0].Key,
		"Node affinity should target control-plane nodes")
	assert.Equal(t, corev1.NodeSelectorOpExists, preferredTerms[0].Preference.MatchExpressions[0].Operator,
		"Node affinity operator should be 'Exists'")
}

func TestCreateUpgradeJob_HostCABundle(t *testing.T) {
	// Test with HostCABundlePath set
	t.Run("with HostCABundlePath set", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, ecv1beta1.AddToScheme(scheme))
		require.NoError(t, batchv1.AddToScheme(scheme))
		require.NoError(t, corev1.AddToScheme(scheme))

		// Version used for testing
		testVersion := "1.2.3"
		testCAPath := "/etc/ssl/certs/ca-certificates.crt"

		// Create a minimal installation CR with RuntimeConfig.HostCABundlePath set
		installation := &ecv1beta1.Installation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-installation",
				Namespace: "default",
			},
			Spec: ecv1beta1.InstallationSpec{
				BinaryName: "test-binary",
				Config: &ecv1beta1.ConfigSpec{
					Version: testVersion,
					Domains: ecv1beta1.Domains{
						ProxyRegistryDomain: "registry.example.com",
					},
				},
				RuntimeConfig: &ecv1beta1.RuntimeConfigSpec{
					HostCABundlePath: testCAPath,
				},
			},
		}

		// Create a cached metadata for the test version
		// This avoids having to properly create a ConfigMap
		testMeta := types.ReleaseMetadata{
			Images: []string{"registry.example.com/embedded-cluster-operator-image:1.2.3"},
		}
		release.CacheMeta(testVersion, testMeta)

		// Create a fake client with the installation
		cli := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(installation).
			Build()

		rc := runtimeconfig.New(nil)

		// Call the function under test
		err := CreateUpgradeJob(
			context.Background(), cli, rc, installation,
			"registry.example.com/local-artifact-mirror:1.2.3",
			"license-id", "app-slug", "channel-id", testVersion,
			"1.2.2",
		)
		require.NoError(t, err)

		// Get the job that was created
		job := &batchv1.Job{}
		err = cli.Get(context.Background(), client.ObjectKey{
			Namespace: upgradeJobNamespace,
			Name:      "embedded-cluster-upgrade-test-installation",
		}, job)
		require.NoError(t, err)

		// Verify that the host CA bundle volume exists
		var hostCABundleVolumeFound bool
		for _, volume := range job.Spec.Template.Spec.Volumes {
			if volume.Name == "host-ca-bundle" {
				hostCABundleVolumeFound = true
				// Verify the volume properties
				require.NotNil(t, volume.HostPath, "Host CA bundle volume should be a hostPath volume")
				assert.Equal(t, testCAPath, volume.HostPath.Path, "Host CA bundle path should match RuntimeConfig.HostCABundlePath")
				assert.Equal(t, corev1.HostPathFileOrCreate, *volume.HostPath.Type, "Host CA bundle type should be FileOrCreate")
				break
			}
		}
		assert.True(t, hostCABundleVolumeFound, "Host CA bundle volume should exist")

		// Verify that the volume mount exists
		var hostCABundleMountFound bool
		for _, mount := range job.Spec.Template.Spec.Containers[0].VolumeMounts {
			if mount.Name == "host-ca-bundle" {
				hostCABundleMountFound = true
				// Verify the mount properties
				assert.Equal(t, "/certs/ca-certificates.crt", mount.MountPath, "Host CA bundle mount path should be correct")
				break
			}
		}
		assert.True(t, hostCABundleMountFound, "Host CA bundle mount should exist")

		// Verify that the SSL_CERT_DIR environment variable exists
		var sslCertDirEnvFound bool
		for _, env := range job.Spec.Template.Spec.Containers[0].Env {
			if env.Name == "SSL_CERT_DIR" {
				sslCertDirEnvFound = true
				// Verify the env var value
				assert.Equal(t, "/certs", env.Value, "SSL_CERT_DIR value should be correct")
				break
			}
		}
		assert.True(t, sslCertDirEnvFound, "SSL_CERT_DIR environment variable should exist")

		// Verify the "private-cas" volume does NOT exist
		var privateCasVolumeFound bool
		for _, volume := range job.Spec.Template.Spec.Volumes {
			if volume.Name == "private-cas" {
				privateCasVolumeFound = true
				break
			}
		}
		assert.False(t, privateCasVolumeFound, "private-cas volume should not exist")
	})

	// Test without HostCABundlePath set
	t.Run("without HostCABundlePath set", func(t *testing.T) {
		scheme := runtime.NewScheme()
		require.NoError(t, ecv1beta1.AddToScheme(scheme))
		require.NoError(t, batchv1.AddToScheme(scheme))
		require.NoError(t, corev1.AddToScheme(scheme))

		// Version used for testing
		testVersion := "1.2.3"

		// Create a minimal installation CR without RuntimeConfig.HostCABundlePath
		installation := &ecv1beta1.Installation{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-installation",
				Namespace: "default",
			},
			Spec: ecv1beta1.InstallationSpec{
				BinaryName: "test-binary",
				Config: &ecv1beta1.ConfigSpec{
					Version: testVersion,
					Domains: ecv1beta1.Domains{
						ProxyRegistryDomain: "registry.example.com",
					},
				},
				// No RuntimeConfig or empty RuntimeConfig
			},
		}

		// Create a cached metadata for the test version
		// This avoids having to properly create a ConfigMap
		testMeta := types.ReleaseMetadata{
			Images: []string{"registry.example.com/embedded-cluster-operator-image:1.2.3"},
		}
		release.CacheMeta(testVersion, testMeta)

		// Create a fake client with the installation
		cli := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(installation).
			Build()

		rc := runtimeconfig.New(nil)

		// Call the function under test
		err := CreateUpgradeJob(
			context.Background(), cli, rc, installation,
			"registry.example.com/local-artifact-mirror:1.2.3",
			"license-id", "app-slug", "channel-id", testVersion,
			"1.2.2",
		)
		require.NoError(t, err)

		// Get the job that was created
		job := &batchv1.Job{}
		err = cli.Get(context.Background(), client.ObjectKey{
			Namespace: upgradeJobNamespace,
			Name:      "embedded-cluster-upgrade-test-installation",
		}, job)
		require.NoError(t, err)

		// Verify that the host CA bundle volume does NOT exist
		var hostCABundleVolumeFound bool
		for _, volume := range job.Spec.Template.Spec.Volumes {
			if volume.Name == "host-ca-bundle" {
				hostCABundleVolumeFound = true
				break
			}
		}
		assert.False(t, hostCABundleVolumeFound, "Host CA bundle volume should not exist when HostCABundlePath is not set")

		// Verify that the volume mount does NOT exist
		var hostCABundleMountFound bool
		for _, mount := range job.Spec.Template.Spec.Containers[0].VolumeMounts {
			if mount.Name == "host-ca-bundle" {
				hostCABundleMountFound = true
				break
			}
		}
		assert.False(t, hostCABundleMountFound, "Host CA bundle mount should not exist when HostCABundlePath is not set")

		// Verify that the SSL_CERT_DIR environment variable does NOT exist
		var sslCertDirEnvFound bool
		for _, env := range job.Spec.Template.Spec.Containers[0].Env {
			if env.Name == "SSL_CERT_DIR" {
				sslCertDirEnvFound = true
				break
			}
		}
		assert.False(t, sslCertDirEnvFound, "SSL_CERT_DIR environment variable should not exist when HostCABundlePath is not set")

		// Verify the "private-cas" volume does NOT exist
		var privateCasVolumeFound bool
		for _, volume := range job.Spec.Template.Spec.Volumes {
			if volume.Name == "private-cas" {
				privateCasVolumeFound = true
				break
			}
		}
		assert.False(t, privateCasVolumeFound, "private-cas volume should not exist")
	})
}
