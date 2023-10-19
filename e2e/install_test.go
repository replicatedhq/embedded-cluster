package e2e

import (
	"testing"

	"github.com/replicatedhq/helmvm/e2e/cluster"
)

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
	commands := [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "openssh-server", "-y"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail to install ssh on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("installing helmvm on node 0")
	line := []string{"single-node-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install helmvm on node %s: %v", tc.Nodes[0], err)
	}
}

func TestSingleNodeInstallationRockyLinux8(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:             t,
		Nodes:         1,
		Image:         "rockylinux/8",
		SSHPublicKey:  "../output/tmp/id_rsa.pub",
		SSHPrivateKey: "../output/tmp/id_rsa",
		HelmVMPath:    "../output/bin/helmvm",
	})
	defer tc.Destroy()
	t.Log("installing ssh on node 0")
	commands := [][]string{
		{"dnf", "install", "-y", "openssh-server"},
		{"systemctl", "enable", "sshd"},
		{"systemctl", "start", "sshd"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail to install ssh on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("installing helmvm on node 0")
	line := []string{"single-node-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install helmvm on node %s: %v", tc.Nodes[0], err)
	}
}

func TestSingleNodeInstallationDebian12(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:             t,
		Nodes:         1,
		Image:         "debian/12",
		SSHPublicKey:  "../output/tmp/id_rsa.pub",
		SSHPrivateKey: "../output/tmp/id_rsa",
		HelmVMPath:    "../output/bin/helmvm",
	})
	defer tc.Destroy()
	t.Log("installing ssh on node 0")
	commands := [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "openssh-server", "-y"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail to install ssh on node 0: %v", err)
	}
	t.Log("installing helmvm on node 0")
	line := []string{"single-node-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install helmvm on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("creating deployment mounting pvc")
}

func TestSingleNodeInstallationCentos8Stream(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:             t,
		Nodes:         1,
		Image:         "centos/8-Stream",
		SSHPublicKey:  "../output/tmp/id_rsa.pub",
		SSHPrivateKey: "../output/tmp/id_rsa",
		HelmVMPath:    "../output/bin/helmvm",
	})
	defer tc.Destroy()
	t.Log("installing ssh on node 0")
	commands := [][]string{
		{"dnf", "install", "-y", "openssh-server"},
		{"systemctl", "enable", "sshd"},
		{"systemctl", "start", "sshd"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail to install ssh on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("installing helmvm on node 0")
	line := []string{"single-node-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install helmvm on node %s: %v", tc.Nodes[0], err)
	}
}

func TestMultiNodeInteractiveInstallation(t *testing.T) {
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
		commands := [][]string{
			{"apt-get", "update", "-y"},
			{"apt-get", "install", "openssh-server", "-y"},
		}
		if err := RunCommandsOnNode(t, tc, i, commands); err != nil {
			t.Fatalf("fail to install ssh on node %d: %v", i, err)
		}
	}
	t.Logf("installing expect on node 0")
	line := []string{"apt-get", "install", "expect", "-y"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install expect on node 0: %v", err)
	}
	t.Log("running multi node interactive install from node 0")
	line = []string{"interactive-multi-node-install.exp"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install helmvm from node 0: %v", err)
	}
	t.Log("waiting for cluster nodes to report ready")
	line = []string{"wait-for-ready-nodes.sh", "3"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("nodes not reporting ready: %v", err)
	}
}

func TestInstallWithDisabledAddons(t *testing.T) {
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
	commands := [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "openssh-server", "-y"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail to install ssh on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("installling with disabled addons on node 0")
	line := []string{"install-with-disabled-addons.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded ssh in node 0: %v", err)
	}
}

func TestHostPreflight(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:             t,
		Nodes:         1,
		Image:         "centos/8-Stream",
		SSHPublicKey:  "../output/tmp/id_rsa.pub",
		SSHPrivateKey: "../output/tmp/id_rsa",
		HelmVMPath:    "../output/bin/helmvm",
	})
	defer tc.Destroy()
	t.Log("installing ssh and binutils on node 0")
	commands := [][]string{
		{"dnf", "--setopt=metadata_expire=120", "install", "-y", "openssh-server", "binutils", "tar"},
		{"systemctl", "enable", "sshd"},
		{"systemctl", "start", "sshd"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail to install ssh on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("installing helmvm on node 0")
	line := []string{"embedded-preflight.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install helmvm on node %s: %v", tc.Nodes[0], err)
	}
}
