package controllers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
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

func TestReconcileHostCABundle(t *testing.T) {
	// Create a temporary file for testing CA bundle
	tempDir := t.TempDir()
	testCAPath := filepath.Join(tempDir, "test-ca.crt")
	err := os.WriteFile(testCAPath, []byte("test CA content"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name               string
		setupEnv           func(t *testing.T)
		setupMockClient    func(base client.Client) client.Client
		expectedErr        bool
		expectedErrMessage string
	}{
		{
			name: "should return nil when PRIVATE_CA_BUNDLE_PATH is not set",
			setupEnv: func(t *testing.T) {
				t.Setenv("PRIVATE_CA_BUNDLE_PATH", "")
			},
			setupMockClient: func(base client.Client) client.Client {
				return base
			},
			expectedErr: false,
		},
		{
			name: "should return nil when IsRequestEntityTooLargeError is returned from Get",
			setupEnv: func(t *testing.T) {
				t.Setenv("PRIVATE_CA_BUNDLE_PATH", testCAPath)
			},
			setupMockClient: func(base client.Client) client.Client {
				return &mockClient{
					Client: base,
					getFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
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
			},
			expectedErr: false,
		},
		{
			name: "should return nil when IsRequestEntityTooLargeError is returned from Create",
			setupEnv: func(t *testing.T) {
				t.Setenv("PRIVATE_CA_BUNDLE_PATH", testCAPath)
			},
			setupMockClient: func(base client.Client) client.Client {
				return &mockClient{
					Client: base,
					getFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						return k8serrors.NewNotFound(schema.GroupResource{
							Group:    "",
							Resource: "configmaps",
						}, key.Name)
					},
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
			},
			expectedErr: false,
		},
		{
			name: "should return nil when IsRequestEntityTooLargeError is returned from Update",
			setupEnv: func(t *testing.T) {
				t.Setenv("PRIVATE_CA_BUNDLE_PATH", testCAPath)
			},
			setupMockClient: func(base client.Client) client.Client {
				return &mockClient{
					Client: base,
					updateFunc: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
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
			},
			expectedErr: false,
		},
		{
			name: "should return nil when IsNotExist is returned from reading CA file",
			setupEnv: func(t *testing.T) {
				// Set a path that doesn't exist
				t.Setenv("PRIVATE_CA_BUNDLE_PATH", filepath.Join(tempDir, "non-existent.crt"))
			},
			setupMockClient: func(base client.Client) client.Client {
				return base
			},
			expectedErr: false,
		},
		{
			name: "should return error for other errors from Get",
			setupEnv: func(t *testing.T) {
				t.Setenv("PRIVATE_CA_BUNDLE_PATH", testCAPath)
			},
			setupMockClient: func(base client.Client) client.Client {
				return &mockClient{
					Client: base,
					getFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						return errors.New("some other error")
					},
				}
			},
			expectedErr:        true,
			expectedErrMessage: "some other error",
		},
		{
			name: "should return error for other errors from Create",
			setupEnv: func(t *testing.T) {
				t.Setenv("PRIVATE_CA_BUNDLE_PATH", testCAPath)
			},
			setupMockClient: func(base client.Client) client.Client {
				return &mockClient{
					Client: base,
					getFunc: func(ctx context.Context, key client.ObjectKey, obj client.Object) error {
						return k8serrors.NewNotFound(schema.GroupResource{
							Group:    "",
							Resource: "configmaps",
						}, key.Name)
					},
					createFunc: func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
						return errors.New("some other create error")
					},
				}
			},
			expectedErr:        true,
			expectedErrMessage: "some other create error",
		},
		{
			name: "should return error for other errors from Update",
			setupEnv: func(t *testing.T) {
				t.Setenv("PRIVATE_CA_BUNDLE_PATH", testCAPath)
			},
			setupMockClient: func(base client.Client) client.Client {
				err := base.Create(context.Background(), &corev1.ConfigMap{
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
					Client: base,
					updateFunc: func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
						return errors.New("some other update error")
					},
				}
			},
			expectedErr:        true,
			expectedErrMessage: "some other update error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			if tt.setupEnv != nil {
				tt.setupEnv(t)
			}

			// Setup reconciler with mock client
			scheme := runtime.NewScheme()
			// Register core v1 types to the scheme
			err := corev1.AddToScheme(scheme)
			require.NoError(t, err)

			baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			reconciler := &InstallationReconciler{
				Client: tt.setupMockClient(baseClient),
			}

			// Create a mock logger
			logger := testr.NewWithOptions(t, testr.Options{Verbosity: 1})

			// Setup context with logger
			ctx := logr.NewContext(context.Background(), logger)

			// Run test
			err = reconciler.ReconcileHostCABundle(ctx)

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

// mockClient implements a custom client.Client for testing
type mockClient struct {
	client.Client
	getFunc    func(ctx context.Context, key client.ObjectKey, obj client.Object) error
	createFunc func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	updateFunc func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
}

func (m *mockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if m.getFunc != nil {
		return m.getFunc(ctx, key, obj)
	}
	return m.Client.Get(ctx, key, obj, opts...)
}

func (m *mockClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, obj, opts...)
	}
	return m.Client.Create(ctx, obj, opts...)
}

func (m *mockClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, obj, opts...)
	}
	return m.Client.Update(ctx, obj, opts...)
}
