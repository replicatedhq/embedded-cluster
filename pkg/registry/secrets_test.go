package registry

import (
	"context"
	"testing"

	"github.com/replicatedhq/embedded-cluster-operator/pkg/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func Test_ensureSeaweedfsS3Secret(t *testing.T) {
	tests := []struct {
		name            string
		initRuntimeObjs []runtime.Object
		wantOp          controllerutil.OperationResult
		assertRuntime   func(t *testing.T, cli client.Client)
		wantErr         bool
	}{
		{
			name:   "create secret",
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
						OwnerReferences: []metav1.OwnerReference{testutils.OwnerReference()},
						Labels:          applySeaweedFSLabels(nil, "s3", false),
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
				WithScheme(testutils.Scheme(t)).
				WithRuntimeObjects(tt.initRuntimeObjs...).
				Build()

			_, gotOp, err := ensureSeaweedfsS3Secret(context.Background(), testutils.Installation(), cli)
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
						OwnerReferences: []metav1.OwnerReference{testutils.OwnerReference()},
						Labels:          applyRegistryLabels(nil, "registry"),
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
				WithScheme(testutils.Scheme(t)).
				WithRuntimeObjects(tt.initRuntimeObjs...).
				Build()

			gotOp, err := ensureRegistryS3Secret(context.Background(), testutils.Installation(), cli, tt.args.sfsConfig)
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
