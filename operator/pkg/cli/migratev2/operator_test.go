package migratev2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_setOperatorDeploymentReplicasZero(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))

	tests := []struct {
		name        string
		deployment  *appsv1.Deployment
		expectError bool
		validate    func(t *testing.T, deployment appsv1.Deployment)
	}{
		{
			name: "scales down operator",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: OperatorDeploymentName, Namespace: OperatorNamespace},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](1),
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "embedded-cluster-operator",
									Image: "replicated-operator:v1",
								},
							},
						},
					},
				},
			},
			validate: func(t *testing.T, deployment appsv1.Deployment) {
				// Validate that the patch did not modify the rest of the deployment
				assert.Equal(t, 1, len(deployment.Spec.Template.Spec.Containers), "expected 1 container")
			},
			expectError: false,
		},
		{
			name: "scales down operator 0 replicas",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: OperatorDeploymentName, Namespace: OperatorNamespace},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](0),
				},
			},
			expectError: false,
		},
		{
			name:        "fails if operator is not found",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fake client with the test case's initial installation
			builder := fake.NewClientBuilder().
				WithScheme(scheme)
			if tt.deployment != nil {
				builder = builder.WithObjects(tt.deployment)
			}
			cli := builder.Build()

			// Call disableOperator
			err := setOperatorDeploymentReplicasZero(context.Background(), cli)

			// Check error expectation
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Verify the installation status was updated
			var updatedDeployment appsv1.Deployment
			nsn := apitypes.NamespacedName{Namespace: OperatorNamespace, Name: OperatorDeploymentName}
			err = cli.Get(context.Background(), nsn, &updatedDeployment)
			require.NoError(t, err)

			// Check that replicas has been scaled down
			assert.Equal(t, int32(0), *updatedDeployment.Spec.Replicas, "expected replicas to be 0")

			if tt.validate != nil {
				tt.validate(t, updatedDeployment)
			}
		})
	}
}

func Test_isOperatorDeploymentUpdated(t *testing.T) {
	tests := []struct {
		name    string
		pods    []corev1.Pod
		want    bool
		wantErr bool
	}{
		{
			name: "pod exists",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "embedded-cluster-operator-123",
						Namespace: OperatorNamespace,
						Labels:    operatorPodLabels(),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-other-pod",
						Namespace: OperatorNamespace,
					},
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "pod does not exist",
			pods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "some-other-pod",
						Namespace: OperatorNamespace,
					},
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name:    "no pods found",
			pods:    []corev1.Pod{},
			want:    true,
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

			got, err := isOperatorDeploymentUpdated(context.Background(), cli)
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
