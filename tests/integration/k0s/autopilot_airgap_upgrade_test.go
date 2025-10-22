package k0s

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/upgrade"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util"
)

// TestK0sAutopilotMultiNodeAirgapUpgrade tests k0s upgrade via autopilot in airgap mode.
// This test:
// 1. Bootstraps a 3-node k0s cluster with an old version
// 2. Starts LAM with the new k0s binary
// 3. Blocks internet access to simulate airgap
// 4. Creates Installation CR pointing to LAM
// 5. Uses upgrader.UpgradeK0s() to trigger autopilot upgrade
// 6. Verifies all nodes upgrade successfully
func TestK0sAutopilotMultiNodeAirgapUpgrade(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Configure versions
	oldK0sVersion := os.Getenv("OLD_K0S_VERSION")
	if oldK0sVersion == "" {
		oldK0sVersion = "v1.30.13+k0s.0" // Default old version
	}

	newK0sVersion := os.Getenv("NEW_K0S_VERSION")
	if newK0sVersion == "" {
		newK0sVersion = "v1.30.14+k0s.0" // Default new version
	}

	dataDir := t.TempDir()

	t.Logf("Testing upgrade from %s to %s", oldK0sVersion, newK0sVersion)

	// Step 1: Bootstrap 3-node k0s cluster with old version
	t.Log("Step 1: Bootstrapping 3-node k0s cluster with old version")
	nodeNames := []string{"controller-0", "controller-1", "controller-2"}
	cluster := bootstrapK0sCluster(t, nodeNames, oldK0sVersion, dataDir)
	defer cluster.Cleanup()

	// Verify old version is running
	for _, node := range cluster.nodes {
		assertK0sVersion(t, node, oldK0sVersion)
	}
	assertAllNodesReady(t, cluster, 3)

	// Get kubeconfig and create clients
	kubeconfig := cluster.GetKubeconfig()
	kcli := util.CtrlClient(t, kubeconfig)
	hcli := util.HelmClient(t, kubeconfig)

	// Step 2: Download new k0s version to shared volume
	t.Log("Step 2: Downloading new k0s version to shared volume")
	downloadK0sToSharedVolume(t, cluster.nodes[0], newK0sVersion)

	// Step 3: Set up LAM with new k0s binary on all nodes
	t.Log("Step 3: Setting up LAM with new k0s binary on all nodes")
	for _, node := range cluster.nodes {
		prepareK0sBinaryInLAM(t, node, dataDir, newK0sVersion)
		_ = startLAM(t, node, dataDir)
	}

	// Step 4: Block internet access (simulate airgap)
	t.Log("Step 4: Blocking internet access to simulate airgap")
	blockInternetAccess(t, cluster)

	// Verify airgap is working
	// for _, node := range cluster.nodes {
	// 	assertNoExternalNetworkCalls(t, node)
	// }

	rc := runtimeconfig.New(nil)
	rc.SetLocalArtifactMirrorPort(50000)
	rc.SetDataDir(dataDir)

	// Step 5: Create Installation CR pointing to LAM
	t.Log("Step 5: Creating Installation CR")
	installation := &ecv1beta1.Installation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-installation",
		},
		Spec: ecv1beta1.InstallationSpec{
			Config: &ecv1beta1.ConfigSpec{
				Version: newK0sVersion,
			},
			AirGap:        true,
			RuntimeConfig: rc.Get(),
		},
	}
	err := kcli.Create(ctx, installation)
	require.NoError(t, err)

	// Step 5: Create upgrader and trigger k0s upgrade using production code
	t.Log("Step 5: Creating upgrader")
	upgrader := upgrade.NewInfraUpgrader(
		upgrade.WithKubeClient(kcli),
		upgrade.WithHelmClient(hcli),
		upgrade.WithRuntimeConfig(rc),
	)

	// THIS IS THE KEY - uses actual production k0s autopilot upgrade logic
	t.Log("Step 6: Triggering k0s autopilot upgrade via upgrader.UpgradeK0s()")
	err = upgrader.UpgradeK0s(ctx, installation)
	require.NoError(t, err)

	// Step 7: Verify all nodes upgraded successfully
	t.Log("Step 7: Verifying all nodes upgraded successfully")
	for _, node := range cluster.nodes {
		assertK0sVersion(t, node, newK0sVersion)
	}
	assertAllNodesReady(t, cluster, 3)

	// Step 8: Verify no external network calls
	t.Log("Step 8: Verifying no external network calls")
	for _, node := range cluster.nodes {
		assertNoExternalNetworkCalls(t, node)
	}

	t.Log("✅ K0s autopilot multi-node airgap upgrade test passed!")
}
