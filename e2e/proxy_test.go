package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

// TestProxiedEnvironment tests the installation behind a proxy server
func TestProxiedEnvironment(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               4,
		WithProxy:           true,
		Image:               "debian/12",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer cleanupCluster(t, tc)
	t.Log("Proxied infrastructure created")

	// bootstrap the first node and makes sure it is healthy. also executes the kots
	// ssl certificate configuration (kurl-proxy).
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-proxy-install.sh"}
	line = append(line, "--http-proxy", cluster.HTTPProxy)
	line = append(line, "--https-proxy", cluster.HTTPProxy)
	line = append(line, "--no-proxy", cluster.NOProxy)
	withEnv := WithEnv(map[string]string{
		"HTTP_PROXY":  cluster.HTTPProxy,
		"HTTPS_PROXY": cluster.HTTPProxy,
		"NO_PROXY":    cluster.NOProxy,
	})
	if _, _, err := RunCommandOnNode(t, tc, 0, line, withEnv); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if err := setupPlaywright(t, tc); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := runPlaywrightTest(t, tc, "deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	// generate all node join commands (2 for controllers and 1 for worker).
	t.Logf("%s: generating two new controller token commands", time.Now().Format(time.RFC3339))
	controllerCommands := []string{}
	for i := 0; i < 2; i++ {
		stdout, stderr, err := runPlaywrightTest(t, tc, "get-join-controller-command")
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
	stdout, stderr, err := runPlaywrightTest(t, tc, "get-join-worker-command")
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
		if _, _, err := RunCommandOnNode(t, tc, node, strings.Split(cmd, " ")); err != nil {
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
	if _, _, err := RunCommandOnNode(t, tc, 3, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 3 to the cluster as a worker: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = RunCommandOnNode(t, tc, 0, []string{"wait-for-ready-nodes.sh", "4"})
	if err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	t.Log(stdout)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// TestCustomCIDR tests the installation with an alternate CIDR range
func TestCustomCIDR(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               3,
		Image:               "debian/12",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer cleanupCluster(t, tc)
	t.Log("Proxied infrastructure created")

	// bootstrap the first node and makes sure it is healthy. also executes the kots
	// ssl certificate configuration (kurl-proxy).
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-proxy-install.sh"}
	line = append(line, "--pod-cidr", "10.128.0.0/20")
	line = append(line, "--service-cidr", "10.129.0.0/20")
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if err := setupPlaywright(t, tc); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := runPlaywrightTest(t, tc, "deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	// generate all node join commands (1 for controller and 1 for worker).
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := runPlaywrightTest(t, tc, "get-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	controllerCommand := command
	t.Log("controller join token command:", command)

	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err = runPlaywrightTest(t, tc, "get-join-worker-command")
	if err != nil {
		t.Fatalf("fail to generate worker join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err = findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("worker join token command:", command)

	// join the nodes.
	t.Logf("%s: joining node %d to the cluster (controller)", time.Now().Format(time.RFC3339), 1)
	if _, _, err := RunCommandOnNode(t, tc, 1, strings.Split(controllerCommand, " ")); err != nil {
		t.Fatalf("fail to join node %d as a controller: %v", 1, err)
	}
	// XXX If we are too aggressive joining nodes we can see the following error being
	// thrown by kotsadm on its log (and we get a 500 back):
	// "
	// failed to get controller role name: failed to get cluster config: failed to get
	// current installation: failed to list installations: etcdserver: leader changed
	// "
	t.Logf("node %d joined, sleeping...", 1)
	time.Sleep(30 * time.Second)

	t.Logf("%s: joining node 2 to the cluster as a worker", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 2, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 2 to the cluster as a worker: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = RunCommandOnNode(t, tc, 0, []string{"wait-for-ready-nodes.sh", "3"})
	if err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	t.Log(stdout)

	// ensure that the cluster is using the right IP ranges.
	t.Logf("%s: checking service and pod IP addresses", time.Now().Format(time.RFC3339))
	stdout, _, err = RunCommandOnNode(t, tc, 0, []string{"check-cidr-ranges.sh", "10.128.0.0/20", "10.129.0.0/20"})
	if err != nil {
		t.Fatalf("fail to check addresses on node %s: %v", tc.Nodes[0], err)
	}
	t.Log(stdout)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
