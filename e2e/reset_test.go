package e2e

import (
	"os"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
)

// This test creates 4 nodes, installs on the first one and then generate 2 join tokens
// for controllers and one join token for worker nodes. Joins the nodes and then waits
// for them to report ready and resets two of the nodes.
func TestMultiNodeReset(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"K0S_INSTALL_VERSION",
		"EC_BINARY_PATH",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        4,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/license.yaml",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()

	installSingleNodeWithOptions(t, tc, installOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// join a controller node
	joinControllerNode(t, tc, 1)

	// XXX If we are too aggressive joining nodes we can see the following error being
	// thrown by kotsadm on its log (and we get a 500 back):
	// "
	// failed to get controller role name: failed to get cluster config: failed to get
	// current installation: failed to list installations: etcdserver: leader changed
	// "
	t.Logf("node 1 joined, sleeping...")
	time.Sleep(30 * time.Second)

	// join another controller node
	joinControllerNode(t, tc, 2)

	// join a worker node
	joinWorkerNode(t, tc, 3)

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 4, nil)

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	bin := "embedded-cluster"
	// reset worker node
	t.Logf("%s: resetting worker node", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunCommandOnNode(3, []string{bin, "reset", "--yes"})
	if err != nil {
		t.Fatalf("fail to reset worker node 3: %v: %s: %s", err, stdout, stderr)
	}

	// reset a controller node
	// this should fail with a prompt to override
	t.Logf("%s: resetting controller node", time.Now().Format(time.RFC3339))
	stdout, stderr, err = tc.RunCommandOnNode(2, []string{bin, "reset", "--yes"})
	if err != nil {
		t.Fatalf("fail to remove controller node 2: %v: %s: %s", err, stdout, stderr)
	}

	stdout, stderr, err = tc.RunCommandOnNode(0, []string{"check-nodes-removed.sh", "2"})
	if err != nil {
		t.Fatalf("fail to check nodes removed: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
