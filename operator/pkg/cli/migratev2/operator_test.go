package migratev2

import (
	"context"
	"testing"

	k0shelmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_removeOperatorFromClusterConfig(t *testing.T) {
	tests := []struct {
		name           string
		initialCharts  k0sv1beta1.ChartsSettings
		expectedCharts k0sv1beta1.ChartsSettings
		expectError    bool
	}{
		{
			name: "removes operator chart while preserving others",
			initialCharts: k0sv1beta1.ChartsSettings{
				{Name: "embedded-cluster-operator"},
				{Name: "other-chart"},
			},
			expectedCharts: k0sv1beta1.ChartsSettings{
				{Name: "other-chart"},
			},
			expectError: false,
		},
		{
			name:           "handles empty charts",
			initialCharts:  nil,
			expectedCharts: nil,
			expectError:    false,
		},
		{
			name: "handles config with no operator chart",
			initialCharts: k0sv1beta1.ChartsSettings{
				{Name: "other-chart-1"},
				{Name: "other-chart-2"},
			},
			expectedCharts: k0sv1beta1.ChartsSettings{
				{Name: "other-chart-1"},
				{Name: "other-chart-2"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		scheme := runtime.NewScheme()
		require.NoError(t, k0sv1beta1.AddToScheme(scheme))
		require.NoError(t, k0shelmv1beta1.AddToScheme(scheme))

		t.Run(tt.name, func(t *testing.T) {
			initialConfig := &k0sv1beta1.ClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k0s",
					Namespace: "kube-system",
				},
				Spec: &k0sv1beta1.ClusterSpec{
					Extensions: &k0sv1beta1.ClusterExtensions{
						Helm: &k0sv1beta1.HelmExtensions{
							Charts: tt.initialCharts,
						},
					},
				},
			}

			// Create a fake client with the test case's initial config
			cli := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(initialConfig).
				Build()

			// Call removeOperatorFromClusterConfig
			err := removeOperatorFromClusterConfig(context.Background(), cli)

			// Check error expectation
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify the updated config
			var updatedConfig k0sv1beta1.ClusterConfig
			err = cli.Get(context.Background(), apitypes.NamespacedName{
				Namespace: "kube-system",
				Name:      "k0s",
			}, &updatedConfig)
			require.NoError(t, err)

			// If we expect no charts, verify extensions or helm is nil
			if tt.expectedCharts == nil {
				if updatedConfig.Spec.Extensions != nil && updatedConfig.Spec.Extensions.Helm != nil {
					assert.Empty(t, updatedConfig.Spec.Extensions.Helm.Charts)
				}
				return
			}

			// Verify the remaining charts match expectations
			require.NotNil(t, updatedConfig.Spec.Extensions)
			require.NotNil(t, updatedConfig.Spec.Extensions.Helm)
			assert.Equal(t, tt.expectedCharts, updatedConfig.Spec.Extensions.Helm.Charts)
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
