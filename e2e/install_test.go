package e2e

import (
	"strings"
	"testing"

	"github.com/replicatedhq/helmvm/e2e/cluster"
)

func TestTokenBasedMultiNodeInstallation(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:             t,
		Nodes:         3,
		Image:         "ubuntu/jammy",
		SSHPublicKey:  "../output/tmp/id_rsa.pub",
		SSHPrivateKey: "../output/tmp/id_rsa",
		HelmVMPath:    "../output/bin/helmvm",
	})
	defer tc.Destroy()
	t.Log("installing ssh on node 0")
	line := []string{"apt", "install", "openssh-server", "-y"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install ssh on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("installing helmvm on node 0")
	line = []string{"single-node-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install helmvm on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("generating token on node 0")
	line = []string{"helmvm", "node", "token", "create", "--role", "controller", "--no-prompt"}
	stdout, _, err := RunCommandOnNode(t, tc, 0, line)
	if err != nil {
		t.Fatalf("fail to generate token on node %s: %v", tc.Nodes[0], err)
	}
	for i := 1; i <= 2; i++ {
		t.Logf("joining node %d to the cluster", i)
		join := strings.Split(stdout, " ")
		if _, _, err := RunCommandOnNode(t, tc, i, join); err != nil {
			t.Fatalf("fail to join node %d: %v", i, err)
		}
	}
	t.Log("waiting for cluster nodes to report ready")
	line = []string{"wait-for-ready-nodes.sh", "3"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("nodes not reporting ready: %v", err)
	}
}

func TestSingleNodeInstallation(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:             t,
		Nodes:         1,
		Image:         "ubuntu/jammy",
		SSHPublicKey:  "../output/tmp/id_rsa.pub",
		SSHPrivateKey: "../output/tmp/id_rsa",
		HelmVMPath:    "../output/bin/helmvm",
	})
	defer tc.Destroy()
	t.Log("installing ssh on node 0")
	line := []string{"apt", "install", "openssh-server", "-y"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install ssh on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("installing helmvm on node 0")
	line = []string{"single-node-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install helmvm on node %s: %v", tc.Nodes[0], err)
	}
}

func TestMultiNodeInstallation(t *testing.T) {
	t.Parallel()
	t.Log("creating cluster")
	tc := cluster.NewTestCluster(&cluster.Input{
		T:             t,
		Nodes:         3,
		Image:         "ubuntu/jammy",
		SSHPublicKey:  "../output/tmp/id_rsa.pub",
		SSHPrivateKey: "../output/tmp/id_rsa",
		HelmVMPath:    "../output/bin/helmvm",
	})
	defer tc.Destroy()
	for i := range tc.Nodes {
		t.Logf("installing ssh on node %d", i)
		line := []string{"apt", "install", "openssh-server", "-y"}
		if _, _, err := RunCommandOnNode(t, tc, i, line); err != nil {
			t.Fatalf("fail to install ssh on node %d: %v", i, err)
		}
	}
	t.Log("running multi node helmvm install from node 0")
	line := []string{"multi-node-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install helmvm from node 00: %v", err)
	}
}
