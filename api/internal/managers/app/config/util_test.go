package config

import (
	"context"
	"fmt"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	kyaml "sigs.k8s.io/yaml"
)

func Test_readConfigValuesFromKube(t *testing.T) {
	// Set ENABLE_V3=1 for all tests so KotsadmNamespace checks for the kotsadm namespace
	t.Setenv("ENABLE_V3", "1")

	tests := []struct {
		name         string
		setupManager func(t *testing.T) *appConfigManager
		validateFunc func(t *testing.T, manager *appConfigManager)
	}{
		{
			name: "kube client is nil - should warn and return empty",
			setupManager: func(t *testing.T) *appConfigManager {
				return &appConfigManager{
					kcli:   nil,
					logger: logger.NewDiscardLogger(),
					license: &kotsv1beta1.License{
						Spec: kotsv1beta1.LicenseSpec{
							AppSlug: "test-app",
						},
					},
				}
			},
			validateFunc: func(t *testing.T, manager *appConfigManager) {
				values, err := manager.readConfigValuesFromKube()
				require.NoError(t, err)
				assert.Empty(t, values)
			},
		},
		{
			name: "secret does not exist - should return empty config",
			setupManager: func(t *testing.T) *appConfigManager {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))

				// Create namespace for KotsadmNamespace to find
				namespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kotsadm",
					},
				}

				fakeKcli := clientfake.NewClientBuilder().
					WithScheme(sch).
					WithObjects(namespace).
					Build()

				return &appConfigManager{
					kcli:   fakeKcli,
					logger: logger.NewDiscardLogger(),
					license: &kotsv1beta1.License{
						Spec: kotsv1beta1.LicenseSpec{
							AppSlug: "test-app",
						},
					},
				}
			},
			validateFunc: func(t *testing.T, manager *appConfigManager) {
				values, err := manager.readConfigValuesFromKube()
				require.NoError(t, err)
				assert.Empty(t, values)
			},
		},
		{
			name: "secret exists with valid config values - should read and return",
			setupManager: func(t *testing.T) *appConfigManager {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))

				// Create namespace
				namespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kotsadm",
					},
				}

				// Create config values
				configValues := &kotsv1beta1.ConfigValues{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "kots.io/v1beta1",
						Kind:       "ConfigValues",
					},
					Spec: kotsv1beta1.ConfigValuesSpec{
						Values: map[string]kotsv1beta1.ConfigValue{
							"hostname": {
								Value:   "example.com",
								Default: "localhost",
							},
							"port": {
								Value:   "8080",
								Default: "80",
							},
						},
					},
				}

				configValuesData, err := kyaml.Marshal(configValues)
				require.NoError(t, err)

				// Create secret with config values
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      utils.GetConfigValuesSecretName("test-app"),
						Namespace: "kotsadm",
					},
					Data: map[string][]byte{
						"config-values.yaml": configValuesData,
					},
				}

				fakeKcli := clientfake.NewClientBuilder().
					WithScheme(sch).
					WithObjects(namespace, secret).
					Build()

				return &appConfigManager{
					kcli:   fakeKcli,
					logger: logger.NewDiscardLogger(),
					license: &kotsv1beta1.License{
						Spec: kotsv1beta1.LicenseSpec{
							AppSlug: "test-app",
						},
					},
				}
			},
			validateFunc: func(t *testing.T, manager *appConfigManager) {
				values, err := manager.readConfigValuesFromKube()
				require.NoError(t, err)
				assert.Len(t, values, 2)

				assert.Contains(t, values, "hostname")
				assert.Equal(t, "example.com", values["hostname"].Value)
				assert.Equal(t, "localhost", values["hostname"].Default)

				assert.Contains(t, values, "port")
				assert.Equal(t, "8080", values["port"].Value)
				assert.Equal(t, "80", values["port"].Default)
			},
		},
		{
			name: "secret exists with password config values - should use ValuePlaintext",
			setupManager: func(t *testing.T) *appConfigManager {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))

				// Create namespace
				namespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kotsadm",
					},
				}

				// Create config values with password (uses ValuePlaintext)
				configValues := &kotsv1beta1.ConfigValues{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "kots.io/v1beta1",
						Kind:       "ConfigValues",
					},
					Spec: kotsv1beta1.ConfigValuesSpec{
						Values: map[string]kotsv1beta1.ConfigValue{
							"admin_password": {
								ValuePlaintext: "supersecret",
								Default:        "",
							},
						},
					},
				}

				configValuesData, err := kyaml.Marshal(configValues)
				require.NoError(t, err)

				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      utils.GetConfigValuesSecretName("test-app"),
						Namespace: "kotsadm",
					},
					Data: map[string][]byte{
						"config-values.yaml": configValuesData,
					},
				}

				fakeKcli := clientfake.NewClientBuilder().
					WithScheme(sch).
					WithObjects(namespace, secret).
					Build()

				return &appConfigManager{
					kcli:   fakeKcli,
					logger: logger.NewDiscardLogger(),
					license: &kotsv1beta1.License{
						Spec: kotsv1beta1.LicenseSpec{
							AppSlug: "test-app",
						},
					},
				}
			},
			validateFunc: func(t *testing.T, manager *appConfigManager) {
				values, err := manager.readConfigValuesFromKube()
				require.NoError(t, err)
				assert.Len(t, values, 1)

				assert.Contains(t, values, "admin_password")
				assert.Equal(t, "supersecret", values["admin_password"].Value)
			},
		},
		{
			name: "secret exists but config-values.yaml key is missing - should return error",
			setupManager: func(t *testing.T) *appConfigManager {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))

				// Create namespace
				namespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kotsadm",
					},
				}

				// Create secret without config-values.yaml key
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      utils.GetConfigValuesSecretName("test-app"),
						Namespace: "kotsadm",
					},
					Data: map[string][]byte{
						"wrong-key": []byte("data"),
					},
				}

				fakeKcli := clientfake.NewClientBuilder().
					WithScheme(sch).
					WithObjects(namespace, secret).
					Build()

				return &appConfigManager{
					kcli:   fakeKcli,
					logger: logger.NewDiscardLogger(),
					license: &kotsv1beta1.License{
						Spec: kotsv1beta1.LicenseSpec{
							AppSlug: "test-app",
						},
					},
				}
			},
			validateFunc: func(t *testing.T, manager *appConfigManager) {
				values, err := manager.readConfigValuesFromKube()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "config-values.yaml")
				assert.Nil(t, values)
			},
		},
		{
			name: "secret exists but config data is invalid YAML - should return error",
			setupManager: func(t *testing.T) *appConfigManager {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))

				// Create namespace
				namespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kotsadm",
					},
				}

				// Create secret with invalid YAML
				secret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      utils.GetConfigValuesSecretName("test-app"),
						Namespace: "kotsadm",
					},
					Data: map[string][]byte{
						"config-values.yaml": []byte("invalid: yaml: data: ["),
					},
				}

				fakeKcli := clientfake.NewClientBuilder().
					WithScheme(sch).
					WithObjects(namespace, secret).
					Build()

				return &appConfigManager{
					kcli:   fakeKcli,
					logger: logger.NewDiscardLogger(),
					license: &kotsv1beta1.License{
						Spec: kotsv1beta1.LicenseSpec{
							AppSlug: "test-app",
						},
					},
				}
			},
			validateFunc: func(t *testing.T, manager *appConfigManager) {
				values, err := manager.readConfigValuesFromKube()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to unmarshal")
				assert.Nil(t, values)
			},
		},
		{
			name: "get secret returns non-NotFound error - should return error",
			setupManager: func(t *testing.T) *appConfigManager {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))

				// Create namespace
				namespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kotsadm",
					},
				}

				// Create interceptor that returns error for secret Get
				fakeKcli := clientfake.NewClientBuilder().
					WithScheme(sch).
					WithObjects(namespace).
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
							if key.Name == utils.GetConfigValuesSecretName("test-app") {
								return fmt.Errorf("simulated get error")
							}
							return c.Get(ctx, key, obj, opts...)
						},
					}).
					Build()

				return &appConfigManager{
					kcli:   fakeKcli,
					logger: logger.NewDiscardLogger(),
					license: &kotsv1beta1.License{
						Spec: kotsv1beta1.LicenseSpec{
							AppSlug: "test-app",
						},
					},
				}
			},
			validateFunc: func(t *testing.T, manager *appConfigManager) {
				values, err := manager.readConfigValuesFromKube()
				require.Error(t, err)
				assert.Nil(t, values)
			},
		},
		{
			name: "KotsadmNamespace returns error - should return error",
			setupManager: func(t *testing.T) *appConfigManager {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))

				// Create interceptor that returns error for namespace Get
				fakeKcli := clientfake.NewClientBuilder().
					WithScheme(sch).
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
							if key.Name == "kotsadm" {
								return fmt.Errorf("simulated namespace error")
							}
							return c.Get(ctx, key, obj, opts...)
						},
					}).
					Build()

				return &appConfigManager{
					kcli:   fakeKcli,
					logger: logger.NewDiscardLogger(),
					license: &kotsv1beta1.License{
						Spec: kotsv1beta1.LicenseSpec{
							AppSlug: "test-app",
						},
					},
				}
			},
			validateFunc: func(t *testing.T, manager *appConfigManager) {
				values, err := manager.readConfigValuesFromKube()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "get kotsadm namespace:")
				assert.Nil(t, values)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := tt.setupManager(t)
			tt.validateFunc(t, manager)
		})
	}
}
