package registry

import (
	"context"
	"testing"
	"time"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	clusterv1beta1 "github.com/replicatedhq/embedded-cluster-kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster-kinds/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func Test_ensureSeaweedfsS3Secret(t *testing.T) {
	type args struct {
		metadata *ectypes.ReleaseMetadata
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
			name: "create secret",
			args: args{
				metadata: &ectypes.ReleaseMetadata{
					BuiltinConfigs: map[string]k0sv1beta1.HelmExtensions{
						"seaweedfs": {
							Charts: []k0sv1beta1.Chart{
								{
									TargetNS: "seaweedfs",
									Values:   `{"filer":{"s3":{"existingConfigSecret":"secret-seaweedfs-s3"}}}`,
								},
							},
						},
					},
				},
			},
			wantOp: controllerutil.OperationResultCreated,
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

				assert.Contains(t, secret.Data, "seaweedfs_s3_config")
			},
		},
		{
			name: "update secret",
			args: args{
				metadata: &ectypes.ReleaseMetadata{
					BuiltinConfigs: map[string]k0sv1beta1.HelmExtensions{
						"seaweedfs": {
							Charts: []k0sv1beta1.Chart{
								{
									TargetNS: "seaweedfs",
									Values:   `{"filer":{"s3":{"existingConfigSecret":"secret-seaweedfs-s3"}}}`,
								},
							},
						},
					},
				},
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
				},
			},
			wantOp: controllerutil.OperationResultUpdated,
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

				assert.Contains(t, secret.Data, "seaweedfs_s3_config")
			},
		},
		{
			name: "no change",
			args: args{
				metadata: &ectypes.ReleaseMetadata{
					BuiltinConfigs: map[string]k0sv1beta1.HelmExtensions{
						"seaweedfs": {
							Charts: []k0sv1beta1.Chart{
								{
									TargetNS: "seaweedfs",
									Values:   `{"filer":{"s3":{"existingConfigSecret":"secret-seaweedfs-s3"}}}`,
								},
							},
						},
					},
				},
			},
			initRuntimeObjs: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "seaweedfs",
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       "seaweedfs",
						Name:            "secret-seaweedfs-s3",
						OwnerReferences: []metav1.OwnerReference{ownerReference()},
					},
					Data: map[string][]byte{
						"seaweedfs_s3_config": []byte(`{"identities":[` +
							`{"name":"anvAdmin","credentials":[{"accessKey":"Ik1yeEVtWVgHFJGQnsCu","secretKey":"5U1QVkIxBhsQnmxRRHeqR1NqOLe4VEtX53Xc5vQt"}],"actions":["Admin","Read","Write"]},` +
							`{"name":"anvReadOnly","credentials":[{"accessKey":"PnDSwipef1EcuzaElabs","secretKey":"YdK8MytjZXF7jbggQkbxJVwnVH1Jpc3whLJijBRK"}],"actions":["Read"]}` +
							`]}`),
					},
				},
			},
			wantOp: controllerutil.OperationResultNone,
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
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := fake.NewClientBuilder().
				WithScheme(scheme(t)).
				WithRuntimeObjects(tt.initRuntimeObjs...).
				Build()

			_, gotOp, err := ensureSeaweedfsS3Secret(context.Background(), installation(), tt.args.metadata, cli)
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

func Test_ensureRegistryS3Secret(t *testing.T) {
	type args struct {
		metadata  *ectypes.ReleaseMetadata
		sfsConfig *seaweedfsConfig
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
			name: "create secret",
			args: args{
				metadata: &ectypes.ReleaseMetadata{
					BuiltinConfigs: map[string]k0sv1beta1.HelmExtensions{
						"registry-ha": {
							Charts: []k0sv1beta1.Chart{
								{
									TargetNS: "registry",
									Values:   `{"secrets":{"s3":{"secretRef":"seaweedfs-s3-rw"}}}`,
								},
							},
						},
					},
				},
				sfsConfig: &seaweedfsConfig{
					Identities: []seaweedfsIdentity{
						{
							Name: "anvAdmin",
							Credentials: []seaweedfsIdentityCredential{
								{
									AccessKey: "ACCESSKEY",
									SecretKey: "SECRETKEY",
								},
							},
						},
					},
				},
			},
			wantOp: controllerutil.OperationResultCreated,
			assertRuntime: func(t *testing.T, cli client.Client) {
				namespace := &corev1.Namespace{}
				err := cli.Get(context.Background(), client.ObjectKey{Name: "registry"}, namespace)
				require.NoError(t, err)

				secret := &corev1.Secret{}
				err = cli.Get(context.Background(), client.ObjectKey{Namespace: "registry", Name: "seaweedfs-s3-rw"}, secret)
				require.NoError(t, err)

				if assert.Len(t, secret.OwnerReferences, 1) {
					assert.Equal(t, secret.OwnerReferences[0].Name, "embedded-cluster-kinds")
				}

				if assert.Contains(t, secret.Data, "s3AccessKey") {
					assert.Equal(t, "ACCESSKEY", string(secret.Data["s3AccessKey"]))
				}
				if assert.Contains(t, secret.Data, "s3SecretKey") {
					assert.Equal(t, "SECRETKEY", string(secret.Data["s3SecretKey"]))
				}
			},
		},
		{
			name: "update secret",
			args: args{
				metadata: &ectypes.ReleaseMetadata{
					BuiltinConfigs: map[string]k0sv1beta1.HelmExtensions{
						"registry-ha": {
							Charts: []k0sv1beta1.Chart{
								{
									TargetNS: "registry",
									Values:   `{"secrets":{"s3":{"secretRef":"seaweedfs-s3-rw"}}}`,
								},
							},
						},
					},
				},
				sfsConfig: &seaweedfsConfig{
					Identities: []seaweedfsIdentity{
						{
							Name: "anvAdmin",
							Credentials: []seaweedfsIdentityCredential{
								{
									AccessKey: "ACCESSKEY",
									SecretKey: "SECRETKEY",
								},
							},
						},
					},
				},
			},
			initRuntimeObjs: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry",
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "registry",
						Name:      "seaweedfs-s3-rw",
					},
				},
			},
			wantOp: controllerutil.OperationResultUpdated,
			assertRuntime: func(t *testing.T, cli client.Client) {
				namespace := &corev1.Namespace{}
				err := cli.Get(context.Background(), client.ObjectKey{Name: "registry"}, namespace)
				require.NoError(t, err)

				secret := &corev1.Secret{}
				err = cli.Get(context.Background(), client.ObjectKey{Namespace: "registry", Name: "seaweedfs-s3-rw"}, secret)
				require.NoError(t, err)

				if assert.Len(t, secret.OwnerReferences, 1) {
					assert.Equal(t, secret.OwnerReferences[0].Name, "embedded-cluster-kinds")
				}

				if assert.Contains(t, secret.Data, "s3AccessKey") {
					assert.Equal(t, "ACCESSKEY", string(secret.Data["s3AccessKey"]))
				}
				if assert.Contains(t, secret.Data, "s3SecretKey") {
					assert.Equal(t, "SECRETKEY", string(secret.Data["s3SecretKey"]))
				}
			},
		},
		{
			name: "no change",
			args: args{
				metadata: &ectypes.ReleaseMetadata{
					BuiltinConfigs: map[string]k0sv1beta1.HelmExtensions{
						"registry-ha": {
							Charts: []k0sv1beta1.Chart{
								{
									TargetNS: "registry",
									Values:   `{"secrets":{"s3":{"secretRef":"seaweedfs-s3-rw"}}}`,
								},
							},
						},
					},
				},
				sfsConfig: &seaweedfsConfig{
					Identities: []seaweedfsIdentity{
						{
							Name: "anvAdmin",
							Credentials: []seaweedfsIdentityCredential{
								{
									AccessKey: "ACCESSKEY",
									SecretKey: "SECRETKEY",
								},
							},
						},
					},
				},
			},
			initRuntimeObjs: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "registry",
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace:       "registry",
						Name:            "seaweedfs-s3-rw",
						OwnerReferences: []metav1.OwnerReference{ownerReference()},
					},
					Data: map[string][]byte{
						"s3AccessKey": []byte("ACCESSKEY"),
						"s3SecretKey": []byte("SECRETKEY"),
					},
				},
			},
			wantOp: controllerutil.OperationResultNone,
			assertRuntime: func(t *testing.T, cli client.Client) {
				namespace := &corev1.Namespace{}
				err := cli.Get(context.Background(), client.ObjectKey{Name: "registry"}, namespace)
				require.NoError(t, err)

				secret := &corev1.Secret{}
				err = cli.Get(context.Background(), client.ObjectKey{Namespace: "registry", Name: "seaweedfs-s3-rw"}, secret)
				require.NoError(t, err)

				if assert.Len(t, secret.OwnerReferences, 1) {
					assert.Equal(t, secret.OwnerReferences[0].Name, "embedded-cluster-kinds")
				}

				if assert.Contains(t, secret.Data, "s3AccessKey") {
					assert.Equal(t, "ACCESSKEY", string(secret.Data["s3AccessKey"]))
				}
				if assert.Contains(t, secret.Data, "s3SecretKey") {
					assert.Equal(t, "SECRETKEY", string(secret.Data["s3SecretKey"]))
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := fake.NewClientBuilder().
				WithScheme(scheme(t)).
				WithRuntimeObjects(tt.initRuntimeObjs...).
				Build()

			gotOp, err := ensureRegistryS3Secret(context.Background(), installation(), tt.args.metadata, cli, tt.args.sfsConfig)
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

func TestEnsureSecrets(t *testing.T) {
	type args struct {
		in       *clusterv1beta1.Installation
		metadata *ectypes.ReleaseMetadata
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
								Type:               SeaweedfsS3SecretReadyConditionType,
								Status:             metav1.ConditionTrue,
								Reason:             "SecretReady",
								ObservedGeneration: int64(1),
								LastTransitionTime: metav1.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
							},
						},
					}
				}),
				metadata: &ectypes.ReleaseMetadata{
					BuiltinConfigs: map[string]k0sv1beta1.HelmExtensions{
						"seaweedfs": {
							Charts: []k0sv1beta1.Chart{
								{
									TargetNS: "seaweedfs",
									Values:   `{"filer":{"s3":{"existingConfigSecret":"secret-seaweedfs-s3"}}}`,
								},
							},
						},
						"registry-ha": {
							Charts: []k0sv1beta1.Chart{
								{
									TargetNS: "registry",
									Values:   `{"secrets":{"s3":{"secretRef":"seaweedfs-s3-rw"}}}`,
								},
							},
						},
					},
				},
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
				if !assert.Len(t, in.Status.Conditions, 2) {
					return
				}

				assert.Equal(t, metav1.Condition{
					Type:               SeaweedfsS3SecretReadyConditionType,
					Status:             metav1.ConditionTrue,
					Reason:             "SecretReady",
					ObservedGeneration: int64(2),
					LastTransitionTime: metav1.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
				}, in.Status.Conditions[0])

				assert.Equal(t, RegistryS3SecretReadyConditionType, in.Status.Conditions[1].Type)
				assert.Equal(t, metav1.ConditionTrue, in.Status.Conditions[1].Status)
				assert.Equal(t, "SecretReady", in.Status.Conditions[1].Reason)
				assert.Equal(t, int64(2), in.Status.Conditions[1].ObservedGeneration)
				assert.WithinDuration(t, metav1.Now().Time, in.Status.Conditions[1].LastTransitionTime.Time, time.Minute)
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
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := fake.NewClientBuilder().
				WithScheme(scheme(t)).
				WithRuntimeObjects(tt.initRuntimeObjs...).
				Build()

			err := EnsureSecrets(context.Background(), tt.args.in, tt.args.metadata, cli)
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

func scheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	require.NoError(t, err)
	err = clusterv1beta1.SchemeBuilder.AddToScheme(scheme)
	require.NoError(t, err)
	return scheme
}
