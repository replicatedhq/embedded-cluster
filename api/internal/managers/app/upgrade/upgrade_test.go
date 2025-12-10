package appupgrademanager

import (
	"context"
	"fmt"
	"testing"

	appupgradestore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/upgrade"
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

func TestAppUpgradeManager_Upgrade(t *testing.T) {
	// Setup environment variable for V3
	t.Setenv("ENABLE_V3", "1")

	// Create test release data
	releaseData := &release.ReleaseData{
		ChannelRelease: &release.ChannelRelease{
			AppSlug:         "test-app",
			VersionLabel:    "v1.0.0",
			ChannelID:       "channel-123",
			ChannelSequence: 456,
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

	t.Run("Successful online upgrade", func(t *testing.T) {
		configValues := kotsv1beta1.ConfigValues{
			Spec: kotsv1beta1.ConfigValuesSpec{
				Values: map[string]kotsv1beta1.ConfigValue{
					"key1": {
						Value: "value1",
					},
				},
			},
		}

		// Create mock deployer for online deployment (no airgap bundle or license)
		mockKotsCLI := &kotscli.MockKotsCLI{}
		mockKotsCLI.On("Deploy", mock.MatchedBy(func(opts kotscli.DeployOptions) bool {
			return opts.AppSlug == "test-app" &&
				opts.Namespace == "test-app" &&
				opts.ClusterID == "test-cluster" &&
				opts.AirgapBundle == "" && // No airgap bundle for online
				len(opts.License) == 0 && // No license for online
				opts.ChannelID == "channel-123" &&
				opts.ChannelSequence == 456 &&
				opts.SkipPreflights == true
		})).Return(nil)

		// Create fake kube client
		sch := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(sch))
		require.NoError(t, scheme.AddToScheme(sch))
		fakeKcli := clientfake.NewClientBuilder().WithScheme(sch).Build()

		// Create manager for online deployment (no license or airgap bundle)
		store := appupgradestore.NewMemoryStore()
		manager, err := NewAppUpgradeManager(
			WithLogger(logger.NewDiscardLogger()),
			WithAppUpgradeStore(store),
			WithReleaseData(releaseData),
			WithClusterID("test-cluster"),
			WithKotsCLI(mockKotsCLI),
			WithKubeClient(fakeKcli),
		)
		require.NoError(t, err)

		// Execute upgrade
		err = manager.Upgrade(context.Background(), configValues)
		require.NoError(t, err)

		// Verify mock was called
		mockKotsCLI.AssertExpectations(t)
	})

	t.Run("Failed upgrade should set failed status", func(t *testing.T) {
		configValues := kotsv1beta1.ConfigValues{}

		// Create mock deployer that fails
		mockKotsCLI := &kotscli.MockKotsCLI{}
		mockKotsCLI.On("Deploy", mock.Anything).Return(assert.AnError)

		// Create fake kube client
		sch := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(sch))
		require.NoError(t, scheme.AddToScheme(sch))
		fakeKcli := clientfake.NewClientBuilder().WithScheme(sch).Build()

		// Create manager (online deployment - no license or airgap bundle)
		store := appupgradestore.NewMemoryStore()
		manager, err := NewAppUpgradeManager(
			WithLogger(logger.NewDiscardLogger()),
			WithAppUpgradeStore(store),
			WithReleaseData(releaseData),
			WithClusterID("test-cluster"),
			WithKotsCLI(mockKotsCLI),
			WithKubeClient(fakeKcli),
		)
		require.NoError(t, err)

		// Execute upgrade
		err = manager.Upgrade(context.Background(), configValues)
		require.Error(t, err)

		// Verify mock was called
		mockKotsCLI.AssertExpectations(t)
	})

	t.Run("Successful airgap upgrade", func(t *testing.T) {
		configValues := kotsv1beta1.ConfigValues{}

		// Create mock deployer for airgap deployment
		mockKotsCLI := &kotscli.MockKotsCLI{}
		mockKotsCLI.On("Deploy", mock.MatchedBy(func(opts kotscli.DeployOptions) bool {
			return opts.AppSlug == "test-app" &&
				opts.Namespace == "test-app" &&
				opts.ClusterID == "test-cluster" &&
				opts.AirgapBundle == "airgap-bundle.tar.gz" &&
				len(opts.License) > 0 && // License provided for airgap
				opts.ChannelID == "channel-123" &&
				opts.ChannelSequence == 456 &&
				opts.SkipPreflights == true
		})).Return(nil)

		// Create fake kube client
		sch := runtime.NewScheme()
		require.NoError(t, corev1.AddToScheme(sch))
		require.NoError(t, scheme.AddToScheme(sch))
		fakeKcli := clientfake.NewClientBuilder().WithScheme(sch).Build()

		// Create valid license YAML for airgap
		license := &kotsv1beta1.License{
			Spec: kotsv1beta1.LicenseSpec{AppSlug: "test-app"},
		}
		licenseBytes, err := kyaml.Marshal(license)
		require.NoError(t, err)

		// Create manager with airgap bundle and license
		store := appupgradestore.NewMemoryStore()
		manager, err := NewAppUpgradeManager(
			WithLogger(logger.NewDiscardLogger()),
			WithAppUpgradeStore(store),
			WithReleaseData(releaseData),
			WithLicense(licenseBytes),
			WithClusterID("test-cluster"),
			WithAirgapBundle("airgap-bundle.tar.gz"),
			WithKotsCLI(mockKotsCLI),
			WithKubeClient(fakeKcli),
		)
		require.NoError(t, err)

		// Execute upgrade
		err = manager.Upgrade(context.Background(), configValues)
		require.NoError(t, err)

		// Verify mock was called with correct airgap bundle
		mockKotsCLI.AssertExpectations(t)
	})
}

func TestAppUpgradeManager_NewWithOptions(t *testing.T) {
	releaseData := &release.ReleaseData{
		ChannelRelease: &release.ChannelRelease{
			AppSlug: "test-app",
		},
	}

	store := appupgradestore.NewMemoryStore()

	manager, err := NewAppUpgradeManager(
		WithLogger(logger.NewDiscardLogger()),
		WithAppUpgradeStore(store),
		WithReleaseData(releaseData),
		WithClusterID("test-cluster-id"),
		WithAirgapBundle("test-bundle.tar.gz"),
	)

	require.NoError(t, err)
	assert.NotNil(t, manager)
	assert.Equal(t, releaseData, manager.releaseData)
	assert.Equal(t, "test-cluster-id", manager.clusterID)
	assert.Equal(t, "test-bundle.tar.gz", manager.airgapBundle)
}

func TestAppUpgradeManager_Upgrade_ConfigValuesSecret(t *testing.T) {
	// Setup environment variable for V3
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
			name: "creates secret with config values if it does not exist",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					AppSlug:         "test-app",
					VersionLabel:    "v1.0.0",
					ChannelID:       "channel-123",
					ChannelSequence: 456,
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

				var cv kotsv1beta1.ConfigValues
				err = kyaml.Unmarshal(data, &cv)
				require.NoError(t, err)
				assert.Equal(t, "value1", cv.Spec.Values["key1"].Value)
				assert.Equal(t, "value2", cv.Spec.Values["key2"].Value)
			},
			validateKotsCalled: true,
		},
		{
			name: "existing secret is fetched and updated",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					AppSlug:         "test-app",
					VersionLabel:    "v2.0.0",
					ChannelID:       "channel-123",
					ChannelSequence: 457,
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

				existingSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app-config-values",
						Namespace: "test-app",
						Labels: map[string]string{
							"app.kubernetes.io/version": "v1.0.0",
						},
					},
					Data: map[string][]byte{
						"config-values.yaml": []byte("spec:\n  values:\n    oldkey:\n      value: oldvalue"),
					},
				}

				return clientfake.NewClientBuilder().
					WithScheme(sch).
					WithObjects(existingSecret).
					Build()
			},
			expectError: false,
			validateSecret: func(t *testing.T, kcli client.Client) {
				secret := &corev1.Secret{}
				err := kcli.Get(context.Background(), client.ObjectKey{
					Name:      "test-app-config-values",
					Namespace: "test-app",
				}, secret)
				require.NoError(t, err)

				// Verify updated version label
				assert.Equal(t, "v2.0.0", secret.Labels["app.kubernetes.io/version"])

				// Verify new data
				var cv kotsv1beta1.ConfigValues
				err = kyaml.Unmarshal(secret.Data["config-values.yaml"], &cv)
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
			expectedErrorContains: "release data",
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
			expectedErrorContains: "release data",
			validateKotsCalled:    false,
		},
		{
			name: "fails when get returns error",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					AppSlug:         "test-app",
					VersionLabel:    "v1.0.0",
					ChannelID:       "channel-123",
					ChannelSequence: 456,
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
					AppSlug:         "test-app",
					VersionLabel:    "v1.0.0",
					ChannelID:       "channel-123",
					ChannelSequence: 456,
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
			name: "handles empty config values",
			releaseData: &release.ReleaseData{
				ChannelRelease: &release.ChannelRelease{
					AppSlug:         "test-app",
					VersionLabel:    "v1.0.0",
					ChannelID:       "channel-123",
					ChannelSequence: 456,
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
			// Setup license
			license := &kotsv1beta1.License{
				Spec: kotsv1beta1.LicenseSpec{AppSlug: "test-app"},
			}
			licenseBytes, err := kyaml.Marshal(license)
			require.NoError(t, err)

			// Setup mock KotsCLI
			mockKotsCLI := &kotscli.MockKotsCLI{}
			if tt.validateKotsCalled {
				mockKotsCLI.On("Deploy", mock.Anything).Return(nil)
			}

			// Setup fake kube client
			kcli := tt.setupClient(t)

			// Create manager
			store := appupgradestore.NewMemoryStore()
			manager, err := NewAppUpgradeManager(
				WithLogger(logger.NewDiscardLogger()),
				WithAppUpgradeStore(store),
				WithReleaseData(tt.releaseData),
				WithLicense(licenseBytes),
				WithClusterID("test-cluster"),
				WithKotsCLI(mockKotsCLI),
				WithKubeClient(kcli),
			)
			require.NoError(t, err)

			// Execute Upgrade (not createConfigValuesSecret directly)
			err = manager.Upgrade(context.Background(), tt.configValues)

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
				mockKotsCLI.AssertNotCalled(t, "Deploy")
			}
		})
	}
}
