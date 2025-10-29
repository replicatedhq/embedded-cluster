package migratev2

import (
	"context"
	"testing"

	k0shelmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNeedsK0sChartCleanup(t *testing.T) {
	tests := []struct {
		name        string
		objects     []runtime.Object
		wantCleanup bool
		wantErr     bool
	}{
		{
			name:        "no charts or config",
			objects:     []runtime.Object{},
			wantCleanup: false,
			wantErr:     true, // should error because cluster config is missing
		},
		{
			name: "empty cluster config",
			objects: []runtime.Object{
				&k0sv1beta1.ClusterConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "k0s",
						Namespace: "kube-system",
					},
					Spec: &k0sv1beta1.ClusterSpec{},
				},
			},
			wantCleanup: false,
			wantErr:     false,
		},
		{
			name: "has helm charts",
			objects: []runtime.Object{
				&k0sv1beta1.ClusterConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "k0s",
						Namespace: "kube-system",
					},
					Spec: &k0sv1beta1.ClusterSpec{},
				},
				&k0shelmv1beta1.Chart{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-chart",
					},
				},
			},
			wantCleanup: true,
			wantErr:     false,
		},
		{
			name: "has helm extensions in config",
			objects: []runtime.Object{
				&k0sv1beta1.ClusterConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "k0s",
						Namespace: "kube-system",
					},
					Spec: &k0sv1beta1.ClusterSpec{
						Extensions: &k0sv1beta1.ClusterExtensions{
							Helm: &k0sv1beta1.HelmExtensions{
								Charts: []k0sv1beta1.Chart{
									{
										Name:    "test-chart",
										Version: "1.0.0",
									},
								},
							},
						},
					},
				},
			},
			wantCleanup: true,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			err := k0sv1beta1.AddToScheme(scheme)
			require.NoError(t, err)
			//nolint:staticcheck // SA1019 we are using the deprecated scheme for backwards compatibility, we can remove this once we stop supporting k0s v1.30
			err = k0shelmv1beta1.AddToScheme(scheme)
			require.NoError(t, err)

			cli := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(tt.objects...).
				Build()

			needsCleanup, err := needsK0sChartCleanup(context.Background(), cli)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantCleanup, needsCleanup)
		})
	}
}
