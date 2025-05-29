package adminconsole

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAdminConsole_ensureCAConfigmap(t *testing.T) {
	// Create a temporary file for testing CA bundle
	tempDir := t.TempDir()
	testCAPath := filepath.Join(tempDir, "test-ca.crt")
	err := os.WriteFile(testCAPath, []byte("test CA content"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name               string
		caPath             string
		setupMockClient    func(base client.Client) client.Client
		expectedErr        bool
		expectedErrMessage string
	}{
		{
			name:   "should return nil when caPath is not set",
			caPath: "",
			setupMockClient: func(base client.Client) client.Client {
				return base
			},
			expectedErr: false,
		},
		{
			name:   "should return nil when IsRequestEntityTooLargeError is returned from Get",
			caPath: testCAPath,
			setupMockClient: func(base client.Client) client.Client {
				return &mockClient{
					fake: base,
					getFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if obj.GetObjectKind().GroupVersionKind().Kind == "ConfigMap" {
							return &k8serrors.StatusError{
								ErrStatus: metav1.Status{
									Status:  metav1.StatusFailure,
									Message: "Request entity too large",
									Reason:  metav1.StatusReasonRequestEntityTooLarge,
									Code:    413,
								},
							}
						}
						return base.Get(ctx, key, obj)
					},
				}
			},
			expectedErr: false,
		},
		{
			name:   "should return nil when IsRequestEntityTooLargeError is returned from Create",
			caPath: testCAPath,
			setupMockClient: func(base client.Client) client.Client {
				return &mockClient{
					fake: base,
					getFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if obj.GetObjectKind().GroupVersionKind().Kind == "ConfigMap" {
							return k8serrors.NewNotFound(schema.GroupResource{
								Group:    "",
								Resource: "configmaps",
							}, key.Name)
						}
						return base.Get(ctx, key, obj)
					},
					createFunc: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
						if obj.GetObjectKind().GroupVersionKind().Kind == "ConfigMap" {
							return &k8serrors.StatusError{
								ErrStatus: metav1.Status{
									Status:  metav1.StatusFailure,
									Message: "Request entity too large",
									Reason:  metav1.StatusReasonRequestEntityTooLarge,
									Code:    413,
								},
							}
						}
						return base.Create(ctx, obj, opts...)
					},
				}
			},
			expectedErr: false,
		},
		{
			name:   "should return nil when IsRequestEntityTooLargeError is returned from Update",
			caPath: testCAPath,
			setupMockClient: func(base client.Client) client.Client {
				return &mockClient{
					fake: base,
					updateFunc: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						if obj.GetObjectKind().GroupVersionKind().Kind == "ConfigMap" {
							return &k8serrors.StatusError{
								ErrStatus: metav1.Status{
									Status:  metav1.StatusFailure,
									Message: "Request entity too large",
									Reason:  metav1.StatusReasonRequestEntityTooLarge,
									Code:    413,
								},
							}
						}
						return base.Update(ctx, obj, opts...)
					},
				}
			},
			expectedErr: false,
		},
		{
			name:   "should return nil when IsNotExist is returned from reading CA file",
			caPath: filepath.Join(tempDir, "non-existent.crt"),
			setupMockClient: func(base client.Client) client.Client {
				return base
			},
			expectedErr: false,
		},
		{
			name:   "should return error for other errors from Get",
			caPath: testCAPath,
			setupMockClient: func(base client.Client) client.Client {
				return &mockClient{
					fake: base,
					getFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if obj.GetObjectKind().GroupVersionKind().Kind == "ConfigMap" {
							return errors.New("some other error")
						}
						return base.Get(ctx, key, obj)
					},
				}
			},
			expectedErr:        true,
			expectedErrMessage: "some other error",
		},
		{
			name:   "should return error for other errors from Create",
			caPath: testCAPath,
			setupMockClient: func(base client.Client) client.Client {
				return &mockClient{
					fake: base,
					getFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if obj.GetObjectKind().GroupVersionKind().Kind == "ConfigMap" {
							return k8serrors.NewNotFound(schema.GroupResource{
								Group:    "",
								Resource: "configmaps",
							}, key.Name)
						}
						return base.Get(ctx, key, obj)
					},
					createFunc: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
						if obj.GetObjectKind().GroupVersionKind().Kind == "ConfigMap" {
							return errors.New("some other create error")
						}
						return base.Create(ctx, obj, opts...)
					},
				}
			},
			expectedErr:        true,
			expectedErrMessage: "some other create error",
		},
		{
			name:   "should return error for other errors from Update",
			caPath: testCAPath,
			setupMockClient: func(base client.Client) client.Client {
				err := base.Create(context.Background(), &corev1.ConfigMap{
					TypeMeta: metav1.TypeMeta{
						Kind:       "ConfigMap",
						APIVersion: "v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kotsadm-private-cas",
						Namespace: "kotsadm",
					},
					Data: map[string]string{
						"ca_0.crt": "old CA content",
					},
				})
				require.NoError(t, err)
				return &mockClient{
					fake: base,
					updateFunc: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						if obj.GetObjectKind().GroupVersionKind().Kind == "ConfigMap" {
							return errors.New("some other update error")
						}
						return base.Update(ctx, obj, opts...)
					},
				}
			},
			expectedErr:        true,
			expectedErrMessage: "some other update error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup reconciler with mock client
			scheme := runtime.NewScheme()
			// Register core v1 types to the scheme
			err := corev1.AddToScheme(scheme)
			require.NoError(t, err)

			baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			kcli := tt.setupMockClient(baseClient)

			logf := func(format string, args ...interface{}) {}

			// Run test
			addon := &AdminConsole{
				HostCABundlePath: tt.caPath,
			}
			err = addon.ensureCAConfigmap(t.Context(), logf, kcli)

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
