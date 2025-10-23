package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"

	apv1b2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/artifacts"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpdateClusterConfig(t *testing.T) {
	// Discard log messages
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	//nolint:staticcheck // SA1019 we are using the deprecated scheme for backwards compatibility, we can remove this once we stop supporting k0s v1.30
	require.NoError(t, k0sv1beta1.AddToScheme(scheme))

	// We need to disable telemetry in a backwards compatible way with k0s v1.30 and v1.29
	// See - https://github.com/k0sproject/k0s/pull/4674/files#diff-eea4a0c68e41d694c3fd23b4865a7b28bcbba61dc9c642e33c2e2f5f7f9ee05d
	// We can drop the json.Unmarshal once we drop support for 1.30
	telemetryConfigEnabled := k0sv1beta1.ClusterTelemetry{}
	json.Unmarshal([]byte(`true`), &telemetryConfigEnabled.Enabled)

	tests := []struct {
		name          string
		currentConfig *k0sv1beta1.ClusterConfig
		installation  *ecv1beta1.Installation
		validate      func(t *testing.T, updatedConfig *k0sv1beta1.ClusterConfig)
	}{
		{
			name: "updates images with proxy registry domain",
			currentConfig: &k0sv1beta1.ClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k0s",
					Namespace: "kube-system",
				},
			},
			installation: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Domains: ecv1beta1.Domains{
							ProxyRegistryDomain: "registry.com",
						},
					},
				},
			},
			validate: func(t *testing.T, updatedConfig *k0sv1beta1.ClusterConfig) {
				assert.Contains(t, updatedConfig.Spec.Images.CoreDNS.Image, "registry.com/")
				assert.Contains(t, updatedConfig.Spec.Images.Calico.Node.Image, "registry.com/")
				assert.Contains(t, updatedConfig.Spec.Images.Calico.CNI.Image, "registry.com/")
				assert.Contains(t, updatedConfig.Spec.Images.Calico.KubeControllers.Image, "registry.com/")
				assert.Contains(t, updatedConfig.Spec.Images.MetricsServer.Image, "registry.com/")
				assert.Contains(t, updatedConfig.Spec.Images.KubeProxy.Image, "registry.com/")
				assert.Contains(t, updatedConfig.Spec.Images.Pause.Image, "registry.com/")
				assert.Contains(t, updatedConfig.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image.Image, "registry.com/")
			},
		},
		{
			name: "updates node local load balancing when different",
			currentConfig: &k0sv1beta1.ClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k0s",
					Namespace: "kube-system",
				},
				Spec: &k0sv1beta1.ClusterSpec{
					Network: &k0sv1beta1.Network{
						NodeLocalLoadBalancing: &k0sv1beta1.NodeLocalLoadBalancing{
							Enabled: true,
							Type:    k0sv1beta1.NllbTypeEnvoyProxy,
							EnvoyProxy: &k0sv1beta1.EnvoyProxy{
								Image: &k0sv1beta1.ImageSpec{
									Image:   "some-image",
									Version: "some-version",
								},
							},
						},
					},
				},
			},
			installation: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Domains: ecv1beta1.Domains{
							ProxyRegistryDomain: "registry.com",
						},
					},
				},
			},
			validate: func(t *testing.T, updatedConfig *k0sv1beta1.ClusterConfig) {
				assert.True(t, updatedConfig.Spec.Network.NodeLocalLoadBalancing.Enabled)
				assert.Equal(t, k0sv1beta1.NllbTypeEnvoyProxy, updatedConfig.Spec.Network.NodeLocalLoadBalancing.Type)
				assert.Contains(t, updatedConfig.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image.Image, "registry.com/")
			},
		},
		{
			name: "does not enable node local load balancing when nil",
			currentConfig: &k0sv1beta1.ClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k0s",
					Namespace: "kube-system",
				},
				Spec: &k0sv1beta1.ClusterSpec{
					Network: &k0sv1beta1.Network{
						NodeLocalLoadBalancing: nil,
					},
				},
			},
			installation: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Domains: ecv1beta1.Domains{
							ProxyRegistryDomain: "registry.com",
						},
					},
				},
			},
			validate: func(t *testing.T, updatedConfig *k0sv1beta1.ClusterConfig) {
				assert.False(t, updatedConfig.Spec.Network.NodeLocalLoadBalancing.Enabled)
				assert.Equal(t, k0sv1beta1.NllbTypeEnvoyProxy, updatedConfig.Spec.Network.NodeLocalLoadBalancing.Type)
				assert.Contains(t, updatedConfig.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image.Image, "registry.com/")
			},
		},
		{
			name: "applies unsupported vendor k0s overrides",
			currentConfig: &k0sv1beta1.ClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k0s",
					Namespace: "kube-system",
				},
				Spec: &k0sv1beta1.ClusterSpec{
					Network: &k0sv1beta1.Network{
						ServiceCIDR: "10.96.0.0/12",
					},
					Telemetry: &telemetryConfigEnabled,
				},
			},
			installation: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Domains: ecv1beta1.Domains{
							ProxyRegistryDomain: "registry.com",
						},
						UnsupportedOverrides: ecv1beta1.UnsupportedOverrides{
							K0s: `
config:
  spec:
    telemetry:
      enabled: false
    workerProfiles:
    - name: ip-forward
      values:
        allowedUnsafeSysctls:
        - net.ipv4.ip_forward
`,
						},
					},
				},
			},
			validate: func(t *testing.T, updatedConfig *k0sv1beta1.ClusterConfig) {
				// Verify that the unsupported override was applied to the telemetry config
				val, err := json.Marshal(updatedConfig.Spec.Telemetry.Enabled)
				require.NoError(t, err)
				assert.Equal(t, "false", string(val))

				// Verify that the unsupported override was applied to the worker profiles
				require.Len(t, updatedConfig.Spec.WorkerProfiles, 1)
				assert.Equal(t, "ip-forward", updatedConfig.Spec.WorkerProfiles[0].Name)
				assert.Equal(t, &runtime.RawExtension{Raw: []byte(`{"allowedUnsafeSysctls":["net.ipv4.ip_forward"]}`)}, updatedConfig.Spec.WorkerProfiles[0].Config)

				// Verify that other changes were not made
				assert.Equal(t, "10.96.0.0/12", updatedConfig.Spec.Network.ServiceCIDR)
				// Verify that supported changes (like image registries) are still applied
				assert.Contains(t, updatedConfig.Spec.Images.CoreDNS.Image, "registry.com/")
			},
		},
		{
			name: "applies unsupported end user k0s overrides",
			currentConfig: &k0sv1beta1.ClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k0s",
					Namespace: "kube-system",
				},
				Spec: &k0sv1beta1.ClusterSpec{
					Network: &k0sv1beta1.Network{
						ServiceCIDR: "10.96.0.0/12",
					},
				},
			},
			installation: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Domains: ecv1beta1.Domains{
							ProxyRegistryDomain: "registry.com",
						},
					},
					EndUserK0sConfigOverrides: `
config:
  spec:
    workerProfiles:
    - name: another-profile
      values:
        allowedUnsafeSysctls:
        - net.ipv4.ip_forward
`,
				},
			},
			validate: func(t *testing.T, updatedConfig *k0sv1beta1.ClusterConfig) {
				// Verify that the unsupported override was applied to the worker profiles
				require.Len(t, updatedConfig.Spec.WorkerProfiles, 1)
				assert.Equal(t, "another-profile", updatedConfig.Spec.WorkerProfiles[0].Name)
				assert.Equal(t, &runtime.RawExtension{Raw: []byte(`{"allowedUnsafeSysctls":["net.ipv4.ip_forward"]}`)}, updatedConfig.Spec.WorkerProfiles[0].Config)

				// Verify that other changes were not made
				assert.Equal(t, "10.96.0.0/12", updatedConfig.Spec.Network.ServiceCIDR)
				// Verify that supported changes (like image registries) are still applied
				assert.Contains(t, updatedConfig.Spec.Images.CoreDNS.Image, "registry.com/")
			},
		},
		{
			name: "immutable fields are not changed by unsupported overrides",
			currentConfig: &k0sv1beta1.ClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k0s",
					Namespace: "kube-system",
				},
				Spec: &k0sv1beta1.ClusterSpec{
					Network: &k0sv1beta1.Network{
						ServiceCIDR: "10.96.0.0/12",
					},
					Storage: &k0sv1beta1.StorageSpec{
						Type: "etcd",
					},
					API: &k0sv1beta1.APISpec{
						Address: "192.168.1.1",
					},
				},
			},
			installation: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Domains: ecv1beta1.Domains{
							ProxyRegistryDomain: "registry.com",
						},
					},
					EndUserK0sConfigOverrides: `
config:
  metadata:
    name: foo
  spec:
    api:
      address: 111.111.111.111
    storage:
      type: local
    workerProfiles:
    - name: another-profile
      values:
        allowedUnsafeSysctls:
        - net.ipv4.ip_forward
`,
				},
			},
			validate: func(t *testing.T, updatedConfig *k0sv1beta1.ClusterConfig) {
				// Verify that the unsupported override was applied to the worker profiles
				require.Len(t, updatedConfig.Spec.WorkerProfiles, 1)
				assert.Equal(t, "another-profile", updatedConfig.Spec.WorkerProfiles[0].Name)
				assert.Equal(t, &runtime.RawExtension{Raw: []byte(`{"allowedUnsafeSysctls":["net.ipv4.ip_forward"]}`)}, updatedConfig.Spec.WorkerProfiles[0].Config)

				// Verify that the immutable fields are not changed
				assert.Equal(t, "k0s", updatedConfig.Name)
				assert.Equal(t, "192.168.1.1", updatedConfig.Spec.API.Address)
				assert.Equal(t, k0sv1beta1.EtcdStorageType, updatedConfig.Spec.Storage.Type)

				// Verify that other changes were not made
				assert.Equal(t, "10.96.0.0/12", updatedConfig.Spec.Network.ServiceCIDR)
				// Verify that supported changes (like image registries) are still applied
				assert.Contains(t, updatedConfig.Spec.Images.CoreDNS.Image, "registry.com/")
			},
		},
		{
			name: "deduplicates API SANs when duplicates are present",
			currentConfig: &k0sv1beta1.ClusterConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "k0s",
					Namespace: "kube-system",
				},
				Spec: &k0sv1beta1.ClusterSpec{
					API: &k0sv1beta1.APISpec{
						Address: "192.168.1.1",
						// Simulate duplicate SANs that might occur from k0s automatically adding node IPs
						SANs: []string{
							"192.168.1.1",
							"fe80::ecee:eeff:feee:eeee",
							"kubernetes.default.svc.cluster.local",
							"fe80::ecee:eeff:feee:eeee", // duplicate IPv6 link-local address
						},
					},
					Network: &k0sv1beta1.Network{
						ServiceCIDR: "10.96.0.0/12",
					},
					Telemetry: &telemetryConfigEnabled,
				},
			},
			installation: &ecv1beta1.Installation{
				Spec: ecv1beta1.InstallationSpec{
					Config: &ecv1beta1.ConfigSpec{
						Domains: ecv1beta1.Domains{
							ProxyRegistryDomain: "registry.com",
						},
						UnsupportedOverrides: ecv1beta1.UnsupportedOverrides{
							K0s: `
config:
  spec:
    telemetry:
      enabled: false
`,
						},
					},
				},
			},
			validate: func(t *testing.T, updatedConfig *k0sv1beta1.ClusterConfig) {
				// Verify that duplicate SANs are removed
				require.NotNil(t, updatedConfig.Spec.API)
				assert.Len(t, updatedConfig.Spec.API.SANs, 3, "Should have 3 unique SANs")
				assert.Contains(t, updatedConfig.Spec.API.SANs, "192.168.1.1")
				assert.Contains(t, updatedConfig.Spec.API.SANs, "fe80::ecee:eeff:feee:eeee")
				assert.Contains(t, updatedConfig.Spec.API.SANs, "kubernetes.default.svc.cluster.local")

				// Verify order is preserved (first occurrence kept)
				assert.Equal(t, "192.168.1.1", updatedConfig.Spec.API.SANs[0])
				assert.Equal(t, "fe80::ecee:eeff:feee:eeee", updatedConfig.Spec.API.SANs[1])
				assert.Equal(t, "kubernetes.default.svc.cluster.local", updatedConfig.Spec.API.SANs[2])

				// Verify that the patch was applied
				val, err := json.Marshal(updatedConfig.Spec.Telemetry.Enabled)
				require.NoError(t, err)
				assert.Equal(t, "false", string(val))

				// Verify that image registries are still updated
				assert.Contains(t, updatedConfig.Spec.Images.CoreDNS.Image, "registry.com/")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.currentConfig).
				Build()

			err := updateClusterConfig(context.Background(), cli, tt.installation, logger)
			require.NoError(t, err)

			var updatedConfig k0sv1beta1.ClusterConfig
			err = cli.Get(context.Background(), client.ObjectKey{Name: "k0s", Namespace: "kube-system"}, &updatedConfig)
			require.NoError(t, err)

			tt.validate(t, &updatedConfig)
		})
	}
}

func TestWaitForAutopilotPlan_Success(t *testing.T) {
	// Set fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))

	plan := &apv1b2.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot",
		},
		Status: apv1b2.PlanStatus{
			State: core.PlanCompleted,
		},
	}

	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(plan).
		Build()

	result, err := waitForAutopilotPlan(t.Context(), cli, logger)
	require.NoError(t, err)
	assert.Equal(t, "autopilot", result.Name)
}

func TestWaitForAutopilotPlan_RetriesOnTransientErrors(t *testing.T) {
	// Set fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))

	// Plan that starts completed
	plan := &apv1b2.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot",
		},
		Status: apv1b2.PlanStatus{
			State: core.PlanCompleted,
		},
	}

	// Mock client that fails first 3 times, then succeeds
	var callCount atomic.Int32
	cli := &mockClientWithRetries{
		Client:       fake.NewClientBuilder().WithScheme(scheme).WithObjects(plan).Build(),
		failCount:    3,
		currentCount: &callCount,
	}

	result, err := waitForAutopilotPlan(t.Context(), cli, logger)
	require.NoError(t, err)
	assert.Equal(t, "autopilot", result.Name)
	assert.Equal(t, int32(4), callCount.Load(), "Should have retried 3 times before succeeding")
}

func TestWaitForAutopilotPlan_ContextCanceled(t *testing.T) {
	// Set fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	cli := fake.NewClientBuilder().WithScheme(scheme).Build()

	_, err := waitForAutopilotPlan(ctx, cli, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestWaitForAutopilotPlan_WaitsForCompletion(t *testing.T) {
	// Set fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))

	// Plan that starts in progress, then completes after some time
	plan := &apv1b2.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot",
		},
		Spec: apv1b2.PlanSpec{
			ID: "test-plan",
		},
		Status: apv1b2.PlanStatus{
			State: core.PlanSchedulable,
		},
	}

	cli := &mockClientWithStateChange{
		Client:     fake.NewClientBuilder().WithScheme(scheme).WithObjects(plan).Build(),
		plan:       plan,
		callsUntil: 3, // Will complete after 3 calls
	}

	result, err := waitForAutopilotPlan(t.Context(), cli, logger)
	require.NoError(t, err)
	assert.Equal(t, "autopilot", result.Name)
	assert.Equal(t, core.PlanCompleted, result.Status.State)
}

// Mock client that fails N times before succeeding
type mockClientWithRetries struct {
	client.Client
	failCount    int
	currentCount *atomic.Int32
}

func (m *mockClientWithRetries) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	count := m.currentCount.Add(1)
	if count <= int32(m.failCount) {
		return fmt.Errorf("connection refused")
	}
	return m.Client.Get(ctx, key, obj, opts...)
}

// Tests for upgradeK0s function
func TestUpgradeK0s_SuccessfulUpgrade(t *testing.T) {
	// Set fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")
	t.Setenv("AUTOPILOT_NODE_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_NODE_MAX_STEPS", "10")

	ctx := context.Background()

	// Setup scheme with required types
	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	// Create test installation
	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-installation",
		},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "v1.30.14+k0s.0",
			},
			AirGap: true,
		},
	}

	// Create nodes that start with old version, then get updated to new version
	nodes := &corev1.NodeList{
		Items: []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.30.13+k0s", // Old version initially
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node-2"},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.30.13+k0s", // Old version initially
					},
				},
			},
		},
	}

	// Mock client that simulates plan state changes during waiting
	mockClient := &mockClientWithStateChange{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(installation).
			WithLists(nodes).
			WithStatusSubresource(&ecv1beta1.Installation{}).
			Build(),
		callsUntil:       5, // Will complete after 5 calls
		finalPlanState:   core.PlanCompleted,
		finalNodeVersion: "v1.30.14+k0s",
	}

	// Mock release metadata
	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
		K0sSHA: "abc123def456",
		Artifacts: map[string]string{
			"k0s": "k0s-v1.30.14+k0s.0-linux-amd64",
		},
	}

	// Cache the metadata for the release package
	release.CacheMeta("1.30.14+k0s.0", *meta)

	// Create runtime config
	rc := runtimeconfig.New(nil)
	rc.SetLocalArtifactMirrorPort(50000)

	// Create logger
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Reduce noise in tests

	// Test: Call UpgradeK0s through the public interface
	upgrader := NewInfraUpgrader(
		WithKubeClient(mockClient),
		WithRuntimeConfig(rc),
		WithLogger(logger),
	)
	err := upgrader.UpgradeK0s(ctx, installation)

	// Verify success
	require.NoError(t, err)
}

func TestUpgradeK0s_AlreadyUpToDate(t *testing.T) {
	// Set fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")
	t.Setenv("AUTOPILOT_NODE_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_NODE_MAX_STEPS", "10")

	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-installation"},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "v1.30.14+k0s.0",
			},
		},
	}

	// Create nodes with target version already
	nodes := &corev1.NodeList{
		Items: []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.30.14+k0s", // Already at target version
					},
				},
			},
		},
	}

	mockClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(installation).
		WithLists(nodes).
		WithStatusSubresource(&ecv1beta1.Installation{}).
		Build()

	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
	}
	release.CacheMeta("1.30.14+k0s.0", *meta)

	rc := runtimeconfig.New(nil)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Test: Should return early without creating plan
	upgrader := NewInfraUpgrader(
		WithKubeClient(mockClient),
		WithRuntimeConfig(rc),
		WithLogger(logger),
	)
	err := upgrader.UpgradeK0s(ctx, installation)
	require.NoError(t, err)
}

func TestUpgradeK0s_PlanFails(t *testing.T) {
	// Set fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")
	t.Setenv("AUTOPILOT_NODE_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_NODE_MAX_STEPS", "10")

	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-installation"},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "v1.30.14+k0s.0",
			},
		},
	}

	// Create nodes with old version
	nodes := &corev1.NodeList{
		Items: []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.30.13+k0s",
					},
				},
			},
		},
	}

	// Mock client that simulates plan failure
	mockClient := &mockClientWithStateChange{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(installation).
			WithLists(nodes).
			WithStatusSubresource(&ecv1beta1.Installation{}).
			Build(),
		callsUntil:       3, // Will fail after 3 calls
		finalPlanState:   core.PlanApplyFailed,
		finalNodeVersion: "v1.30.13+k0s", // Keep old version
	}

	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
	}
	release.CacheMeta("1.30.14+k0s.0", *meta)

	rc := runtimeconfig.New(nil)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Test: Should return error when plan fails
	upgrader := NewInfraUpgrader(
		WithKubeClient(mockClient),
		WithRuntimeConfig(rc),
		WithLogger(logger),
	)
	err := upgrader.UpgradeK0s(ctx, installation)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "autopilot plan failed")
	assert.Contains(t, err.Error(), "Upgrade apply has failed")
}

func TestUpgradeK0s_NodesNotUpgradedAfterPlan(t *testing.T) {
	// Set fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")
	t.Setenv("AUTOPILOT_NODE_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_NODE_MAX_STEPS", "10")

	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-installation"},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "v1.30.14+k0s.0",
			},
		},
	}

	// Create nodes that still have old version after plan completion
	nodes := &corev1.NodeList{
		Items: []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
				Status: corev1.NodeStatus{
					NodeInfo: corev1.NodeSystemInfo{
						KubeletVersion: "v1.30.13+k0s", // Still old version
					},
				},
			},
		},
	}

	// Mock client that simulates plan completion but nodes don't get updated
	mockClient := &mockClientWithStateChange{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(installation).
			WithLists(nodes).
			WithStatusSubresource(&ecv1beta1.Installation{}).
			Build(),
		callsUntil:       3, // Will complete after 3 calls
		finalPlanState:   core.PlanCompleted,
		finalNodeVersion: "v1.30.13+k0s", // Keep old version - nodes don't get updated
	}

	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
	}
	release.CacheMeta("1.30.14+k0s.0", *meta)

	rc := runtimeconfig.New(nil)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Test: Should return error when nodes don't match version after plan
	upgrader := NewInfraUpgrader(
		WithKubeClient(mockClient),
		WithRuntimeConfig(rc),
		WithLogger(logger),
	)
	err := upgrader.UpgradeK0s(ctx, installation)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster nodes did not match version")
}

// Mock client that changes plan state after N calls
type mockClientWithStateChange struct {
	client.Client
	plan             *apv1b2.Plan
	callCount        int
	callsUntil       int
	finalPlanState   apv1b2.PlanStateType
	finalNodeVersion string
}

func (m *mockClientWithStateChange) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	m.callCount++
	err := m.Client.Get(ctx, key, obj, opts...)
	if err != nil {
		return err
	}

	// After N calls, mark the plan with the specified final state
	if m.callCount >= m.callsUntil {
		if plan, ok := obj.(*apv1b2.Plan); ok {
			if m.finalPlanState != "" {
				plan.Status.State = m.finalPlanState
			} else {
				plan.Status.State = core.PlanCompleted // Default behavior
			}
		}
	}

	return nil
}

func (m *mockClientWithStateChange) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	err := m.Client.List(ctx, list, opts...)
	if err != nil {
		return err
	}

	// After plan completion, update node versions to the specified version
	if m.callCount >= m.callsUntil && m.finalNodeVersion != "" {
		if nodeList, ok := list.(*corev1.NodeList); ok {
			for i := range nodeList.Items {
				nodeList.Items[i].Status.NodeInfo.KubeletVersion = m.finalNodeVersion
			}
		}
	}

	return nil
}

func TestWaitForClusterNodesMatchVersion(t *testing.T) {
	// Set fast polling for tests
	t.Setenv("AUTOPILOT_NODE_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_NODE_MAX_STEPS", "10")

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))

	tests := []struct {
		name          string
		nodes         *corev1.NodeList
		targetVersion string
		mockClient    func(*corev1.NodeList) client.Client
		expectError   bool
		errorContains string
		validate      func(t *testing.T, cli client.Client)
	}{
		{
			name: "all nodes already match version",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node1"},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.30.0+k0s",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node2"},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.30.0+k0s",
							},
						},
					},
				},
			},
			targetVersion: "v1.30.0+k0s",
			mockClient: func(nodes *corev1.NodeList) client.Client {
				return fake.NewClientBuilder().WithScheme(scheme).WithLists(nodes).Build()
			},
			expectError: false,
		},
		{
			name: "nodes update after retries",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node1"},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.29.0+k0s",
							},
						},
					},
				},
			},
			targetVersion: "v1.30.0+k0s",
			mockClient: func(nodes *corev1.NodeList) client.Client {
				return &mockClientWithNodeVersionUpdate{
					Client:         fake.NewClientBuilder().WithScheme(scheme).WithLists(nodes).Build(),
					callsUntil:     3,
					targetVersion:  "v1.30.0+k0s",
					initialVersion: "v1.29.0+k0s",
				}
			},
			expectError: false,
			validate: func(t *testing.T, cli client.Client) {
				if mock, ok := cli.(*mockClientWithNodeVersionUpdate); ok {
					assert.Equal(t, 3, mock.callCount, "Should have retried until nodes reported correct version")
				}
			},
		},
		{
			name: "multi-node staggered updates",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node1"},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.29.0+k0s",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node2"},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.29.0+k0s",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node3"},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.29.0+k0s",
							},
						},
					},
				},
			},
			targetVersion: "v1.30.0+k0s",
			mockClient: func(nodes *corev1.NodeList) client.Client {
				return &mockClientWithStaggeredNodeUpdates{
					Client:        fake.NewClientBuilder().WithScheme(scheme).WithLists(nodes).Build(),
					targetVersion: "v1.30.0+k0s",
				}
			},
			expectError: false,
			validate: func(t *testing.T, cli client.Client) {
				if mock, ok := cli.(*mockClientWithStaggeredNodeUpdates); ok {
					assert.GreaterOrEqual(t, mock.callCount, 3, "Should have waited for all nodes to update")
				}
			},
		},
		{
			name: "timeout when nodes never match",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "node1"},
						Status: corev1.NodeStatus{
							NodeInfo: corev1.NodeSystemInfo{
								KubeletVersion: "v1.29.0+k0s",
							},
						},
					},
				},
			},
			targetVersion: "v1.30.0+k0s",
			mockClient: func(nodes *corev1.NodeList) client.Client {
				return fake.NewClientBuilder().WithScheme(scheme).WithLists(nodes).Build()
			},
			expectError:   true,
			errorContains: "cluster nodes did not match version v1.30.0+k0s after upgrade",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := tt.mockClient(tt.nodes)
			err := waitForClusterNodesMatchVersion(context.Background(), cli, tt.targetVersion, logger)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, cli)
				}
			}
		})
	}
}

// Mock client that updates node versions after N calls
type mockClientWithNodeVersionUpdate struct {
	client.Client
	callCount      int
	callsUntil     int
	targetVersion  string
	initialVersion string
}

func (m *mockClientWithNodeVersionUpdate) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	m.callCount++
	err := m.Client.List(ctx, list, opts...)
	if err != nil {
		return err
	}

	if m.callCount >= m.callsUntil {
		if nodeList, ok := list.(*corev1.NodeList); ok {
			for i := range nodeList.Items {
				nodeList.Items[i].Status.NodeInfo.KubeletVersion = m.targetVersion
			}
		}
	}

	return nil
}

// Mock client that updates nodes one at a time to simulate staggered upgrades
type mockClientWithStaggeredNodeUpdates struct {
	client.Client
	callCount     int
	targetVersion string
}

func (m *mockClientWithStaggeredNodeUpdates) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	m.callCount++
	err := m.Client.List(ctx, list, opts...)
	if err != nil {
		return err
	}

	if nodeList, ok := list.(*corev1.NodeList); ok {
		for i := range nodeList.Items {
			if m.callCount > i {
				nodeList.Items[i].Status.NodeInfo.KubeletVersion = m.targetVersion
			}
		}
	}

	return nil
}

// Tests for createAutopilotPlan function
func TestCreateAutopilotPlan_PlanAlreadyExists(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	// Create existing plan
	existingPlan := &apv1b2.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot",
		},
		Spec: apv1b2.PlanSpec{
			ID: "existing-plan-id",
		},
	}

	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-installation"},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "v1.30.14+k0s.0",
			},
		},
	}

	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
		K0sSHA: "abc123def456",
		Artifacts: map[string]string{
			"k0s": "k0s-v1.30.14+k0s.0-linux-amd64",
		},
	}

	mockClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(existingPlan, installation).
		WithStatusSubresource(&ecv1beta1.Installation{}).
		Build()

	rc := runtimeconfig.New(nil)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Test: Should not create a new plan when one already exists
	err := createAutopilotPlan(ctx, mockClient, rc, "v1.30.14+k0s", installation, meta, logger)
	require.NoError(t, err)

	// Verify no new plan was created by checking the existing plan is unchanged
	var plan apv1b2.Plan
	err = mockClient.Get(ctx, client.ObjectKey{Name: "autopilot"}, &plan)
	require.NoError(t, err)
	assert.Equal(t, "existing-plan-id", plan.Spec.ID)
}

func TestCreateAutopilotPlan_AirgapSingleNode(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	// Create single controller node
	nodes := &corev1.NodeList{
		Items: []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "controller-1",
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
					},
				},
			},
		},
	}

	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-installation"},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "v1.30.14+k0s.0",
			},
			AirGap: true,
		},
	}

	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
		K0sSHA: "abc123def456",
		Artifacts: map[string]string{
			"k0s": "k0s-v1.30.14+k0s.0-linux-amd64",
		},
	}

	mockClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(installation).
		WithLists(nodes).
		WithStatusSubresource(&ecv1beta1.Installation{}).
		Build()

	rc := runtimeconfig.New(nil)
	rc.SetLocalArtifactMirrorPort(50000)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Test: Should create plan with local artifact mirror URL for airgap
	err := createAutopilotPlan(ctx, mockClient, rc, "v1.30.14+k0s", installation, meta, logger)
	require.NoError(t, err)

	// Verify plan was created
	var plan apv1b2.Plan
	err = mockClient.Get(ctx, client.ObjectKey{Name: "autopilot"}, &plan)
	require.NoError(t, err)
	assert.Equal(t, "autopilot", plan.Name)
	assert.Equal(t, "test-installation", plan.Annotations[artifacts.InstallationNameAnnotation])
	assert.Len(t, plan.Spec.Commands, 1)
	assert.NotNil(t, plan.Spec.Commands[0].K0sUpdate)
	assert.Equal(t, "v1.30.14+k0s.0", plan.Spec.Commands[0].K0sUpdate.Version)

	// Verify airgap URL format
	platformKey := fmt.Sprintf("%s-%s", helpers.ClusterOS(), helpers.ClusterArch())
	platformResource, exists := plan.Spec.Commands[0].K0sUpdate.Platforms[platformKey]
	require.True(t, exists)
	assert.Equal(t, "http://127.0.0.1:50000/bin/k0s-upgrade", platformResource.URL)
	assert.Equal(t, "abc123def456", platformResource.Sha256)

	// Verify single controller target
	assert.Len(t, plan.Spec.Commands[0].K0sUpdate.Targets.Controllers.Discovery.Static.Nodes, 1)
	assert.Equal(t, "controller-1", plan.Spec.Commands[0].K0sUpdate.Targets.Controllers.Discovery.Static.Nodes[0])
	assert.Len(t, plan.Spec.Commands[0].K0sUpdate.Targets.Workers.Discovery.Static.Nodes, 0)
}

func TestCreateAutopilotPlan_AirgapMultiNode(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	// Create multi-node cluster
	nodes := &corev1.NodeList{
		Items: []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "controller-1",
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "controller-2",
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker-1",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "worker-2",
				},
			},
		},
	}

	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-installation"},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "v1.30.14+k0s.0",
			},
			AirGap: true,
		},
	}

	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
		K0sSHA: "abc123def456",
		Artifacts: map[string]string{
			"k0s": "k0s-v1.30.14+k0s.0-linux-amd64",
		},
	}

	mockClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(installation).
		WithLists(nodes).
		WithStatusSubresource(&ecv1beta1.Installation{}).
		Build()

	rc := runtimeconfig.New(nil)
	rc.SetLocalArtifactMirrorPort(50000)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Test: Should create plan with local artifact mirror URL for airgap multi-node
	err := createAutopilotPlan(ctx, mockClient, rc, "v1.30.14+k0s", installation, meta, logger)
	require.NoError(t, err)

	// Verify plan was created
	var plan apv1b2.Plan
	err = mockClient.Get(ctx, client.ObjectKey{Name: "autopilot"}, &plan)
	require.NoError(t, err)
	assert.Equal(t, "autopilot", plan.Name)
	assert.Equal(t, "test-installation", plan.Annotations[artifacts.InstallationNameAnnotation])
	assert.Len(t, plan.Spec.Commands, 1)
	assert.NotNil(t, plan.Spec.Commands[0].K0sUpdate)
	assert.Equal(t, "v1.30.14+k0s.0", plan.Spec.Commands[0].K0sUpdate.Version)

	// Verify airgap URL format
	platformKey := fmt.Sprintf("%s-%s", helpers.ClusterOS(), helpers.ClusterArch())
	platformResource, exists := plan.Spec.Commands[0].K0sUpdate.Platforms[platformKey]
	require.True(t, exists)
	assert.Equal(t, "http://127.0.0.1:50000/bin/k0s-upgrade", platformResource.URL)
	assert.Equal(t, "abc123def456", platformResource.Sha256)

	// Verify multi-node targets
	controllerNodes := plan.Spec.Commands[0].K0sUpdate.Targets.Controllers.Discovery.Static.Nodes
	workerNodes := plan.Spec.Commands[0].K0sUpdate.Targets.Workers.Discovery.Static.Nodes
	assert.Len(t, controllerNodes, 2)
	assert.Len(t, workerNodes, 2)
	assert.Contains(t, controllerNodes, "controller-1")
	assert.Contains(t, controllerNodes, "controller-2")
	assert.Contains(t, workerNodes, "worker-1")
	assert.Contains(t, workerNodes, "worker-2")
}

func TestCreateAutopilotPlan_OnlineSingleNode(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	// Create single controller node
	nodes := &corev1.NodeList{
		Items: []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "controller-1",
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
					},
				},
			},
		},
	}

	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-installation"},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "v1.30.14+k0s.0",
			},
			AirGap:         false,
			MetricsBaseURL: "https://metrics.example.com",
		},
	}

	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
		K0sSHA: "abc123def456",
		Artifacts: map[string]string{
			"k0s": "k0s-v1.30.14+k0s.0-linux-amd64",
		},
	}

	mockClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(installation).
		WithLists(nodes).
		WithStatusSubresource(&ecv1beta1.Installation{}).
		Build()

	rc := runtimeconfig.New(nil)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Test: Should create plan with metrics base URL for online
	err := createAutopilotPlan(ctx, mockClient, rc, "v1.30.14+k0s", installation, meta, logger)
	require.NoError(t, err)

	// Verify plan was created
	var plan apv1b2.Plan
	err = mockClient.Get(ctx, client.ObjectKey{Name: "autopilot"}, &plan)
	require.NoError(t, err)
	assert.Equal(t, "autopilot", plan.Name)
	assert.Equal(t, "test-installation", plan.Annotations[artifacts.InstallationNameAnnotation])
	assert.Len(t, plan.Spec.Commands, 1)
	assert.NotNil(t, plan.Spec.Commands[0].K0sUpdate)
	assert.Equal(t, "v1.30.14+k0s.0", plan.Spec.Commands[0].K0sUpdate.Version)

	// Verify online URL format
	platformKey := fmt.Sprintf("%s-%s", helpers.ClusterOS(), helpers.ClusterArch())
	platformResource, exists := plan.Spec.Commands[0].K0sUpdate.Platforms[platformKey]
	require.True(t, exists)
	expectedURL := "https://metrics.example.com/embedded-cluster-public-files/k0s-v1.30.14+k0s.0-linux-amd64"
	assert.Equal(t, expectedURL, platformResource.URL)
	assert.Equal(t, "abc123def456", platformResource.Sha256)

	// Verify single controller target
	assert.Len(t, plan.Spec.Commands[0].K0sUpdate.Targets.Controllers.Discovery.Static.Nodes, 1)
	assert.Equal(t, "controller-1", plan.Spec.Commands[0].K0sUpdate.Targets.Controllers.Discovery.Static.Nodes[0])
	assert.Len(t, plan.Spec.Commands[0].K0sUpdate.Targets.Workers.Discovery.Static.Nodes, 0)
}

func TestCreateAutopilotPlan_OnlineWithHttpUrl(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	// Create single controller node
	nodes := &corev1.NodeList{
		Items: []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "controller-1",
					Labels: map[string]string{
						"node-role.kubernetes.io/control-plane": "",
					},
				},
			},
		},
	}

	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-installation"},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "v1.30.14+k0s.0",
			},
			AirGap:         false,
			MetricsBaseURL: "https://metrics.example.com",
		},
	}

	// Test with HTTP URL override (for dev/e2e tests)
	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
		K0sSHA: "abc123def456",
		Artifacts: map[string]string{
			"k0s": "https://dev.example.com/k0s-v1.30.14+k0s.0-linux-amd64",
		},
	}

	mockClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(installation).
		WithLists(nodes).
		WithStatusSubresource(&ecv1beta1.Installation{}).
		Build()

	rc := runtimeconfig.New(nil)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Test: Should use the HTTP URL directly when provided
	err := createAutopilotPlan(ctx, mockClient, rc, "v1.30.14+k0s", installation, meta, logger)
	require.NoError(t, err)

	// Verify plan was created
	var plan apv1b2.Plan
	err = mockClient.Get(ctx, client.ObjectKey{Name: "autopilot"}, &plan)
	require.NoError(t, err)

	// Verify HTTP URL is used directly
	platformKey := fmt.Sprintf("%s-%s", helpers.ClusterOS(), helpers.ClusterArch())
	platformResource, exists := plan.Spec.Commands[0].K0sUpdate.Platforms[platformKey]
	require.True(t, exists)
	assert.Equal(t, "https://dev.example.com/k0s-v1.30.14+k0s.0-linux-amd64", platformResource.URL)
	assert.Equal(t, "abc123def456", platformResource.Sha256)
}

func TestCreateAutopilotPlan_GetError(t *testing.T) {
	ctx := context.Background()

	scheme := runtime.NewScheme()
	require.NoError(t, apv1b2.Install(scheme))
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-installation"},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: "v1.30.14+k0s.0",
			},
		},
	}

	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
	}

	// Mock client that returns an error on Get
	mockClient := &mockClientWithGetError{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(installation).
			WithStatusSubresource(&ecv1beta1.Installation{}).
			Build(),
	}

	rc := runtimeconfig.New(nil)
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Test: Should return error when Get fails
	err := createAutopilotPlan(ctx, mockClient, rc, "v1.30.14+k0s", installation, meta, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get upgrade plan")
	assert.Contains(t, err.Error(), "connection refused")
}

// Mock client that returns an error on Get
type mockClientWithGetError struct {
	client.Client
}

func (m *mockClientWithGetError) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return fmt.Errorf("connection refused")
}
