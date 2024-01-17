package e2e

import (
	"encoding/json"
	"testing"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

func TestSingleNodeInstallation(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "ubuntu/jammy",
		SSHPublicKey:        "../output/tmp/id_rsa.pub",
		SSHPrivateKey:       "../output/tmp/id_rsa",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
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
	t.Log("installing embedded-cluster on node 0")
	line := []string{"single-node-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("installing puppeteer on node 0")
	line = []string{"install-puppeteer.sh"}
	if stdout, stderr, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Log("stdout:", stdout)
		t.Log("stderr:", stderr)
		t.Fatalf("fail to install puppeteer on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("accessing kotsadm interface and checking app and cluster state")
	line = []string{"puppeteer.sh", "check-app-and-cluster-status.js", "10.0.0.2"}
	stdout, stderr, err := RunCommandOnNode(t, tc, 0, line)
	if err != nil {
		t.Log("stdout:", stdout)
		t.Log("stderr:", stderr)
		t.Fatalf("fail to access kotsadm interface and state: %v", err)
	}
	type response struct {
		App     string `json:"app"`
		Cluster string `json:"cluster"`
	}
	var r response
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		t.Log("stdout:", stdout)
		t.Log("stderr:", stderr)
		t.Fatalf("fail to parse script response: %v", err)
	}
	if r.App != "Ready" || r.Cluster != "Up to date" {
		t.Log("stdout:", stdout)
		t.Log("stderr:", stderr)
		t.Fatalf("cluster or app not ready: %s", stdout)
	}
}

func TestSingleNodeInstallationRockyLinux8(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "rockylinux/8",
		SSHPublicKey:        "../output/tmp/id_rsa.pub",
		SSHPrivateKey:       "../output/tmp/id_rsa",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
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
	t.Log("installing embedded-cluster on node 0")
	line := []string{"single-node-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
}

func TestSingleNodeInstallationDebian12(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "debian/12",
		SSHPublicKey:        "../output/tmp/id_rsa.pub",
		SSHPrivateKey:       "../output/tmp/id_rsa",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
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
	t.Log("installing embedded-cluster on node 0")
	line := []string{"single-node-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("creating deployment mounting pvc")
}

func TestSingleNodeInstallationCentos8Stream(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "centos/8-Stream",
		SSHPublicKey:        "../output/tmp/id_rsa.pub",
		SSHPrivateKey:       "../output/tmp/id_rsa",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
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
	t.Log("installing embedded-cluster on node 0")
	line := []string{"single-node-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
}

// func TestMultiNodeInteractiveInstallation(t *testing.T) {
// 	t.Parallel()
// 	t.Log("creating cluster")
// 	tc := cluster.NewTestCluster(&cluster.Input{
// 		T:                   t,
// 		Nodes:               3,
// 		Image:               "ubuntu/jammy",
// 		SSHPublicKey:        "../output/tmp/id_rsa.pub",
// 		SSHPrivateKey:       "../output/tmp/id_rsa",
// 		EmbeddedClusterPath: "../output/bin/embedded-cluster",
// 	})
// 	defer tc.Destroy()
// 	for i := range tc.Nodes {
// 		t.Logf("installing ssh on node %d", i)
// 		commands := [][]string{
// 			{"apt-get", "update", "-y"},
// 			{"apt-get", "install", "openssh-server", "-y"},
// 		}
// 		if err := RunCommandsOnNode(t, tc, i, commands); err != nil {
// 			t.Fatalf("fail to install ssh on node %d: %v", i, err)
// 		}
// 	}
// 	t.Logf("installing expect on node 0")
// 	line := []string{"apt-get", "install", "expect", "-y"}
// 	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
// 		t.Fatalf("fail to install expect on node 0: %v", err)
// 	}
// 	t.Log("running multi node interactive install from node 0")
// 	line = []string{"interactive-multi-node-install.exp"}
// 	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
// 		t.Fatalf("fail to install embedded-cluster from node 0: %v", err)
// 	}
// 	t.Log("waiting for cluster nodes to report ready")
// 	line = []string{"wait-for-ready-nodes.sh", "3"}
// 	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
// 		t.Fatalf("nodes not reporting ready: %v", err)
// 	}
// 	t.Log("restarting embedded-cluster service in all nodes")
// 	for i := range tc.Nodes {
// 		cmd := []string{"systemctl", "restart", "embedded-cluster"}
// 		if stdout, stderr, err := RunCommandOnNode(t, tc, i, cmd); err != nil {
// 			t.Logf("stdout: %s", stdout)
// 			t.Logf("stderr: %s", stderr)
// 			t.Fatalf("fail to restart service node %d: %v", i, err)
// 		}
// 	}
// }

func TestInstallWithDisabledAddons(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "ubuntu/jammy",
		SSHPublicKey:        "../output/tmp/id_rsa.pub",
		SSHPrivateKey:       "../output/tmp/id_rsa",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
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
		T:                   t,
		Nodes:               1,
		Image:               "centos/8-Stream",
		SSHPublicKey:        "../output/tmp/id_rsa.pub",
		SSHPrivateKey:       "../output/tmp/id_rsa",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
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
	t.Log("installing embedded-cluster on node 0")
	line := []string{"embedded-preflight.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
}
