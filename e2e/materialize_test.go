package e2e

import (
	"os"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
)

func TestMaterialize(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"EC_BINARY_PATH",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()

	commands := [][]string{
		{"rm", "-rf", "/var/lib/embedded-cluster/bin/kubectl"},
		{"rm", "-rf", "/var/lib/embedded-cluster/bin/kubectl-preflight"},
		{"rm", "-rf", "/var/lib/embedded-cluster/bin/kubectl-support_bundle"},
		{"rm", "-rf", "/var/lib/embedded-cluster/bin/fio"},
		{"embedded-cluster", "materialize"},
		{"ls", "-la", "/var/lib/embedded-cluster/bin/kubectl"},
		{"ls", "-la", "/var/lib/embedded-cluster/bin/kubectl-preflight"},
		{"ls", "-la", "/var/lib/embedded-cluster/bin/kubectl-support_bundle"},
		{"ls", "-la", "/var/lib/embedded-cluster/bin/fio"},
	}
	for _, cmd := range commands {
		if stdout, stderr, err := tc.RunCommandOnNode(0, cmd); err != nil {
			t.Fatalf("fail to run command %q: %v: %s: %s", cmd, err, stdout, stderr)
		}
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
