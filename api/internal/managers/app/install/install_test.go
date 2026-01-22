package install

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	kotscli "github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestAppInstallManager_Install(t *testing.T) {
	// Setup environment variable for V3
	t.Setenv("ENABLE_V3", "1")

	// Create test license with proper Kubernetes resource format
	licenseYAML := `apiVersion: kots.io/v1beta1
kind: License
spec:
  appSlug: test-app
`
	licenseBytes := []byte(licenseYAML)

	// Create test release data
	releaseData := &release.ReleaseData{
		ChannelRelease: &release.ChannelRelease{
			DefaultDomains: release.Domains{
				ReplicatedAppDomain: "replicated.app",
			},
		},
	}

	// Set up release data globally so AppSlug() returns the correct value for v3
	err := release.SetReleaseDataForTests(map[string][]byte{
		"channelrelease.yaml": []byte("# channel release object\nappSlug: test-app"),
	})
	require.NoError(t, err)

	t.Run("Config values should be passed to the installer", func(t *testing.T) {
		configValues := kotsv1beta1.ConfigValues{
			Spec: kotsv1beta1.ConfigValuesSpec{
				Values: map[string]kotsv1beta1.ConfigValue{
					"key1": {
						Value: "value1",
					},
					"key2": {
						Value: "value2",
					},
				},
			},
		}

		// Create mock installer with detailed verification
		mockKotsCLI := &kotscli.MockKotsCLI{}
		mockKotsCLI.On("Install", mock.MatchedBy(func(opts kotscli.InstallOptions) bool {
			// Verify basic install options
			if opts.AppSlug != "test-app" {
				t.Logf("AppSlug mismatch: expected 'test-app', got '%s'", opts.AppSlug)
				return false
			}
			if opts.License == nil {
				t.Logf("License is nil")
				return false
			}
			if opts.Namespace != "test-app" {
				t.Logf("Namespace mismatch: expected 'test-app', got '%s'", opts.Namespace)
				return false
			}
			if opts.ClusterID != "test-cluster" {
				t.Logf("ClusterID mismatch: expected 'test-cluster', got '%s'", opts.ClusterID)
				return false
			}
			if opts.AirgapBundle != "test-airgap.tar.gz" {
				t.Logf("AirgapBundle mismatch: expected 'test-airgap.tar.gz', got '%s'", opts.AirgapBundle)
				return false
			}
			if opts.ReplicatedAppEndpoint == "" {
				t.Logf("ReplicatedAppEndpoint is empty")
				return false
			}
			if opts.ConfigValuesFile == "" {
				t.Logf("ConfigValuesFile is empty")
				return false
			}
			if !opts.DisableImagePush {
				t.Logf("DisableImagePush is false")
				return false
			}

			// Verify config values file content
			b, err := os.ReadFile(opts.ConfigValuesFile)
			if err != nil {
				t.Logf("Failed to read config values file: %v", err)
				return false
			}
			var cv kotsv1beta1.ConfigValues
			if err := kyaml.Unmarshal(b, &cv); err != nil {
				t.Logf("Failed to unmarshal config values: %v", err)
				return false
			}
			if cv.Spec.Values["key1"].Value != "value1" {
				t.Logf("Config value key1 mismatch: expected 'value1', got '%s'", cv.Spec.Values["key1"].Value)
				return false
			}
			if cv.Spec.Values["key2"].Value != "value2" {
				t.Logf("Config value key2 mismatch: expected 'value2', got '%s'", cv.Spec.Values["key2"].Value)
				return false
			}
			return true
		})).Return(nil)

		// Create fake kube client
		sch := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(sch))
		require.NoError(t, scheme.AddToScheme(sch))
		fakeKcli := clientfake.NewClientBuilder().WithScheme(sch).Build()

		// Create manager
		manager, err := NewAppInstallManager(
			WithLicense(licenseBytes),
			WithClusterID("test-cluster"),
			WithAirgapBundle("test-airgap.tar.gz"),
			WithReleaseData(releaseData),
			WithKotsCLI(mockKotsCLI),
			WithLogger(logger.NewDiscardLogger()),
			WithKubeClient(fakeKcli),
		)
		require.NoError(t, err)

		// Run installation
		err = manager.Install(context.Background(), configValues)
		require.NoError(t, err)

		// Verify mock was called
		mockKotsCLI.AssertExpectations(t)
	})
}

func TestAppInstallManager_createConfigValuesFile(t *testing.T) {
	manager := &appInstallManager{}

	configValues := kotsv1beta1.ConfigValues{
		Spec: kotsv1beta1.ConfigValuesSpec{
			Values: map[string]kotsv1beta1.ConfigValue{
				"testKey": {
					Value: "testValue",
				},
			},
		},
	}

	filename, err := manager.createConfigValuesFile(configValues)
	assert.NoError(t, err)
	assert.NotEmpty(t, filename)

	// Verify file exists and contains correct content
	data, err := os.ReadFile(filename)
	assert.NoError(t, err)

	var unmarshaled kotsv1beta1.ConfigValues
	err = kyaml.Unmarshal(data, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, "testValue", unmarshaled.Spec.Values["testKey"].Value)

	// Clean up
	os.Remove(filename)
}

func TestAppInstallManager_Install_ConfigValuesSecret(t *testing.T) {
	// Set up environment and release data for all tests
	t.Setenv("ENABLE_V3", "1")
	err := release.SetReleaseDataForTests(map[string][]byte{
		"channelrelease.yaml": []byte("# channel release object\nappSlug: test-app"),
	})
	require.NoError(t, err)

	tests := []struct {
		name                  string
		releaseData           *release.ReleaseData
		configValues          kotsv1beta1.ConfigValues
		setupClient           func(t *testing.T) client.Client
		expectError           bool
		expectedErrorContains string
		validateSecret        func(t *testing.T, kcli client.Client)
		validateKotsCalled    bool
	}{
		{
			name: "first install creates secret with multiple config values",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					VersionLabel: "v1.0.0",
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.app",
					},
				},
			},
			configValues: kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"key1": {Value: "value1"},
						"key2": {Value: "value2"},
						"key3": {Value: "value3"},
					},
				},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))
				return clientfake.NewClientBuilder().WithScheme(sch).Build()
			},
			expectError: false,
			validateSecret: func(t *testing.T, kcli client.Client) {
				// Get and verify secret
				secret := &corev1.Secret{}
				err := kcli.Get(context.Background(), client.ObjectKey{
					Name:      "test-app-config-values",
					Namespace: "test-app",
				}, secret)
				require.NoError(t, err)

				// Verify labels
				assert.Equal(t, "test-app", secret.Labels["app.kubernetes.io/name"])
				assert.Equal(t, "v1.0.0", secret.Labels["app.kubernetes.io/version"])
				assert.Equal(t, "config", secret.Labels["app.kubernetes.io/component"])
				assert.Equal(t, "embedded-cluster", secret.Labels["app.kubernetes.io/part-of"])
				assert.Equal(t, "embedded-cluster-installer", secret.Labels["app.kubernetes.io/managed-by"])

				// Verify type
				assert.Equal(t, corev1.SecretTypeOpaque, secret.Type)

				// Verify data
				data, ok := secret.Data["config-values.yaml"]
				require.True(t, ok)

				// Unmarshal and verify values
				var cv kotsv1beta1.ConfigValues
				err = kyaml.Unmarshal(data, &cv)
				require.NoError(t, err)
				assert.Equal(t, "value1", cv.Spec.Values["key1"].Value)
				assert.Equal(t, "value2", cv.Spec.Values["key2"].Value)
				assert.Equal(t, "value3", cv.Spec.Values["key3"].Value)
			},
			validateKotsCalled: true,
		},
		{
			name: "existing secret is fetched and updated",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					VersionLabel: "v1.0.0",
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.app",
					},
				},
			},
			configValues: kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"newkey": {Value: "newvalue"},
					},
				},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))

				// Create existing secret with old version
				existingSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app-config-values",
						Namespace: "test-app",
						Labels: map[string]string{
							"app.kubernetes.io/version": "v0.9.0",
						},
					},
					Data: map[string][]byte{
						"config-values.yaml": []byte("old: data"),
					},
				}

				return clientfake.NewClientBuilder().
					WithScheme(sch).
					WithObjects(existingSecret).
					Build()
			},
			expectError: false,
			validateSecret: func(t *testing.T, kcli client.Client) {
				// Get and verify secret was recreated
				secret := &corev1.Secret{}
				err := kcli.Get(context.Background(), client.ObjectKey{
					Name:      "test-app-config-values",
					Namespace: "test-app",
				}, secret)
				require.NoError(t, err)

				// Verify updated version label
				assert.Equal(t, "v1.0.0", secret.Labels["app.kubernetes.io/version"])

				// Verify new data
				data, ok := secret.Data["config-values.yaml"]
				require.True(t, ok)

				var cv kotsv1beta1.ConfigValues
				err = kyaml.Unmarshal(data, &cv)
				require.NoError(t, err)
				assert.Equal(t, "newvalue", cv.Spec.Values["newkey"].Value)
			},
			validateKotsCalled: true,
		},
		{
			name:        "fails when release data is missing",
			releaseData: nil,
			configValues: kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"key1": {Value: "value1"},
					},
				},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))
				return clientfake.NewClientBuilder().WithScheme(sch).Build()
			},
			expectError:           true,
			expectedErrorContains: "release data is required",
			validateKotsCalled:    false,
		},
		{
			name: "fails when channel release is missing",
			releaseData: &release.ReleaseData{
				ChannelRelease: nil,
			},
			configValues: kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"key1": {Value: "value1"},
					},
				},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))
				return clientfake.NewClientBuilder().WithScheme(sch).Build()
			},
			expectError:           true,
			expectedErrorContains: "release data is required",
			validateKotsCalled:    false,
		},
		{
			name: "fails when get returns error",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					VersionLabel: "v1.0.0",
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.app",
					},
				},
			},
			configValues: kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"key1": {Value: "value1"},
					},
				},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))

				// Create existing secret
				existingSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app-config-values",
						Namespace: "test-app",
					},
				}

				return clientfake.NewClientBuilder().
					WithScheme(sch).
					WithObjects(existingSecret).
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
							if key.Name == "test-app-config-values" {
								return fmt.Errorf("simulated get error")
							}
							return c.Get(ctx, key, obj, opts...)
						},
					}).
					Build()
			},
			expectError:           true,
			expectedErrorContains: "get existing config values secret",
			validateKotsCalled:    false,
		},
		{
			name: "fails when update returns error",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					VersionLabel: "v1.0.0",
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.app",
					},
				},
			},
			configValues: kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"key1": {Value: "value1"},
					},
				},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))

				// Create existing secret
				existingSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app-config-values",
						Namespace: "test-app",
					},
				}

				return clientfake.NewClientBuilder().
					WithScheme(sch).
					WithObjects(existingSecret).
					WithInterceptorFuncs(interceptor.Funcs{
						Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
							if secret, ok := obj.(*corev1.Secret); ok && secret.Name == "test-app-config-values" {
								return fmt.Errorf("simulated update error")
							}
							return c.Update(ctx, obj, opts...)
						},
					}).
					Build()
			},
			expectError:           true,
			expectedErrorContains: "update config values secret",
			validateKotsCalled:    false,
		},
		{
			name: "fails when create returns non-AlreadyExists error",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					VersionLabel: "v1.0.0",
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.app",
					},
				},
			},
			configValues: kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"key1": {Value: "value1"},
					},
				},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))

				return clientfake.NewClientBuilder().
					WithScheme(sch).
					WithInterceptorFuncs(interceptor.Funcs{
						Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
							if _, ok := obj.(*corev1.Secret); ok {
								return fmt.Errorf("simulated create error")
							}
							return c.Create(ctx, obj, opts...)
						},
					}).
					Build()
			},
			expectError:           true,
			expectedErrorContains: "create config values secret",
			validateKotsCalled:    false,
		},
		{
			name: "handles empty config values",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					VersionLabel: "v1.0.0",
					DefaultDomains: release.Domains{
						ReplicatedAppDomain: "replicated.app",
					},
				},
			},
			configValues: kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{},
				},
			},
			setupClient: func(t *testing.T) client.Client {
				sch := runtime.NewScheme()
				require.NoError(t, corev1.AddToScheme(sch))
				require.NoError(t, scheme.AddToScheme(sch))
				return clientfake.NewClientBuilder().WithScheme(sch).Build()
			},
			expectError: false,
			validateSecret: func(t *testing.T, kcli client.Client) {
				// Get and verify secret was created even with empty values
				secret := &corev1.Secret{}
				err := kcli.Get(context.Background(), client.ObjectKey{
					Name:      "test-app-config-values",
					Namespace: "test-app",
				}, secret)
				require.NoError(t, err)

				// Verify data exists
				data, ok := secret.Data["config-values.yaml"]
				require.True(t, ok)
				require.NotEmpty(t, data)
			},
			validateKotsCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			license := &kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{AppSlug: "test-app"},
			}
			licenseBytes, err := kyaml.Marshal(license)
			require.NoError(t, err)

			mockKotsCLI := &kotscli.MockKotsCLI{}
			if tt.validateKotsCalled {
				mockKotsCLI.On("Install", mock.Anything).Return(nil)
			}

			kcli := tt.setupClient(t)

			manager, err := NewAppInstallManager(
				WithLicense(licenseBytes),
				WithClusterID("test-cluster"),
				WithAirgapBundle("test-airgap.tar.gz"),
				WithReleaseData(tt.releaseData),
				WithKotsCLI(mockKotsCLI),
				WithLogger(logger.NewDiscardLogger()),
				WithKubeClient(kcli),
			)
			require.NoError(t, err)

			// Execute
			err = manager.Install(context.Background(), tt.configValues)

			// Verify
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorContains)
			} else {
				require.NoError(t, err)
				if tt.validateSecret != nil {
					tt.validateSecret(t, kcli)
				}
			}

			if tt.validateKotsCalled {
				mockKotsCLI.AssertExpectations(t)
			} else {
				mockKotsCLI.AssertNotCalled(t, "Install")
			}
		})
	}
}
