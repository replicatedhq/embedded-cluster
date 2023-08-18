package e2e

import (
	"testing"

	"github.com/replicatedhq/helmvm/e2e/cluster"
)

func TestBuildBundle(t *testing.T) {
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
	t.Log("building helmvm bundle in node 0")
	line := []string{"build-bundle.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install ssh on node %s: %v", tc.Nodes[0], err)
	}
}
