package registry

import (
	"context"
	"testing"
	"time"

	clusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster-operator/pkg/k8sutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureResources(t *testing.T) {
	type args struct {
		in          *clusterv1beta1.Installation
		serviceCIDR string
	}
	tests := []struct {
		name            string
		args            args
		initRuntimeObjs []runtime.Object
		assertIn        func(t *testing.T, in *clusterv1beta1.Installation)
		assertRuntime   func(t *testing.T, cli client.Client)
		wantErr         bool
	}{
		{
			name: "basic",
			args: args{
				in: installation(func(in *clusterv1beta1.Installation) {
					in.Spec.AirGap = true
					in.Spec.HighAvailability = true
					in.Status = clusterv1beta1.InstallationStatus{
						Conditions: []metav1.Condition{
							{
								Type:               seaweedfsS3SecretReadyConditionType,
								Status:             metav1.ConditionTrue,
								Reason:             "SecretReady",
								ObservedGeneration: int64(1),
								LastTransitionTime: metav1.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
							},
						},
					}
				}),
				serviceCIDR: "10.96.0.0/12",
			},
			initRuntimeObjs: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "seaweedfs",
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "seaweedfs",
						Name:      "secret-seaweedfs-s3",
					},
					Data: map[string][]byte{
						"seaweedfs_s3_config": []byte(`{"identities":[` +
							`{"name":"anvAdmin","credentials":[{"accessKey":"Ik1yeEVtWVgHFJGQnsCu","secretKey":"5U1QVkIxBhsQnmxRRHeqR1NqOLe4VEtX53Xc5vQt"}],"actions":["Admin","Read","Write"]},` +
							`{"name":"anvReadOnly","credentials":[{"accessKey":"PnDSwipef1EcuzaElabs","secretKey":"YdK8MytjZXF7jbggQkbxJVwnVH1Jpc3whLJijBRK"}],"actions":["Read"]}` +
							`]}`),
					},
				},
			},
			assertIn: func(t *testing.T, in *clusterv1beta1.Installation) {
				if !assert.Len(t, in.Status.Conditions, 3) {
					return
				}

				assert.Equal(t, metav1.Condition{
					Type:               seaweedfsS3SecretReadyConditionType,
					Status:             metav1.ConditionTrue,
					Reason:             "SecretReady",
					ObservedGeneration: int64(2),
					LastTransitionTime: metav1.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
				}, in.Status.Conditions[0])

				assert.Equal(t, registryS3SecretReadyConditionType, in.Status.Conditions[1].Type)
				assert.Equal(t, metav1.ConditionTrue, in.Status.Conditions[1].Status)
				assert.Equal(t, "SecretReady", in.Status.Conditions[1].Reason)
				assert.Equal(t, int64(2), in.Status.Conditions[1].ObservedGeneration)
				assert.WithinDuration(t, metav1.Now().Time, in.Status.Conditions[1].LastTransitionTime.Time, time.Minute)

				assert.Equal(t, seaweedfsS3ServiceReadyConditionType, in.Status.Conditions[2].Type)
				assert.Equal(t, metav1.ConditionTrue, in.Status.Conditions[2].Status)
				assert.Equal(t, "ServiceReady", in.Status.Conditions[2].Reason)
				assert.Equal(t, int64(2), in.Status.Conditions[2].ObservedGeneration)
				assert.WithinDuration(t, metav1.Now().Time, in.Status.Conditions[2].LastTransitionTime.Time, time.Minute)
			},
			assertRuntime: func(t *testing.T, cli client.Client) {
				namespace := &corev1.Namespace{}
				err := cli.Get(context.Background(), client.ObjectKey{Name: "seaweedfs"}, namespace)
				require.NoError(t, err)

				secret := &corev1.Secret{}
				err = cli.Get(context.Background(), client.ObjectKey{Namespace: "seaweedfs", Name: "secret-seaweedfs-s3"}, secret)
				require.NoError(t, err)

				if assert.Len(t, secret.OwnerReferences, 1) {
					assert.Equal(t, secret.OwnerReferences[0].Name, "embedded-cluster-kinds")
				}

				if assert.Contains(t, secret.Data, "seaweedfs_s3_config") {
					assert.Equal(t, `{"identities":[`+
						`{"name":"anvAdmin","credentials":[{"accessKey":"Ik1yeEVtWVgHFJGQnsCu","secretKey":"5U1QVkIxBhsQnmxRRHeqR1NqOLe4VEtX53Xc5vQt"}],"actions":["Admin","Read","Write"]},`+
						`{"name":"anvReadOnly","credentials":[{"accessKey":"PnDSwipef1EcuzaElabs","secretKey":"YdK8MytjZXF7jbggQkbxJVwnVH1Jpc3whLJijBRK"}],"actions":["Read"]}`+
						`]}`,
						string(secret.Data["seaweedfs_s3_config"]))
				}

				namespace = &corev1.Namespace{}
				err = cli.Get(context.Background(), client.ObjectKey{Name: "registry"}, namespace)
				require.NoError(t, err)

				secret = &corev1.Secret{}
				err = cli.Get(context.Background(), client.ObjectKey{Namespace: "registry", Name: "seaweedfs-s3-rw"}, secret)
				require.NoError(t, err)

				if assert.Len(t, secret.OwnerReferences, 1) {
					assert.Equal(t, secret.OwnerReferences[0].Name, "embedded-cluster-kinds")
				}

				if assert.Contains(t, secret.Data, "s3AccessKey") {
					assert.Equal(t, "Ik1yeEVtWVgHFJGQnsCu", string(secret.Data["s3AccessKey"]))
				}
				if assert.Contains(t, secret.Data, "s3SecretKey") {
					assert.Equal(t, "5U1QVkIxBhsQnmxRRHeqR1NqOLe4VEtX53Xc5vQt", string(secret.Data["s3SecretKey"]))
				}

				service := &corev1.Service{}
				err = cli.Get(context.Background(), client.ObjectKey{Namespace: "seaweedfs", Name: "ec-seaweedfs-s3"}, service)
				require.NoError(t, err)

				if assert.Len(t, service.OwnerReferences, 1) {
					assert.Equal(t, service.OwnerReferences[0].Name, "embedded-cluster-kinds")
				}

				assert.Equal(t, "10.96.0.12", service.Spec.ClusterIP)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := fake.NewClientBuilder().
				WithScheme(scheme(t)).
				WithRuntimeObjects(tt.initRuntimeObjs...).
				Build()

			err := EnsureResources(context.Background(), tt.args.in, cli, tt.args.serviceCIDR)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			tt.assertIn(t, tt.args.in)
			tt.assertRuntime(t, cli)
		})
	}
}

func installation(options ...func(*clusterv1beta1.Installation)) *clusterv1beta1.Installation {
	in := &clusterv1beta1.Installation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clusterv1beta1.GroupVersion.String(),
			Kind:       "Installation",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "embedded-cluster-kinds",
			Generation: int64(2),
		},
		Spec: clusterv1beta1.InstallationSpec{
			BinaryName: "binary-name",
			ClusterID:  "cluster-id",
			Config: &clusterv1beta1.ConfigSpec{
				Version: "version",
			},
		},
	}
	for _, option := range options {
		option(in)
	}
	return in
}

func ownerReference() metav1.OwnerReference {
	in := installation()
	return metav1.OwnerReference{
		APIVersion:         clusterv1beta1.GroupVersion.String(),
		Kind:               "Installation",
		Name:               in.GetName(),
		UID:                in.GetUID(),
		BlockOwnerDeletion: ptr.To(true),
		Controller:         ptr.To(true),
	}
}

func labels(component string) map[string]string {
	in := installation()
	return k8sutil.ApplyCommonLabels(nil, in, component)
}

func scheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	require.NoError(t, err)
	err = clusterv1beta1.SchemeBuilder.AddToScheme(scheme)
	require.NoError(t, err)
	return scheme
}
