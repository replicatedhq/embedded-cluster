package e2e

import (
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

func TestLocalArtifactMirror(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "ubuntu/jammy",
		EmbeddedClusterPath: "../output/bin/embedded-cluster-original",
	})
	defer tc.Destroy()

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"default-install.sh"}
	stdout, stderr, err := RunCommandOnNode(t, tc, 0, line)
	if err != nil {
		t.Log("stdout:", stdout)
		t.Log("stderr:", stderr)
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	commands := [][]string{
		{"apt-get", "install", "curl", "-y"},
		{"systemctl", "status", "local-artifact-mirror"},
		{"systemctl", "stop", "local-artifact-mirror"},
		{"systemctl", "start", "local-artifact-mirror"},
		{"systemctl", "status", "local-artifact-mirror"},
		{"curl", "-o", "/tmp/kubectl-test", "127.0.0.1:50000/bin/kubectl"},
		{"chmod", "755", "/tmp/kubectl-test"},
		{"/tmp/kubectl-test", "version", "--client"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail testing local artifact mirror: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
