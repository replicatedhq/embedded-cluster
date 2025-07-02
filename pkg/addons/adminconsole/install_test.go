package adminconsole

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
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
	"k8s.io/client-go/metadata"
	metadatafake "k8s.io/client-go/metadata/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAdminConsole_ensureCAConfigmap(t *testing.T) {
	// Create a temporary file for testing CA bundle
	tempDir := t.TempDir()
	testCAPath := filepath.Join(tempDir, "test-ca.crt")
	err := os.WriteFile(testCAPath, []byte("new CA content"), 0644)
	require.NoError(t, err)

	metascheme := metadatafake.NewTestScheme()
	metav1.AddMetaToScheme(metascheme)

	newConfigMap := func(content string) *corev1.ConfigMap {
		hash := md5.Sum([]byte(content))
		checksum := hex.EncodeToString(hash[:])
		return &corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      privateCASConfigMapName,
				Namespace: _namespace,
				Annotations: map[string]string{
					"replicated.com/cas-checksum": checksum,
				},
			},
			Data: map[string]string{
				"ca_0.crt": content,
			},
		}
	}

	tests := []struct {
		name               string
		caPath             string
		initClients        func(t *testing.T) (client.Client, metadata.Interface)
		expectedErr        bool
		expectedErrMessage string
	}{
		{
			name:   "should return nil when caPath is not set",
			caPath: "",
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				kcli := clientfake.NewClientBuilder().Build()
				mcli := metadatafake.NewSimpleMetadataClient(metascheme)
				return kcli, mcli
			},
			expectedErr: false,
		},
		{
			name:   "should return nil when IsRequestEntityTooLargeError is returned from Create",
			caPath: testCAPath,
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				kcli := &mockClient{
					fake: clientfake.NewClientBuilder().Build(),
					createFunc: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
						return &k8serrors.StatusError{
							ErrStatus: metav1.Status{
								Status:  metav1.StatusFailure,
								Message: "Request entity too large",
								Reason:  metav1.StatusReasonRequestEntityTooLarge,
								Code:    413,
							},
						}
					},
				}
				mcli := metadatafake.NewSimpleMetadataClient(metascheme)
				return kcli, mcli
			},
			expectedErr: false,
		},
		{
			name:   "should return nil when IsRequestEntityTooLargeError is returned from Patch",
			caPath: testCAPath,
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				cm := newConfigMap("old CA content")
				kcli := &mockClient{
					fake: clientfake.NewClientBuilder().WithObjects(cm).Build(),
					patchFunc: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						return &k8serrors.StatusError{
							ErrStatus: metav1.Status{
								Status:  metav1.StatusFailure,
								Message: "Request entity too large",
								Reason:  metav1.StatusReasonRequestEntityTooLarge,
								Code:    413,
							},
						}
					},
				}
				mcli := metadatafake.NewSimpleMetadataClient(metascheme,
					&metav1.PartialObjectMetadata{TypeMeta: cm.TypeMeta, ObjectMeta: cm.ObjectMeta})
				return kcli, mcli
			},
			expectedErr: false,
		},
		{
			name:   "should return nil when IsNotExist is returned from reading CA file",
			caPath: filepath.Join(tempDir, "non-existent.crt"),
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				kcli := clientfake.NewClientBuilder().Build()
				mcli := metadatafake.NewSimpleMetadataClient(metascheme)
				return kcli, mcli
			},
			expectedErr: false,
		},
		{
			name:   "should return error for other errors from Create",
			caPath: testCAPath,
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				kcli := &mockClient{
					fake: clientfake.NewClientBuilder().Build(),
					createFunc: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
						return errors.New("some other create error")
					},
				}
				mcli := metadatafake.NewSimpleMetadataClient(metascheme)
				return kcli, mcli
			},
			expectedErr:        true,
			expectedErrMessage: "some other create error",
		},
		{
			name:   "should return error for other errors from Patch",
			caPath: testCAPath,
			initClients: func(t *testing.T) (client.Client, metadata.Interface) {
				cm := newConfigMap("old CA content")
				kcli := &mockClient{
					fake: clientfake.NewClientBuilder().WithObjects(cm).Build(),
					patchFunc: func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
						return errors.New("some other patch error")
					},
				}
				mcli := metadatafake.NewSimpleMetadataClient(metascheme,
					&metav1.PartialObjectMetadata{TypeMeta: cm.TypeMeta, ObjectMeta: cm.ObjectMeta})
				return kcli, mcli
			},
			expectedErr:        true,
			expectedErrMessage: "some other patch error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup reconciler with mock client
			scheme := runtime.NewScheme()
			// Register core v1 types to the scheme
			err := corev1.AddToScheme(scheme)
			require.NoError(t, err)

			kcli, mcli := tt.initClients(t)

			// Run test
			addon := &AdminConsole{
				DataDir:          t.TempDir(),
				HostCABundlePath: tt.caPath,
			}
			err = addon.ensureCAConfigmap(t.Context(), t.Logf, kcli, mcli)

			// Check results
			if tt.expectedErr {
				require.Error(t, err)
				if tt.expectedErrMessage != "" {
					assert.Contains(t, err.Error(), tt.expectedErrMessage)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// mockClient implements client.Client interface with customizable behavior
type mockClient struct {
	fake       client.Client
	createFunc func(context.Context, client.Object, ...client.CreateOption) error
	updateFunc func(context.Context, client.Object, ...client.UpdateOption) error
	patchFunc  func(context.Context, client.Object, client.Patch, ...client.PatchOption) error
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
	if m.patchFunc != nil {
		return m.patchFunc(ctx, obj, patch, opts...)
	}
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
