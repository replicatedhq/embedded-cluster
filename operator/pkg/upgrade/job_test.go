package upgrade

import (
	"context"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
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

	// Call the function under test
	err := CreateUpgradeJob(
		context.Background(), cli, installation,
		"registry.example.com/local-artifact-mirror:1.2.3",
		"license-id", "app-slug", "channel-slug", testVersion,
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
