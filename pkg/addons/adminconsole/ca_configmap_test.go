package adminconsole

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureCAConfigmap(t *testing.T) {
	tests := []struct {
		name               string
		kcli               client.Client
		setup              func(t *testing.T)
		existingConfigMaps []runtime.Object
		wantErr            bool
		assert             func(t *testing.T, client client.Client)
	}{
		{
			name: "empty CA path should do nothing",
			kcli: fake.NewClientBuilder().Build(),
			setup: func(t *testing.T) {
				// No setup needed for this test
			},
			wantErr: false,
			assert: func(t *testing.T, c client.Client) {
				cm := &corev1.ConfigMap{}
				err := c.Get(context.Background(), client.ObjectKey{
					Namespace: namespace,
					Name:      privateCASConfigMapName,
				}, cm)
				assert.True(t, k8serrors.IsNotFound(err), "ConfigMap should not exist")
			},
		},
		{
			name: "should create configmap when it doesn't exist",
			kcli: fake.NewClientBuilder().Build(),
			setup: func(t *testing.T) {
				cafile := filepath.Join(t.TempDir(), "ca.crt")
				err := os.WriteFile(cafile, []byte("test-ca-content"), 0644)
				require.NoError(t, err)
				t.Setenv("PRIVATE_CA_BUNDLE_PATH", cafile)
			},
			wantErr: false,
			assert: func(t *testing.T, c client.Client) {
				cm := &corev1.ConfigMap{}
				err := c.Get(context.Background(), client.ObjectKey{
					Namespace: namespace,
					Name:      privateCASConfigMapName,
				}, cm)
				require.NoError(t, err)
				assert.Equal(t, "test-ca-content", cm.Data["ca_0.crt"])
			},
		},
		{
			name: "should update configmap when it exists with different content",
			kcli: fake.NewClientBuilder().WithObjects(&corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      privateCASConfigMapName,
					Namespace: namespace,
				},
				Data: map[string]string{
					"ca_0.crt": "old-ca-content",
				},
			}).Build(),
			setup: func(t *testing.T) {
				cafile := filepath.Join(t.TempDir(), "ca.crt")
				err := os.WriteFile(cafile, []byte("new-ca-content"), 0644)
				require.NoError(t, err)
				t.Setenv("PRIVATE_CA_BUNDLE_PATH", cafile)
			},
			wantErr: false,
			assert: func(t *testing.T, c client.Client) {
				cm := &corev1.ConfigMap{}
				err := c.Get(context.Background(), client.ObjectKey{
					Namespace: namespace,
					Name:      privateCASConfigMapName,
				}, cm)
				require.NoError(t, err)
				assert.Equal(t, "new-ca-content", cm.Data["ca_0.crt"])
			},
		},
		{
			name: "should not update configmap when content is the same",
			kcli: fake.NewClientBuilder().WithObjects(&corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      privateCASConfigMapName,
					Namespace: namespace,
				},
				Data: map[string]string{
					"ca_0.crt": "same-ca-content",
				},
			}).Build(),
			setup: func(t *testing.T) {
				cafile := filepath.Join(t.TempDir(), "ca.crt")
				err := os.WriteFile(cafile, []byte("same-ca-content"), 0644)
				require.NoError(t, err)
				t.Setenv("PRIVATE_CA_BUNDLE_PATH", cafile)
			},
			wantErr: false,
			assert: func(t *testing.T, c client.Client) {
				cm := &corev1.ConfigMap{}
				err := c.Get(context.Background(), client.ObjectKey{
					Namespace: namespace,
					Name:      privateCASConfigMapName,
				}, cm)
				require.NoError(t, err)
				assert.Equal(t, "same-ca-content", cm.Data["ca_0.crt"])
			},
		},
		{
			name: "should return error when CA file doesn't exist",
			kcli: fake.NewClientBuilder().Build(),
			setup: func(t *testing.T) {
				t.Setenv("PRIVATE_CA_BUNDLE_PATH", "/nonexistent/path/ca.crt")
			},
			wantErr: true,
			assert:  func(t *testing.T, c client.Client) {},
		},
		{
			name: "should return error when client create fails",
			kcli: &mockClient{
				fake: fake.NewClientBuilder().Build(),
				createFunc: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
					return &k8serrors.StatusError{
						ErrStatus: metav1.Status{
							Status:  metav1.StatusFailure,
							Message: "create error",
							Reason:  metav1.StatusReasonUnknown,
							Code:    500,
						},
					}
				},
			},
			setup: func(t *testing.T) {
				cafile := filepath.Join(t.TempDir(), "ca.crt")
				err := os.WriteFile(cafile, []byte("new-ca-content"), 0644)
				require.NoError(t, err)
				t.Setenv("PRIVATE_CA_BUNDLE_PATH", cafile)
			},
			wantErr: true,
			assert:  func(t *testing.T, c client.Client) {},
		},
		{
			name: "should return error when client update fails",
			kcli: &mockClient{
				fake: fake.NewClientBuilder().WithObjects(&corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      privateCASConfigMapName,
						Namespace: namespace,
					},
					Data: map[string]string{
						"ca_0.crt": "old-ca-content",
					},
				}).Build(),
				updateFunc: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
					return &k8serrors.StatusError{
						ErrStatus: metav1.Status{
							Status:  metav1.StatusFailure,
							Message: "update error",
							Reason:  metav1.StatusReasonUnknown,
							Code:    500,
						},
					}
				},
			},
			setup: func(t *testing.T) {
				cafile := filepath.Join(t.TempDir(), "ca.crt")
				err := os.WriteFile(cafile, []byte("new-ca-content"), 0644)
				require.NoError(t, err)
				t.Setenv("PRIVATE_CA_BUNDLE_PATH", cafile)
			},
			wantErr: true,
			assert:  func(t *testing.T, c client.Client) {},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t)
			}

			caPath := os.Getenv("PRIVATE_CA_BUNDLE_PATH")
			logf := func(format string, args ...any) {} // discard logs
			err := EnsureCAConfigmap(context.Background(), logf, tt.kcli, caPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureCAConfigmap() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.assert != nil {
				tt.assert(t, tt.kcli)
			}
		})
	}
}

// mockClient implements client.Client interface with customizable behavior
type mockClient struct {
	fake       client.Client
	createFunc func(context.Context, client.Object, ...client.CreateOption) error
	updateFunc func(context.Context, client.Object, ...client.UpdateOption) error
	getFunc    func(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error
}

func (m *mockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if m.getFunc != nil {
		return m.getFunc(ctx, key, obj, opts...)
	}
	return m.fake.Get(ctx, key, obj, opts...)
}

func (m *mockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return m.fake.List(ctx, list, opts...)
}

func (m *mockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, obj, opts...)
	}
	return m.fake.Create(ctx, obj, opts...)
}

func (m *mockClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return m.fake.Delete(ctx, obj, opts...)
}

func (m *mockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, obj, opts...)
	}
	return m.fake.Update(ctx, obj, opts...)
}

func (m *mockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return m.fake.Patch(ctx, obj, patch, opts...)
}

func (m *mockClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return m.fake.DeleteAllOf(ctx, obj, opts...)
}

func (m *mockClient) Status() client.StatusWriter {
	return m.fake.Status()
}

func (m *mockClient) Scheme() *runtime.Scheme {
	return m.fake.Scheme()
}

func (m *mockClient) RESTMapper() meta.RESTMapper {
	return m.fake.RESTMapper()
}

func (m *mockClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return m.fake.GroupVersionKindFor(obj)
}

func (m *mockClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return m.fake.IsObjectNamespaced(obj)
}

func (m *mockClient) SubResource(subResource string) client.SubResourceClient {
	return m.fake.SubResource(subResource)
}
