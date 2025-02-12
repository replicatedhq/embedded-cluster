package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/lxd"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	"github.com/stretchr/testify/require"
)

// SkipProxyTest returns true if the k0s version in use does not support
// proxied environments.
func SkipProxyTest() bool {
	supportedVersion := semver.MustParse("1.29.0")
	currentVersion := semver.MustParse(versions.K0sVersion)
	return currentVersion.LessThan(supportedVersion)
}

// TestProxiedEnvironment tests the installation behind a proxy server
func TestProxiedEnvironment(t *testing.T) {
	t.Parallel()
	if SkipProxyTest() {
		t.Skip("skipping test for k0s versions < 1.29.0")
	}

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                   t,
		Nodes:               4,
		WithProxy:           true,
		Image:               "debian/12",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()
	t.Log("Proxied infrastructure created")

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	t.Logf("%s: reconfiguring squid to only allow whitelist access", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunCommandOnProxyNode(t, []string{"/usr/local/bin/reconfigure-squid.sh"}); err != nil {
		t.Fatalf("failed to reconfigure squid: %v", err)
	}

	// bootstrap the first node and makes sure it is healthy. also executes the kots
	// ssl certificate configuration (kurl-proxy).
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install.sh", "ui", os.Getenv("SHORT_SHA")}
	line = append(line, "--http-proxy", lxd.HTTPProxy)
	line = append(line, "--https-proxy", lxd.HTTPProxy)
	if _, _, err := tc.RunCommandOnNode(0, line, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	// generate all node join commands (2 for controllers and 1 for worker).
	t.Logf("%s: generating two new controller token commands", time.Now().Format(time.RFC3339))
	controllerCommands := []string{}
	for i := 0; i < 2; i++ {
		stdout, stderr, err := tc.RunPlaywrightTest("get-join-controller-command")
		if err != nil {
			t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
		}
		command, err := findJoinCommandInOutput(stdout)
		if err != nil {
			t.Fatalf("fail to find the join command in the output: %v", err)
		}
		controllerCommands = append(controllerCommands, command)
		t.Log("controller join token command:", command)
	}
	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunPlaywrightTest("get-join-worker-command")
	if err != nil {
		t.Fatalf("fail to generate worker join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("worker join token command:", command)

	// join the nodes.
	for i, cmd := range controllerCommands {
		node := i + 1
		t.Logf("%s: joining node %d to the cluster (controller)", time.Now().Format(time.RFC3339), node)
		if _, _, err := tc.RunCommandOnNode(node, strings.Split(cmd, " ")); err != nil {
			t.Fatalf("fail to join node %d as a controller: %v", node, err)
		}
		// XXX If we are too aggressive joining nodes we can see the following error being
		// thrown by kotsadm on its log (and we get a 500 back):
		// "
		// failed to get controller role name: failed to get cluster config: failed to get
		// current installation: failed to list installations: etcdserver: leader changed
		// "
		t.Logf("node %d joined, sleeping...", node)
		time.Sleep(30 * time.Second)
	}
	t.Logf("%s: joining node 3 to the cluster as a worker", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunCommandOnNode(3, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 3 to the cluster as a worker: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = tc.RunCommandOnNode(0, []string{"wait-for-ready-nodes.sh", "4"})
	if err != nil {
		t.Log(stdout)
		t.Fatalf("fail to wait for ready nodes: %v", err)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA"), k8sVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// TestProxiedCustomCIDR tests the installation behind a proxy server while using a custom pod and service CIDR
func TestProxiedCustomCIDR(t *testing.T) {
	t.Parallel()
	if SkipProxyTest() {
		t.Skip("skipping test for k0s versions < 1.29.0")
	}

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                   t,
		Nodes:               4,
		WithProxy:           true,
		Image:               "debian/12",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()
	t.Log("Proxied infrastructure created")

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	// bootstrap the first node and makes sure it is healthy. also executes the kots
	// ssl certificate configuration (kurl-proxy).
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install.sh", "ui", os.Getenv("SHORT_SHA")}
	line = append(line, "--http-proxy", lxd.HTTPProxy)
	line = append(line, "--https-proxy", lxd.HTTPProxy)
	line = append(line, "--no-proxy", strings.Join(tc.IPs, ","))
	line = append(line, "--pod-cidr", "10.128.0.0/20")
	line = append(line, "--service-cidr", "10.129.0.0/20")
	if _, _, err := tc.RunCommandOnNode(0, line, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	// generate all node join commands (2 for controllers and 1 for worker).
	t.Logf("%s: generating two new controller token commands", time.Now().Format(time.RFC3339))
	controllerCommands := []string{}
	for i := 0; i < 2; i++ {
		stdout, stderr, err := tc.RunPlaywrightTest("get-join-controller-command")
		if err != nil {
			t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
		}
		command, err := findJoinCommandInOutput(stdout)
		if err != nil {
			t.Fatalf("fail to find the join command in the output: %v", err)
		}
		controllerCommands = append(controllerCommands, command)
		t.Log("controller join token command:", command)
	}
	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunPlaywrightTest("get-join-worker-command")
	if err != nil {
		t.Fatalf("fail to generate worker join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("worker join token command:", command)

	// join the nodes.
	for i, cmd := range controllerCommands {
		node := i + 1
		t.Logf("%s: joining node %d to the cluster (controller)", time.Now().Format(time.RFC3339), node)
		if _, _, err := tc.RunCommandOnNode(node, strings.Split(cmd, " ")); err != nil {
			t.Fatalf("fail to join node %d as a controller: %v", node, err)
		}
		// XXX If we are too aggressive joining nodes we can see the following error being
		// thrown by kotsadm on its log (and we get a 500 back):
		// "
		// failed to get controller role name: failed to get cluster config: failed to get
		// current installation: failed to list installations: etcdserver: leader changed
		// "
		t.Logf("node %d joined, sleeping...", node)
		time.Sleep(30 * time.Second)
	}
	t.Logf("%s: joining node 3 to the cluster as a worker", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunCommandOnNode(3, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 3 to the cluster as a worker: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = tc.RunCommandOnNode(0, []string{"wait-for-ready-nodes.sh", "4"})
	if err != nil {
		t.Log(stdout)
		t.Fatalf("fail to wait for ready nodes: %v", err)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA"), k8sVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	// ensure that the cluster is using the right IP ranges.
	t.Logf("%s: checking service and pod IP addresses", time.Now().Format(time.RFC3339))
	stdout, _, err = tc.RunCommandOnNode(0, []string{"check-cidr-ranges.sh", "^10.128.[0-9]*.[0-9]", "^10.129.[0-9]*.[0-9]"})
	if err != nil {
		t.Log(stdout)
		t.Fatalf("fail to check addresses on node %s: %v", tc.Nodes[0], err)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestInstallWithMITMProxy(t *testing.T) {
	if SkipProxyTest() {
		t.Skip("skipping test for k0s versions < 1.29.0")
	}

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                   t,
		Nodes:               4,
		WithProxy:           true,
		Image:               "debian/12",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
		LicensePath:         "license.yaml",
	})
	defer tc.Cleanup()

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	// bootstrap the first node and makes sure it is healthy. also executes the kots
	// ssl certificate configuration (kurl-proxy).
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install.sh", "ui", os.Getenv("SHORT_SHA")}
	line = append(line, "--http-proxy", lxd.HTTPMITMProxy)
	line = append(line, "--https-proxy", lxd.HTTPMITMProxy)
	line = append(line, "--private-ca", "/usr/local/share/ca-certificates/proxy/ca.crt")
	_, _, err := tc.RunCommandOnNode(0, line, lxd.WithMITMProxyEnv(tc.IPs))
	require.NoError(t, err, "failed to install embedded-cluster on node 0")

	_, _, err = tc.SetupPlaywrightAndRunTest("deploy-app")
	require.NoError(t, err, "failed to deploy app")

	// generate all node join commands (2 for controllers and 1 for worker).
	t.Logf("%s: generating two new controller token commands", time.Now().Format(time.RFC3339))
	controllerCommands := []string{}
	for i := 0; i < 2; i++ {
		stdout, _, err := tc.RunPlaywrightTest("get-join-controller-command")
		require.NoError(t, err, "failed to generate controller join token")

		command, err := findJoinCommandInOutput(stdout)
		require.NoError(t, err, "failed to find the join command in the output")

		controllerCommands = append(controllerCommands, command)
		t.Log("controller join token command:", command)
	}

	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	stdout, _, err := tc.RunPlaywrightTest("get-join-worker-command")
	require.NoError(t, err, "failed to generate worker join token")

	command, err := findJoinCommandInOutput(stdout)
	require.NoError(t, err, "failed to find the join command in the output")
	t.Log("worker join token command:", command)

	// join the nodes.
	for i, cmd := range controllerCommands {
		node := i + 1
		t.Logf("%s: joining node %d to the cluster (controller)", time.Now().Format(time.RFC3339), node)
		_, _, err := tc.RunCommandOnNode(node, strings.Split(cmd, " "))
		require.NoError(t, err, "failed to join node as a controller")

		// XXX If we are too aggressive joining nodes we can see the following error being
		// thrown by kotsadm on its log (and we get a 500 back):
		// "
		// failed to get controller role name: failed to get cluster config: failed to get
		// current installation: failed to list installations: etcdserver: leader changed
		// "
		t.Logf("node %d joined, sleeping...", node)
		time.Sleep(30 * time.Second)
	}

	t.Logf("%s: joining node 3 to the cluster as a worker", time.Now().Format(time.RFC3339))
	_, _, err = tc.RunCommandOnNode(3, strings.Split(command, " "))
	require.NoError(t, err, "failed to join node 3 to the cluster as a worker")

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	_, _, err = tc.RunCommandOnNode(0, []string{"wait-for-ready-nodes.sh", "4"})
	require.NoError(t, err, "failed to wait for nodes to be ready")

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA"), k8sVersion()}
	_, _, err = tc.RunCommandOnNode(0, line)
	require.NoError(t, err, "failed to check installation state")

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
