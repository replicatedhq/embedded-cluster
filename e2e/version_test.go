package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/replicatedhq/helmvm/e2e/cluster"
)

func TestVersion(t *testing.T) {
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
	t.Log("validating helmvm version in node 0")
	line := []string{"helmvm", "version"}
	stdout, stderr, err := RunCommandOnNode(t, tc, 0, line)
	if err != nil {
		t.Fatalf("fail to install ssh on node %s: %v", tc.Nodes[0], err)
	}
	var failed bool
	output := fmt.Sprintf("%s\n%s", stdout, stderr)
	expected := []string{"Installer", "Kubernetes", "OpenEBS", "AdminConsole"}
	for _, component := range expected {
		if strings.Contains(output, component) {
			continue
		}
		t.Errorf("missing %q version in 'version' output", component)
		failed = true
	}
	if failed {
		t.Log(output)
	}
}
