package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

func TestLocalArtifactMirror(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "debian/12",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster-original",
	})
	defer cleanupCluster(t, tc)

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"default-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	commands := [][]string{
		{"apt-get", "install", "curl", "-y"},
		{"systemctl", "status", "local-artifact-mirror"},
		{"systemctl", "stop", "local-artifact-mirror"},
		{"systemctl", "start", "local-artifact-mirror"},
		{"systemctl", "status", "local-artifact-mirror"},
		{"curl", "-o", "/tmp/kubectl", "127.0.0.1:50000/bin/kubectl"},
		{"chmod", "755", "/tmp/kubectl"},
		{"/tmp/kubectl", "version", "--client"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail testing local artifact mirror: %v", err)
	}

	command := []string{"cp", "/etc/passwd", "/var/lib/embedded-cluster/logs/passwd"}
	if _, _, err := RunCommandOnNode(t, tc, 0, command); err != nil {
		t.Fatalf("fail to copy file: %v", err)
	}

	command = []string{"curl", "-O", "--fail", "127.0.0.1:50000/logs/passwd"}
	t.Logf("running %v", command)
	if _, _, err := RunCommandOnNode(t, tc, 0, command); err == nil {
		t.Fatalf("we should not be able to fetch logs from local artifact mirror")
	}

	command = []string{"curl", "-O", "--fail", "127.0.0.1:50000/../../../etc/passwd"}
	t.Logf("running %v", command)
	if _, _, err := RunCommandOnNode(t, tc, 0, command); err == nil {
		t.Fatalf("we should not be able to fetch paths with ../")
	}

	t.Logf("testing local artifact mirror restart after materialize")
	command = []string{"embedded-cluster", "materialize"}
	if _, _, err := RunCommandOnNode(t, tc, 0, command); err != nil {
		t.Fatalf("fail materialize embedded cluster binaries: %v", err)
	}

	t.Logf("waiting to verify if local artifact mirror has restarted")
	time.Sleep(20 * time.Second)

	command = []string{"journalctl", "-u", "local-artifact-mirror"}
	stdout, _, err := RunCommandOnNode(t, tc, 0, command)
	if err != nil {
		t.Fatalf("fail to get journalctl logs: %v", err)
	}

	expected := []string{
		"Binary changed, sending signal to stop",
		"Scheduled restart job, restart counter is at",
	}
	for _, str := range expected {
		if !strings.Contains(stdout, str) {
			t.Fatalf("expected %q in journalctl logs, got %q", str, stdout)
		}
		t.Logf("found %q in journalctl logs", str)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
