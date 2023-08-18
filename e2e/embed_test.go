package e2e

import (
	"testing"

	"github.com/replicatedhq/helmvm/e2e/cluster"
)

func TestEmbedAndInstall(t *testing.T) {
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
	t.Log("installing ssh in node 0")
	line := []string{"apt", "install", "openssh-server", "-y"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install ssh on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("pulling helm chart, embedding, and installing on node 0")
	line = []string{"embed-and-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded ssh in node 0: %v", err)
	}
}

func TestInstallSingleNodeAndUpgradeToEmbed(t *testing.T) {
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
	t.Log("installing ssh in node 0")
	line := []string{"apt", "install", "openssh-server", "-y"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install ssh on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("installing helmvm on node 0")
	line = []string{"single-node-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install helmvm on node 0: %v", err)
	}
	t.Log("installing helmvm embed with memcached on node 0")
	line = []string{"embed-and-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embed helmvm on node 0: %v", err)
	}
}
