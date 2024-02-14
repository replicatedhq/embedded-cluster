package e2e

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

// This test creates 4 nodes, installs on the first one and then generate 2 join tokens
// for controllers and one join token for worker nodes. Joins the nodes and then waits
// for them to report ready and resets two of the nodes.
func TestMultiNodeReset(t *testing.T) {
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               4,
		Image:               "ubuntu/jammy",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Destroy()
	// bootstrap the first node and makes sure it is healthy. also executes the kots
	// ssl certificate configuration (kurl-proxy).
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := RunCommandOnNode(t, tc, 0, []string{"single-node-install.sh"}); err != nil {
		t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	runPuppeteerAppStatusCheck(t, 0, tc)

	// generate all node join commands (2 for controllers and 1 for worker).
	t.Logf("%s: generating two new controller token commands", time.Now().Format(time.RFC3339))
	controllerCommands := []string{}
	for i := 0; i < 2; i++ {
		line := []string{"puppeteer.sh", "generate-controller-join-token.js", "10.0.0.2"}
		stdout, stderr, err := RunCommandOnNode(t, tc, 0, line)
		if err != nil {
			t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
			t.Fatalf("fail to generate controller join token: %s", stdout)
		}
		var r nodeJoinResponse
		if err := json.Unmarshal([]byte(stdout), &r); err != nil {
			t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
			t.Fatalf("fail to parse script response: %v", err)
		}
		// trim down the "./" and the "sudo" command as those are not needed. we run as
		// root and the embedded-cluster binary is on the PATH.
		command := strings.TrimPrefix(r.Command, "sudo ./")
		controllerCommands = append(controllerCommands, command)
		t.Log("controller join token command:", command)
	}
	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	line := []string{"puppeteer.sh", "generate-worker-join-token.js", "10.0.0.2"}
	stdout, stderr, err := RunCommandOnNode(t, tc, 0, line)
	if err != nil {
		t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
		t.Fatalf("fail to generate controller join token: %s", stdout)
	}
	var jr nodeJoinResponse
	if err := json.Unmarshal([]byte(stdout), &jr); err != nil {
		t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
		t.Fatalf("fail to parse script response: %v", err)
	}

	// join the nodes.
	for i, cmd := range controllerCommands {
		node := i + 1
		t.Logf("%s: joining node %d to the cluster (controller)", time.Now().Format(time.RFC3339), node)
		stdout, stderr, err := RunCommandOnNode(t, tc, node, strings.Split(cmd, " "))
		if err != nil {
			t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
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
	command := strings.TrimPrefix(jr.Command, "sudo ./")
	t.Log("worker join token command:", command)
	t.Logf("%s: joining node 3 to the cluster as a worker", time.Now().Format(time.RFC3339))
	stdout, stderr, err = RunCommandOnNode(t, tc, 3, strings.Split(command, " "))
	if err != nil {
		t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
		t.Fatalf("fail to join node 3 to the cluster as a worker: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, stderr, err = RunCommandOnNode(t, tc, 0, []string{"wait-for-ready-nodes.sh", "4"})
	if err != nil {
		t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	t.Log(stdout)

	bin := strings.Split(command, " ")[0]
	// reset worker node
	t.Logf("%s: resetting worker node", time.Now().Format(time.RFC3339))
	stdout, stderr, err = RunCommandOnNode(t, tc, 3, []string{bin, "node", "reset", "--no-prompt"})
	if err != nil {
		t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
		t.Fatalf("fail to reset worker node")
	}
	t.Log(stdout)

	// reset a controller node
	// this should fail with a prompt to override
	t.Logf("%s: resetting controller node", time.Now().Format(time.RFC3339))
	stdout, stderr, err = RunCommandOnNode(t, tc, 2, []string{bin, "node", "reset", "--no-prompt"})
	if err != nil {
		t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
		t.Fatalf("fail to remove controller node %s:", err)
	}
	t.Log(stdout)

	stdout, stderr, err = RunCommandOnNode(t, tc, 0, []string{"check-nodes-removed.sh", "2"})
	if err != nil {
		t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
		t.Fatalf("fail to remove worker node %s:", err)
	}
	t.Log(stdout)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
