package registry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func Test_ensureSeaweedfsS3Service(t *testing.T) {
	type args struct {
		clusterIP string
	}
	tests := []struct {
		name            string
		args            args
		initRuntimeObjs []runtime.Object
		wantOp          controllerutil.OperationResult
		assertRuntime   func(t *testing.T, cli client.Client)
		wantErr         bool
	}{
		{
			name:   "create service",
			args:   args{clusterIP: "1.1.1.1"},
			wantOp: controllerutil.OperationResultCreated,
			assertRuntime: func(t *testing.T, cli client.Client) {
				namespace := &corev1.Namespace{}
				err := cli.Get(context.Background(), client.ObjectKey{Name: "seaweedfs"}, namespace)
				require.NoError(t, err)

				service := &corev1.Service{}
				err = cli.Get(context.Background(), client.ObjectKey{Namespace: "seaweedfs", Name: "ec-seaweedfs-s3"}, service)
				require.NoError(t, err)

				if assert.Len(t, service.OwnerReferences, 1) {
					assert.Equal(t, service.OwnerReferences[0].Name, "embedded-cluster-kinds")
				}

				assert.Equal(t, "1.1.1.1", service.Spec.ClusterIP)
			},
		},
		{
			name: "update service",
			args: args{clusterIP: "1.1.1.1"},
			initRuntimeObjs: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "seaweedfs",
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "seaweedfs",
						Name:      "ec-seaweedfs-s3",
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: "2.2.2.2",
					},
				},
			},
			wantOp: controllerutil.OperationResultUpdated,
			assertRuntime: func(t *testing.T, cli client.Client) {
				namespace := &corev1.Namespace{}
				err := cli.Get(context.Background(), client.ObjectKey{Name: "seaweedfs"}, namespace)
				require.NoError(t, err)

				service := &corev1.Service{}
				err = cli.Get(context.Background(), client.ObjectKey{Namespace: "seaweedfs", Name: "ec-seaweedfs-s3"}, service)
				require.NoError(t, err)

				if assert.Len(t, service.OwnerReferences, 1) {
					assert.Equal(t, service.OwnerReferences[0].Name, "embedded-cluster-kinds")
				}

				assert.Equal(t, "1.1.1.1", service.Spec.ClusterIP)
			},
		},
		{
			name: "no change",
			args: args{clusterIP: "1.1.1.1"},
			initRuntimeObjs: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "seaweedfs",
					},
				},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       "seaweedfs",
						Name:            "ec-seaweedfs-s3",
						OwnerReferences: []metav1.OwnerReference{ownerReference()},
						Labels:          labels("s3"),
					},
					Spec: corev1.ServiceSpec{
						ClusterIP: "1.1.1.1",
						Ports: []corev1.ServicePort{
							{
								Name:       "swfs-s3",
								Port:       8333,
								Protocol:   corev1.ProtocolTCP,
								TargetPort: intstr.FromInt(8333),
							},
						},
						Selector: map[string]string{
							"app.kubernetes.io/component": "filer",
							"app.kubernetes.io/name":      "seaweedfs",
						},
					},
				},
			},
			wantOp: controllerutil.OperationResultNone,
			assertRuntime: func(t *testing.T, cli client.Client) {
				namespace := &corev1.Namespace{}
				err := cli.Get(context.Background(), client.ObjectKey{Name: "seaweedfs"}, namespace)
				require.NoError(t, err)

				service := &corev1.Service{}
				err = cli.Get(context.Background(), client.ObjectKey{Namespace: "seaweedfs", Name: "ec-seaweedfs-s3"}, service)
				require.NoError(t, err)

				if assert.Len(t, service.OwnerReferences, 1) {
					assert.Equal(t, service.OwnerReferences[0].Name, "embedded-cluster-kinds")
				}

				assert.Equal(t, "1.1.1.1", service.Spec.ClusterIP)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := fake.NewClientBuilder().
				WithScheme(scheme(t)).
				WithRuntimeObjects(tt.initRuntimeObjs...).
				Build()

			gotOp, err := ensureSeaweedfsS3Service(context.Background(), installation(), cli, tt.args.clusterIP)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantOp, gotOp)

			tt.assertRuntime(t, cli)
		})
	}
}
