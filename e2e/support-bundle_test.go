package e2e

import (
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

func TestCollectSupportBundle(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "ubuntu/jammy",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Destroy()
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install.sh"}
	stdout, stderr, err := RunCommandOnNode(t, tc, 0, line)
	if err != nil {
		t.Log("stdout:", stdout)
		t.Log("stderr:", stderr)
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	line = []string{"collect-support-bundle.sh"}
	stdout, stderr, err = RunCommandOnNode(t, tc, 0, line)
	if err != nil {
		t.Log("stdout:", stdout)
		t.Log("stderr:", stderr)
		t.Fatalf("fail to install collect support bundle on node %s: %v", tc.Nodes[0], err)
	}

	t.Log("stdout:", stdout)
	t.Log("stderr:", stderr)
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
