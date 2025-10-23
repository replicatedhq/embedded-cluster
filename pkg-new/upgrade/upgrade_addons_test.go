package upgrade

import (
	"context"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	addontypes "github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	helmrelease "helm.sh/helm/v3/pkg/release"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpgradeAddons_SuccessfulUpgrade(t *testing.T) {
	// Set fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")
	t.Setenv("AUTOPILOT_NODE_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_NODE_MAX_STEPS", "10")

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, rbacv1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))

	// Create installation with basic config
	installation := createTestInstallation()
	installation.Spec.AirGap = false
	installation.Spec.HighAvailability = false
	installation.Spec.LicenseInfo = &ecv1beta1.LicenseInfo{
		IsDisasterRecoverySupported: false,
		IsMultiNodeEnabled:          false,
	}

	// Cache metadata for the release package
	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
		Images: []string{
			"registry.example.com/embedded-cluster-operator-image:1.30.14+k0s.0",
			"registry.example.com/embedded-cluster-utils:1.30.14+k0s.0",
		},
		Configs: ecv1beta1.Helm{
			Charts: []ecv1beta1.Chart{
				{
					Name:      "embedded-cluster-operator",
					ChartName: "embedded-cluster-operator",
					Version:   "1.30.14+k0s.0",
				},
			},
		},
	}
	release.CacheMeta("1.30.14+k0s.0", *meta)

	// Create mock helm client
	helmClient := &helm.MockClient{}

	// Mock ReleaseExists for all addons
	helmClient.On("ReleaseExists", mock.Anything, "openebs", "openebs").Return(true, nil)
	helmClient.On("ReleaseExists", mock.Anything, "embedded-cluster", "embedded-cluster-operator").Return(true, nil)
	helmClient.On("ReleaseExists", mock.Anything, "kotsadm", "admin-console").Return(true, nil)

	// Mock helm operations in order
	mock.InOrder(
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "openebs"
		})).Return(&helmrelease.Release{Name: "openebs"}, nil),
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "embedded-cluster-operator"
		})).Return(&helmrelease.Release{Name: "embedded-cluster-operator"}, nil),
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "admin-console"
		})).Return(&helmrelease.Release{Name: "admin-console"}, nil),
	)

	// Create fake kube client with installation
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(installation).
		WithStatusSubresource(&ecv1beta1.Installation{}).
		Build()

	// Create runtime config
	rc := runtimeconfig.New(installation.Spec.RuntimeConfig)

	// Create upgrader
	upgrader := NewInfraUpgrader(
		WithKubeClient(cli),
		WithHelmClient(helmClient),
		WithRuntimeConfig(rc),
		WithLogger(logger),
	)

	// Test: Call UpgradeAddons
	ctx := context.Background()
	progressChan := make(chan addontypes.AddOnProgress, 10)

	err := upgrader.UpgradeAddons(ctx, installation, progressChan)
	require.NoError(t, err)

	// Verify helm operations were called
	helmClient.AssertExpectations(t)

	// Verify installation state was updated
	var updatedInstallation ecv1beta1.Installation
	err = cli.Get(ctx, types.NamespacedName{Name: installation.Name}, &updatedInstallation)
	require.NoError(t, err)
	assert.Equal(t, ecv1beta1.InstallationStateAddonsInstalled, updatedInstallation.Status.State)
}

func TestUpgradeAddons_AirgapEnvironment(t *testing.T) {
	// Set fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")
	t.Setenv("AUTOPILOT_NODE_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_NODE_MAX_STEPS", "10")

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, rbacv1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))

	// Create installation with airgap config
	installation := createTestInstallation()
	installation.Spec.AirGap = true
	installation.Spec.HighAvailability = false
	installation.Spec.LicenseInfo = &ecv1beta1.LicenseInfo{
		IsDisasterRecoverySupported: false,
		IsMultiNodeEnabled:          false,
	}

	// Cache metadata for the release package
	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
		Images: []string{
			"registry.example.com/embedded-cluster-operator-image:1.30.14+k0s.0",
			"registry.example.com/embedded-cluster-utils:1.30.14+k0s.0",
		},
		Configs: ecv1beta1.Helm{
			Charts: []ecv1beta1.Chart{
				{
					Name:      "embedded-cluster-operator",
					ChartName: "embedded-cluster-operator",
					Version:   "1.30.14+k0s.0",
				},
			},
		},
	}
	release.CacheMeta("1.30.14+k0s.0", *meta)

	// Create mock helm client
	helmClient := &helm.MockClient{}

	// Mock ReleaseExists for all addons
	helmClient.On("ReleaseExists", mock.Anything, "openebs", "openebs").Return(true, nil)
	helmClient.On("ReleaseExists", mock.Anything, "embedded-cluster", "embedded-cluster-operator").Return(true, nil)
	helmClient.On("ReleaseExists", mock.Anything, "registry", "docker-registry").Return(true, nil)
	helmClient.On("ReleaseExists", mock.Anything, "kotsadm", "admin-console").Return(true, nil)

	// Mock helm operations in order for airgap addons (includes registry)
	mock.InOrder(
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "openebs"
		})).Return(&helmrelease.Release{Name: "openebs"}, nil),
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "embedded-cluster-operator"
		})).Return(&helmrelease.Release{Name: "embedded-cluster-operator"}, nil),
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "docker-registry"
		})).Return(&helmrelease.Release{Name: "docker-registry"}, nil),
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "admin-console"
		})).Return(&helmrelease.Release{Name: "admin-console"}, nil),
	)

	// Create fake kube client with installation
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(installation).
		WithStatusSubresource(&ecv1beta1.Installation{}).
		Build()

	// Create runtime config
	rc := runtimeconfig.New(installation.Spec.RuntimeConfig)

	// Create upgrader
	upgrader := NewInfraUpgrader(
		WithKubeClient(cli),
		WithHelmClient(helmClient),
		WithRuntimeConfig(rc),
		WithLogger(logger),
	)

	// Test: Call UpgradeAddons
	ctx := context.Background()
	progressChan := make(chan addontypes.AddOnProgress, 10)

	err := upgrader.UpgradeAddons(ctx, installation, progressChan)
	require.NoError(t, err)

	// Verify helm operations were called (should include registry for airgap)
	helmClient.AssertExpectations(t)

	// Verify installation state was updated
	var updatedInstallation ecv1beta1.Installation
	err = cli.Get(ctx, types.NamespacedName{Name: installation.Name}, &updatedInstallation)
	require.NoError(t, err)
	assert.Equal(t, ecv1beta1.InstallationStateAddonsInstalled, updatedInstallation.Status.State)
}

func TestUpgradeAddons_HAEnvironment(t *testing.T) {
	// Set fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")
	t.Setenv("AUTOPILOT_NODE_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_NODE_MAX_STEPS", "10")

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, rbacv1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))

	// Create installation with HA config
	installation := createTestInstallation()
	installation.Spec.AirGap = true
	installation.Spec.HighAvailability = true
	installation.Spec.LicenseInfo = &ecv1beta1.LicenseInfo{
		IsDisasterRecoverySupported: false,
		IsMultiNodeEnabled:          false,
	}

	// Create seaweedfs secret for HA environment
	seaweedfsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secret-seaweedfs-s3",
			Namespace: "seaweedfs",
		},
		Data: map[string][]byte{
			"seaweedfs_s3_config": []byte(`{
				"identities": [
					{
						"name": "anvAdmin",
						"credentials": [
							{
								"accessKey": "test-access-key",
								"secretKey": "test-secret-key"
							}
						],
						"actions": ["Admin", "Read", "Write"]
					}
				]
			}`),
		},
	}

	// Cache metadata for the release package
	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
		Images: []string{
			"registry.example.com/embedded-cluster-operator-image:1.30.14+k0s.0",
			"registry.example.com/embedded-cluster-utils:1.30.14+k0s.0",
		},
		Configs: ecv1beta1.Helm{
			Charts: []ecv1beta1.Chart{
				{
					Name:      "embedded-cluster-operator",
					ChartName: "embedded-cluster-operator",
					Version:   "1.30.14+k0s.0",
				},
			},
		},
	}
	release.CacheMeta("1.30.14+k0s.0", *meta)

	// Create mock helm client
	helmClient := &helm.MockClient{}

	// Mock ReleaseExists for all addons
	helmClient.On("ReleaseExists", mock.Anything, "openebs", "openebs").Return(true, nil)
	helmClient.On("ReleaseExists", mock.Anything, "embedded-cluster", "embedded-cluster-operator").Return(true, nil)
	helmClient.On("ReleaseExists", mock.Anything, "registry", "docker-registry").Return(true, nil)
	helmClient.On("ReleaseExists", mock.Anything, "seaweedfs", "seaweedfs").Return(true, nil)
	helmClient.On("ReleaseExists", mock.Anything, "kotsadm", "admin-console").Return(true, nil)

	// Mock helm operations in order for HA addons (includes seaweedfs)
	mock.InOrder(
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "openebs"
		})).Return(&helmrelease.Release{Name: "openebs"}, nil),
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "embedded-cluster-operator"
		})).Return(&helmrelease.Release{Name: "embedded-cluster-operator"}, nil),
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "docker-registry"
		})).Return(&helmrelease.Release{Name: "docker-registry"}, nil),
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "seaweedfs"
		})).Return(&helmrelease.Release{Name: "seaweedfs"}, nil),
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "admin-console"
		})).Return(&helmrelease.Release{Name: "admin-console"}, nil),
	)

	// Create fake kube client with installation and seaweedfs secret
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(installation, seaweedfsSecret).
		WithStatusSubresource(&ecv1beta1.Installation{}).
		Build()

	// Create runtime config
	rc := runtimeconfig.New(installation.Spec.RuntimeConfig)

	// Create upgrader
	upgrader := NewInfraUpgrader(
		WithKubeClient(cli),
		WithHelmClient(helmClient),
		WithRuntimeConfig(rc),
		WithLogger(logger),
	)

	// Test: Call UpgradeAddons
	ctx := context.Background()
	progressChan := make(chan addontypes.AddOnProgress, 10)

	err := upgrader.UpgradeAddons(ctx, installation, progressChan)
	require.NoError(t, err)

	// Verify helm operations were called (should include seaweedfs for HA)
	helmClient.AssertExpectations(t)

	// Verify installation state was updated
	var updatedInstallation ecv1beta1.Installation
	err = cli.Get(ctx, types.NamespacedName{Name: installation.Name}, &updatedInstallation)
	require.NoError(t, err)
	assert.Equal(t, ecv1beta1.InstallationStateAddonsInstalled, updatedInstallation.Status.State)
}

func TestUpgradeAddons_DisasterRecoveryEnabled(t *testing.T) {
	// Set fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")
	t.Setenv("AUTOPILOT_NODE_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_NODE_MAX_STEPS", "10")

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, rbacv1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))

	// Create installation with disaster recovery enabled
	installation := createTestInstallation()
	installation.Spec.AirGap = false
	installation.Spec.HighAvailability = false
	installation.Spec.LicenseInfo = &ecv1beta1.LicenseInfo{
		IsDisasterRecoverySupported: true,
		IsMultiNodeEnabled:          false,
	}

	// Cache metadata for the release package
	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
		Images: []string{
			"registry.example.com/embedded-cluster-operator-image:1.30.14+k0s.0",
			"registry.example.com/embedded-cluster-utils:1.30.14+k0s.0",
		},
		Configs: ecv1beta1.Helm{
			Charts: []ecv1beta1.Chart{
				{
					Name:      "embedded-cluster-operator",
					ChartName: "embedded-cluster-operator",
					Version:   "1.30.14+k0s.0",
				},
			},
		},
	}
	release.CacheMeta("1.30.14+k0s.0", *meta)

	// Create mock helm client
	helmClient := &helm.MockClient{}

	// Mock ReleaseExists for all addons
	helmClient.On("ReleaseExists", mock.Anything, "openebs", "openebs").Return(true, nil)
	helmClient.On("ReleaseExists", mock.Anything, "embedded-cluster", "embedded-cluster-operator").Return(true, nil)
	helmClient.On("ReleaseExists", mock.Anything, "velero", "velero").Return(true, nil)
	helmClient.On("ReleaseExists", mock.Anything, "kotsadm", "admin-console").Return(true, nil)

	// Mock helm operations in order for addons including velero
	mock.InOrder(
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "openebs"
		})).Return(&helmrelease.Release{Name: "openebs"}, nil),
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "embedded-cluster-operator"
		})).Return(&helmrelease.Release{Name: "embedded-cluster-operator"}, nil),
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "velero"
		})).Return(&helmrelease.Release{Name: "velero"}, nil),
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "admin-console"
		})).Return(&helmrelease.Release{Name: "admin-console"}, nil),
	)

	// Create fake kube client with installation
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(installation).
		WithStatusSubresource(&ecv1beta1.Installation{}).
		Build()

	// Create runtime config
	rc := runtimeconfig.New(installation.Spec.RuntimeConfig)

	// Create upgrader
	upgrader := NewInfraUpgrader(
		WithKubeClient(cli),
		WithHelmClient(helmClient),
		WithRuntimeConfig(rc),
		WithLogger(logger),
	)

	// Test: Call UpgradeAddons
	ctx := context.Background()
	progressChan := make(chan addontypes.AddOnProgress, 10)

	err := upgrader.UpgradeAddons(ctx, installation, progressChan)
	require.NoError(t, err)

	// Verify helm operations were called (should include velero for disaster recovery)
	helmClient.AssertExpectations(t)

	// Verify installation state was updated
	var updatedInstallation ecv1beta1.Installation
	err = cli.Get(ctx, types.NamespacedName{Name: installation.Name}, &updatedInstallation)
	require.NoError(t, err)
	assert.Equal(t, ecv1beta1.InstallationStateAddonsInstalled, updatedInstallation.Status.State)
}

func TestUpgradeAddons_HelmFailure(t *testing.T) {
	// Set fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")
	t.Setenv("AUTOPILOT_NODE_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_NODE_MAX_STEPS", "10")

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, rbacv1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))

	// Create installation
	installation := createTestInstallation()
	installation.Spec.AirGap = false
	installation.Spec.HighAvailability = false
	installation.Spec.LicenseInfo = &ecv1beta1.LicenseInfo{
		IsDisasterRecoverySupported: false,
		IsMultiNodeEnabled:          false,
	}

	// Cache metadata for the release package
	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
		Images: []string{
			"registry.example.com/embedded-cluster-operator-image:1.30.14+k0s.0",
			"registry.example.com/embedded-cluster-utils:1.30.14+k0s.0",
		},
		Configs: ecv1beta1.Helm{
			Charts: []ecv1beta1.Chart{
				{
					Name:      "embedded-cluster-operator",
					ChartName: "embedded-cluster-operator",
					Version:   "1.30.14+k0s.0",
				},
			},
		},
	}
	release.CacheMeta("1.30.14+k0s.0", *meta)

	// Create mock helm client that fails on first upgrade
	helmClient := &helm.MockClient{}

	// Mock ReleaseExists for all addons
	helmClient.On("ReleaseExists", mock.Anything, "openebs", "openebs").Return(true, nil)

	// Mock helm operations - first one fails
	mock.InOrder(
		helmClient.On("Upgrade", mock.Anything, mock.MatchedBy(func(opts helm.UpgradeOptions) bool {
			return opts.ReleaseName == "openebs"
		})).Return(nil, assert.AnError),
	)

	// Create fake kube client with installation
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(installation).
		WithStatusSubresource(&ecv1beta1.Installation{}).
		Build()

	// Create runtime config
	rc := runtimeconfig.New(installation.Spec.RuntimeConfig)

	// Create upgrader
	upgrader := NewInfraUpgrader(
		WithKubeClient(cli),
		WithHelmClient(helmClient),
		WithRuntimeConfig(rc),
		WithLogger(logger),
	)

	// Test: Call UpgradeAddons should fail
	ctx := context.Background()
	progressChan := make(chan addontypes.AddOnProgress, 10)

	err := upgrader.UpgradeAddons(ctx, installation, progressChan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upgrade addons")

	// Verify helm operations were called
	helmClient.AssertExpectations(t)

	// Verify installation status remains in AddonsInstalling state when Helm operation fails
	var updatedInstallation ecv1beta1.Installation
	err = cli.Get(ctx, types.NamespacedName{Name: installation.Name}, &updatedInstallation)
	require.NoError(t, err)

	// Verify that a condition was set for the failed addon
	var foundCondition *metav1.Condition
	for _, condition := range updatedInstallation.Status.Conditions {
		if condition.Type == "openebs-openebs" {
			foundCondition = &condition
			break
		}
	}
	require.NotNil(t, foundCondition, "Expected condition for openebs-openebs to be set")
	assert.Equal(t, metav1.ConditionFalse, foundCondition.Status)
	assert.Equal(t, "UpgradeFailed", foundCondition.Reason)
	assert.Contains(t, foundCondition.Message, "helm upgrade")
}

// Helper function to create a test installation
func createTestInstallation() *ecv1beta1.Installation {
	return &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-installation",
		},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "v1.30.14+k0s.0",
			},
			RuntimeConfig: &ecv1beta1.RuntimeConfigSpec{
				DataDir: "/var/lib/embedded-cluster",
				Network: ecv1beta1.NetworkSpec{
					ServiceCIDR: "10.244.128.0/17",
				},
			},
		},
	}
}
