package migratev2

import (
	"context"
	"testing"

	k0shelmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_disableOperator(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, ecv1beta1.AddToScheme(scheme))

	tests := []struct {
		name         string
		installation *ecv1beta1.Installation
		expectError  bool
	}{
		{
			name: "disables operator reconciliation",
			installation: &ecv1beta1.Installation{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-installation",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake client with the test case's initial installation
			cli := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.installation).
				WithStatusSubresource(tt.installation).
				Build()

			// Call disableOperator
			err := disableOperator(context.Background(), t.Logf, cli, tt.installation)

			// Check error expectation
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify the installation status was updated
			var updatedInstallation ecv1beta1.Installation
			err = cli.Get(context.Background(), client.ObjectKey{Name: tt.installation.Name}, &updatedInstallation)
			require.NoError(t, err)

			// Check that the DisableReconcile condition was set correctly
			condition := meta.FindStatusCondition(updatedInstallation.Status.Conditions, ecv1beta1.ConditionTypeDisableReconcile)
			require.NotNil(t, condition)
			assert.Equal(t, metav1.ConditionTrue, condition.Status)
			assert.Equal(t, "V2MigrationInProgress", condition.Reason)
			assert.Equal(t, tt.installation.Generation, condition.ObservedGeneration)
		})
	}
}

func Test_forceDeleteChartCRs(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, k0sv1beta1.AddToScheme(scheme))
	require.NoError(t, k0shelmv1beta1.AddToScheme(scheme))

	tests := []struct {
		name        string
		initialCRs  []k0shelmv1beta1.Chart
		expectError bool
	}{
		{
			name: "removes finalizers and deletes charts",
			initialCRs: []k0shelmv1beta1.Chart{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "chart1",
						Namespace:  "default",
						Finalizers: []string{"helm.k0s.k0sproject.io/uninstall-helm-release"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "chart2",
						Namespace:  "default",
						Finalizers: []string{"helm.k0s.k0sproject.io/uninstall-helm-release"},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "handles no charts",
			initialCRs:  []k0shelmv1beta1.Chart{},
			expectError: false,
		},
		{
			name: "handles charts without finalizers",
			initialCRs: []k0shelmv1beta1.Chart{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "chart1",
						Namespace: "default",
					},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake client with the test case's initial CRs
			cli := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(chartCRsToRuntimeObjects(tt.initialCRs)...).
				Build()

			// Call forceDeleteChartCRs
			err := forceDeleteChartCRs(context.Background(), cli)

			// Check error expectation
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify all charts were deleted
			var remainingCharts k0shelmv1beta1.ChartList
			err = cli.List(context.Background(), &remainingCharts)
			require.NoError(t, err)
			assert.Empty(t, remainingCharts.Items, "expected all charts to be deleted")
		})
	}
}

// Helper function to convert []Chart to []runtime.Object
func chartCRsToRuntimeObjects(charts []k0shelmv1beta1.Chart) []client.Object {
	objects := make([]client.Object, len(charts))
	for i := range charts {
		objects[i] = &charts[i]
	}
	return objects
}

func Test_removeClusterConfigHelmExtensions(t *testing.T) {
	tests := []struct {
		name          string
		initialConfig *k0sv1beta1.ClusterConfig
		expectError   bool
	}{
		{
			name: "cleans up helm extensions",
			initialConfig: &k0sv1beta1.ClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k0s",
					Namespace: "kube-system",
				},
				Spec: &k0sv1beta1.ClusterSpec{
					Extensions: &k0sv1beta1.ClusterExtensions{
						Helm: &k0sv1beta1.HelmExtensions{
							Charts: k0sv1beta1.ChartsSettings{
								{Name: "chart1"},
								{Name: "chart2"},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "handles nil extensions",
			initialConfig: &k0sv1beta1.ClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k0s",
					Namespace: "kube-system",
				},
				Spec: &k0sv1beta1.ClusterSpec{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, k0sv1beta1.AddToScheme(scheme))

			cli := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.initialConfig).
				Build()

			err := removeClusterConfigHelmExtensions(context.Background(), cli)

			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			var updatedConfig k0sv1beta1.ClusterConfig
			err = cli.Get(context.Background(), apitypes.NamespacedName{
				Namespace: "kube-system",
				Name:      "k0s",
			}, &updatedConfig)
			require.NoError(t, err)

			assert.Equal(t, &k0sv1beta1.HelmExtensions{}, updatedConfig.Spec.Extensions.Helm)
		})
	}
}
