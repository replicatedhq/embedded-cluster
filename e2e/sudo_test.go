package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

func TestCommandsRequireSudo(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		CreateRegularUser:   true,
		Image:               "debian/12",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Destroy()
	t.Logf(`%s: running "embedded-cluster version" as regular user`, time.Now().Format(time.RFC3339))
	command := []string{"embedded-cluster", "version"}
	if _, _, err := RunRegularUserCommandOnNode(t, tc, 0, command); err != nil {
		t.Errorf("expected no error running `version` as regular user, got %v", err)
	}
	for _, cmd := range [][]string{
		{"embedded-cluster", "node", "join", "https://test", "token"},
		{"embedded-cluster", "join", "https://test", "token"},
		{"embedded-cluster", "reset", "--force"},
		{"embedded-cluster", "node", "reset", "--force"},
		{"embedded-cluster", "shell"},
		{"embedded-cluster", "install", "--no-prompt"},
		{"embedded-cluster", "restore"},
	} {
		t.Logf("%s: running %q as regular user", time.Now().Format(time.RFC3339), strings.Join(cmd, "_"))
		stdout, stderr, err := RunRegularUserCommandOnNode(t, tc, 0, cmd)
		if err == nil {
			t.Fatalf("expected error running `%v` as regular user, got none", cmd)
		}
		if !strings.Contains(stderr, "command must be run as root") {
			t.Logf("stdout:\n%s\nstderr:%s\n", stdout, stderr)
			t.Fatalf("invalid error found running `%v` as regular user", cmd)
		}
	}
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
