package migratev2

import (
	"context"
	"testing"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_updateAdminConsoleClusterConfig(t *testing.T) {
	tests := []struct {
		name           string
		initialCharts  k0sv1beta1.ChartsSettings
		expectedCharts k0sv1beta1.ChartsSettings
		expectError    bool
	}{
		{
			name: "updates admin-console chart values",
			initialCharts: k0sv1beta1.ChartsSettings{
				{
					Name:   "admin-console",
					Values: "foo: bar",
				},
				{
					Name:   "other-chart",
					Values: "unchanged: true",
				},
			},
			expectedCharts: k0sv1beta1.ChartsSettings{
				{
					Name:   "admin-console",
					Values: "foo: bar\nisEC2Install: \"true\"\n",
				},
				{
					Name:   "other-chart",
					Values: "unchanged: true",
				},
			},
			expectError: false,
		},
		{
			name: "does not update admin-console chart values if already set",
			initialCharts: k0sv1beta1.ChartsSettings{
				{
					Name:   "admin-console",
					Values: "foo: bar\nisEC2Install: \"true\"\n",
				},
				{
					Name:   "other-chart",
					Values: "unchanged: true",
				},
			},
			expectedCharts: k0sv1beta1.ChartsSettings{
				{
					Name:   "admin-console",
					Values: "foo: bar\nisEC2Install: \"true\"\n",
				},
				{
					Name:   "other-chart",
					Values: "unchanged: true",
				},
			},
			expectError: false,
		},
		{
			name: "handles missing admin-console chart",
			initialCharts: k0sv1beta1.ChartsSettings{
				{
					Name:   "other-chart",
					Values: "unchanged: true",
				},
			},
			expectedCharts: k0sv1beta1.ChartsSettings{
				{
					Name:   "other-chart",
					Values: "unchanged: true",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, k0sv1beta1.AddToScheme(scheme))

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

			cli := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(initialConfig).
				Build()

			err := updateAdminConsoleClusterConfig(context.Background(), cli)

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

			assert.Equal(t, tt.expectedCharts, updatedConfig.Spec.Extensions.Helm.Charts)
		})
	}
}

func Test_isAdminConsoleDeploymentUpdated(t *testing.T) {
	tests := []struct {
		name    string
		pods    []corev1.Pod
		want    bool
		wantErr bool
	}{
		{
			name: "pod is ready and has correct env var",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-123",
						Namespace: "kotsadm",
						Labels: map[string]string{
							"app": "kotsadm",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "kotsadm",
								Env: []corev1.EnvVar{
									{
										Name:  "IS_EC2_INSTALL",
										Value: "true",
									},
								},
							},
						},
					},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "pod is ready but missing env var",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-123",
						Namespace: "kotsadm",
						Labels: map[string]string{
							"app": "kotsadm",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "kotsadm",
								Env:  []corev1.EnvVar{},
							},
						},
					},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionTrue,
							},
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "pod has env var but not ready",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-123",
						Namespace: "kotsadm",
						Labels: map[string]string{
							"app": "kotsadm",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "kotsadm",
								Env: []corev1.EnvVar{
									{
										Name:  "IS_EC2_INSTALL",
										Value: "true",
									},
								},
							},
						},
					},
					Status: corev1.PodStatus{
						Conditions: []corev1.PodCondition{
							{
								Type:   corev1.PodReady,
								Status: corev1.ConditionFalse,
							},
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "multiple pods during rolling update",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-123",
						Namespace: "kotsadm",
						Labels: map[string]string{
							"app": "kotsadm",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "kotsadm",
								Env: []corev1.EnvVar{
									{
										Name:  "IS_EC2_INSTALL",
										Value: "true",
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-124",
						Namespace: "kotsadm",
						Labels: map[string]string{
							"app": "kotsadm",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "kotsadm",
								Env: []corev1.EnvVar{
									{
										Name:  "IS_EC2_INSTALL",
										Value: "true",
									},
								},
							},
						},
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name:    "no pods found",
			pods:    []corev1.Pod{},
			want:    false,
			wantErr: false,
		},
	}

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := fake.NewClientBuilder().
				WithObjects(podsToRuntimeObjects(tt.pods)...).
				Build()

			got, err := isAdminConsoleDeploymentUpdated(context.Background(), cli)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func podsToRuntimeObjects(pods []corev1.Pod) []client.Object {
	objects := make([]client.Object, len(pods))
	for i := range pods {
		objects[i] = &pods[i]
	}
	return objects
}

func Test_updateAdminConsoleChartValues(t *testing.T) {
	type args struct {
		values []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "basic",
			args: args{
				values: []byte(`isAirgap: "true"
embeddedClusterID: e79f0701-67f3-4abf-a672-42a1f830064e
`),
			},
			want: []byte(`embeddedClusterID: e79f0701-67f3-4abf-a672-42a1f830064e
isAirgap: "true"
isEC2Install: "true"
`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := updateAdminConsoleChartValues(tt.args.values)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, string(tt.want), string(got))
		})
	}
}
