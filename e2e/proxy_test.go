package e2e

import (
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

func TestMultiNodeInstallWithProxy(t *testing.T) {
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "ubuntu/jammy",
		WithProxy:           true,
		SSHPublicKey:        "../output/tmp/id_rsa.pub",
		SSHPrivateKey:       "../output/tmp/id_rsa",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Destroy()

	t.Log("done")
	time.Sleep(time.Hour)
	t.Log("Installing squid proxy on node 0")
	command := []string{"install-squid.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, command); err != nil {
		t.Fatalf("failed to install squid: %v", err)
	}
	t.Log("Installing ssh on node 1")
	commands := [][]string{
		{"apt-get", "update", "-y"}, {"apt-get", "install", "openssh-server", "-y"},
	}
	if err := RunCommandsOnNode(t, tc, 1, commands); err != nil {
		t.Fatalf("fail to install ssh on node %s: %v", tc.Nodes[0], err)
	}
	for i := 1; i < len(tc.Nodes); i++ {
		t.Logf("reseting the default gw of node %d", i)
		commands := [][]string{
			{"ip", "route", "del", "default"},
			{"ip", "route", "add", "default", "via", "10.0.0.2"},
		}
		if err := RunCommandsOnNode(t, tc, i, commands); err != nil {
			t.Fatalf("fail to reset default gw on node %d: %v", i, err)
		}
	}
	t.Log("done")
	time.Sleep(time.Hour)
}
