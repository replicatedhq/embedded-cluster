package e2e

import (
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

func TestMaterialize(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "ubuntu/jammy",
		EmbeddedClusterPath: "../output/bin/embedded-cluster-original",
	})
	defer tc.Destroy()

	commands := [][]string{
		{"mkdir", "/tmp/home"},
		{"embedded-cluster", "materialize", "/tmp/home"},
		{"ls", "-la", "/tmp/home/bin/kubectl-preflight"},
		{"ls", "-la", "/tmp/home/bin/kubectl"},
		{"ls", "-la", "/tmp/home/bin/k0s"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail testing materialize assets: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
