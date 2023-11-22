package e2e

import (
	"os"
	"testing"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

func TestBuilder(t *testing.T) {
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
	commands := [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "openssh-server", "curl", "git", "-y"},
	}
	t.Log("installing test dependencies on node 0")
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("failed to install test dependencies on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("installing embedded-cluster on node 0")
	line := []string{"single-node-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	t.Log("deploying minio on the embedded cluster")
	line = []string{"deploy-minio.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to deploy minio: %v", err)
	}
	t.Log("deploying embedded cluster builder")
	line = []string{"deploy-builder.sh", os.Getenv("BUILDER_IMAGE")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to deploy embedded cluster builder: %v", err)
	}
}
