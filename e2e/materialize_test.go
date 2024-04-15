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
		Image:               "j",
		EmbeddedClusterPath: "../output/bin/embedded-cluster-original",
	})
	defer tc.Destroy()

	commands := [][]string{
		{"rm", "-rf", "/var/lib/embedded-cluster/bin/kubectl"},
		{"rm", "-rf", "/var/lib/embedded-cluster/bin/kubectl-preflight"},
		{"rm", "-rf", "/var/lib/embedded-cluster/bin/kubectl-support_bundle"},
		{"embedded-cluster", "materialize"},
		{"ls", "-la", "/var/lib/embedded-cluster/bin/kubectl"},
		{"ls", "-la", "/var/lib/embedded-cluster/bin/kubectl-preflight"},
		{"ls", "-la", "/var/lib/embedded-cluster/bin/kubectl-support_bundle"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail testing materialize assets: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
