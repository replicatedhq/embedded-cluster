package e2e

import (
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

func TestUnsupportedOverrides(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                                 t,
		Nodes:                             1,
		Image:                             "ubuntu/jammy",
		LicensePath:                       "license.yaml",
		EmbeddedClusterPath:               "../output/bin/embedded-cluster",
		EmbeddedClusterReleaseBuilderPath: "../output/bin/embedded-cluster-release-builder",
	})
	defer tc.Destroy()
	t.Logf("%s: installing dependencies on node 0", time.Now().Format(time.RFC3339))
	commands := [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "binutils", "-y"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail to install dependencies on node %s: %v", tc.Nodes[0], err)
	}
	t.Logf("%s: installing embedded-cluster with unsupported overrides on node 0", time.Now().Format(time.RFC3339))
	line := []string{"unsupported-overrides.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
