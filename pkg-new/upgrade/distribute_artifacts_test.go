package upgrade

import (
	"context"
	"testing"

	apv1b2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/artifacts"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func createTestNodes() []corev1.Node {
	return []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node1",
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "node2",
			},
			Status: corev1.NodeStatus{
				Conditions: []corev1.NodeCondition{
					{
						Type:   corev1.NodeReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}
}

func createFailedAutopilotPlan() *apv1b2.Plan {
	return &apv1b2.Plan{
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot",
			Annotations: map[string]string{
				artifacts.InstallationNameAnnotation: "test-installation",
			},
		},
		Status: apv1b2.PlanStatus{
			State: core.PlanApplyFailed,
		},
	}
}

func TestDistributeArtifacts_SuccessfulOnline(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()

	// Set up fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")
	t.Setenv("AUTOPILOT_NODE_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_NODE_MAX_STEPS", "10")

	// Create fake client with necessary schemes
	scheme := runtime.NewScheme()
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apv1b2.AddToScheme(scheme))

	// Create test nodes
	nodes := createTestNodes()
	initialObjects := make([]client.Object, len(nodes))
	for i, node := range nodes {
		initialObjects[i] = &node
	}

	baseCli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&ecv1beta1.Installation{}).
		WithObjects(initialObjects...).
		Build()

	// Wrap with mock client that simulates successful job completion
	mockCli := &mockClientWithStateChange{
		Client:         baseCli,
		callCount:      0,
		callsUntil:     1,                               // Change state after first call
		finalJobStatus: batchv1.JobStatus{Succeeded: 1}, // Mark jobs as succeeded
		// No autopilot plan needed for online mode
	}

	// Create upgrader
	upgrader := NewInfraUpgrader(
		WithKubeClient(mockCli),
		WithRuntimeConfig(runtimeconfig.New(&ecv1beta1.RuntimeConfigSpec{})),
		WithLogger(logger),
	)

	// Create test installation
	installation := createTestInstallation()
	installation.Spec.AirGap = false

	// Execute the function
	err := upgrader.DistributeArtifacts(ctx, installation, "test-mirror:latest", "test-license", "test-app", "test-channel", "1.0.0")

	// The function should succeed as our mock client simulates successful job completion
	require.NoError(t, err)
}

func TestDistributeArtifacts_SuccessfulAirgap(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()

	// Set up fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")
	t.Setenv("AUTOPILOT_NODE_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_NODE_MAX_STEPS", "10")

	// Cache release metadata for the test
	meta := &ectypes.ReleaseMetadata{
		Versions: map[string]string{
			"Kubernetes": "v1.30.14+k0s.0",
		},
	}
	release.CacheMeta("1.30.14+k0s.0", *meta)

	// Create fake client with necessary schemes
	scheme := runtime.NewScheme()
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apv1b2.AddToScheme(scheme))

	// Create test nodes
	nodes := createTestNodes()
	initialObjects := make([]client.Object, len(nodes))
	for i, node := range nodes {
		initialObjects[i] = &node
	}

	// Add registry secret for airgap
	registrySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-creds",
			Namespace: "kotsadm",
		},
		Data: map[string][]byte{
			"username": []byte("test-user"),
			"password": []byte("test-password"),
		},
	}
	initialObjects = append(initialObjects, registrySecret)

	baseCli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&ecv1beta1.Installation{}).
		WithObjects(initialObjects...).
		Build()

	// Wrap with mock client that simulates successful job completion and autopilot plan success
	mockCli := &mockClientWithStateChange{
		Client:         baseCli,
		callCount:      0,
		callsUntil:     1,                               // Change state after first call
		finalJobStatus: batchv1.JobStatus{Succeeded: 1}, // Mark jobs as succeeded
		finalPlanState: core.PlanCompleted,
	}

	// Create upgrader
	upgrader := NewInfraUpgrader(
		WithKubeClient(mockCli),
		WithRuntimeConfig(runtimeconfig.New(&ecv1beta1.RuntimeConfigSpec{})),
		WithLogger(logger),
	)

	// Create test installation
	installation := createTestInstallation()
	installation.Spec.AirGap = true
	installation.Spec.Artifacts = &ecv1beta1.ArtifactsLocation{
		Images:                  "test-images-url",
		HelmCharts:              "test-helmcharts-url",
		EmbeddedClusterBinary:   "test-binary-url",
		EmbeddedClusterMetadata: "test-metadata-url",
	}

	// Execute the function
	err := upgrader.DistributeArtifacts(ctx, installation, "test-mirror:latest", "test-license", "test-app", "test-channel", "1.0.0")

	// The function should succeed as our mock client simulates successful job completion and autopilot plan success
	require.NoError(t, err)
}

func TestDistributeArtifacts_AirgapAutopilotPlanFailure(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()

	// Set up fast polling for tests
	t.Setenv("AUTOPILOT_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_MAX_STEPS", "10")
	t.Setenv("AUTOPILOT_NODE_POLL_INTERVAL", "100ms")
	t.Setenv("AUTOPILOT_NODE_MAX_STEPS", "10")

	// Create fake client with necessary schemes
	scheme := runtime.NewScheme()
	require.NoError(t, ecv1beta1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, apv1b2.AddToScheme(scheme))

	// Create test nodes
	nodes := createTestNodes()
	initialObjects := make([]client.Object, len(nodes))
	for i, node := range nodes {
		initialObjects[i] = &node
	}

	// Add registry secret for airgap
	registrySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "registry-creds",
			Namespace: "kotsadm",
		},
		Data: map[string][]byte{
			"username": []byte("test-user"),
			"password": []byte("test-password"),
		},
	}
	initialObjects = append(initialObjects, registrySecret)

	// Add failed autopilot plan
	autopilotPlan := createFailedAutopilotPlan()
	initialObjects = append(initialObjects, autopilotPlan)

	baseCli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&ecv1beta1.Installation{}).
		WithObjects(initialObjects...).
		Build()

	// Wrap with mock client that simulates successful job completion but failed autopilot plan
	mockCli := &mockClientWithStateChange{
		Client:         baseCli,
		callCount:      0,
		callsUntil:     1,                               // Change state after first call
		finalJobStatus: batchv1.JobStatus{Succeeded: 1}, // Mark jobs as succeeded
		finalPlanState: core.PlanApplyFailed,            // Autopilot plan fails
	}

	// Create upgrader
	upgrader := NewInfraUpgrader(
		WithKubeClient(mockCli),
		WithRuntimeConfig(runtimeconfig.New(&ecv1beta1.RuntimeConfigSpec{})),
		WithLogger(logger),
	)

	// Create test installation
	installation := createTestInstallation()
	installation.Spec.AirGap = true
	installation.Spec.Artifacts = &ecv1beta1.ArtifactsLocation{
		Images:                  "test-images-url",
		HelmCharts:              "test-helmcharts-url",
		EmbeddedClusterBinary:   "test-binary-url",
		EmbeddedClusterMetadata: "test-metadata-url",
	}

	// Execute the function
	err := upgrader.DistributeArtifacts(ctx, installation, "test-mirror:latest", "test-license", "test-app", "test-channel", "1.0.0")

	// Verify error due to failed autopilot plan
	require.Error(t, err)
	assert.Contains(t, err.Error(), "autopilot plan failed")
}
