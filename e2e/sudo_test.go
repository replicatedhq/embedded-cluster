package e2e

import (
	"strings"
	"testing"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

func TestCommandsRequireSudo(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		CreateRegularUser:   true,
		Image:               "ubuntu/jammy",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Destroy()
	command := []string{"embedded-cluster", "version"}
	if _, _, err := RunRegularUserCommandOnNode(t, tc, 0, command); err != nil {
		t.Errorf("expected no error running `version` as regular user, got %v", err)
	}
	for _, cmd := range [][]string{
		{"embedded-cluster", "node", "join", "https://test", "token"},
		{"embedded-cluster", "node", "reset", "--force"},
		{"embedded-cluster", "shell"},
		{"embedded-cluster", "install", "--no-prompt"},
	} {
		stdout, stderr, err := RunRegularUserCommandOnNode(t, tc, 0, cmd)
		if err == nil {
			t.Fatalf("expected error running `%v` as regular user, got none", cmd)
		}
		if !strings.Contains(stderr, "command must be run as root") {
			t.Logf("stdout:\n%s\nstderr:%s\n", stdout, stderr)
			t.Fatalf("invalid error found running `%v` as regular user", cmd)
		}
	}
}
