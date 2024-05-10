package e2e

import (
	"os"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

func TestSingleNodeDisasterRecovery(t *testing.T) {
	t.Parallel()

	requiredEnvVars := []string{
		"DR_AWS_S3_ENDPOINT",
		"DR_AWS_S3_REGION",
		"DR_AWS_S3_BUCKET",
		"DR_AWS_S3_PREFIX",
		"DR_AWS_ACCESS_KEY_ID",
		"DR_AWS_SECRET_ACCESS_KEY",
	}
	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			t.Fatalf("missing required environment variable: %s", envVar)
		}
	}

	testArgs := []string{}
	for _, envVar := range requiredEnvVars {
		testArgs = append(testArgs, os.Getenv(envVar))
	}

	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "ubuntu/jammy",
		LicensePath:         "snapshot-license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer cleanupCluster(t, tc)

	t.Logf("%s: installing test dependencies on node 0", time.Now().Format(time.RFC3339))
	commands := [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "expect", "-y"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail to install test dependencies on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install.sh", "ui"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if err := setupPlaywright(t, tc); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := runPlaywrightTest(t, tc, "create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v", err)
	}

	t.Logf("%s: resetting the installation", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to reset the installation: %v", err)
	}

	t.Logf("%s: restoring the installation", time.Now().Format(time.RFC3339))
	line = append([]string{"restore-installation.exp"}, testArgs...)
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to restore the installation: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeResumeDisasterRecovery(t *testing.T) {
	t.Parallel()

	requiredEnvVars := []string{
		"DR_AWS_S3_ENDPOINT",
		"DR_AWS_S3_REGION",
		"DR_AWS_S3_BUCKET",
		"DR_AWS_S3_PREFIX",
		"DR_AWS_ACCESS_KEY_ID",
		"DR_AWS_SECRET_ACCESS_KEY",
	}
	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			t.Fatalf("missing required environment variable: %s", envVar)
		}
	}

	testArgs := []string{}
	for _, envVar := range requiredEnvVars {
		testArgs = append(testArgs, os.Getenv(envVar))
	}

	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "ubuntu/jammy",
		LicensePath:         "snapshot-license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer cleanupCluster(t, tc)

	t.Logf("%s: installing test dependencies on node 0", time.Now().Format(time.RFC3339))
	commands := [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "expect", "-y"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail to install test dependencies on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install.sh", "ui"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if err := setupPlaywright(t, tc); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := runPlaywrightTest(t, tc, "create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v", err)
	}

	t.Logf("%s: resetting the installation", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to reset the installation: %v", err)
	}

	t.Logf("%s: restoring the installation", time.Now().Format(time.RFC3339))
	line = append([]string{"resume-restore.exp"}, testArgs...)
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to restore the installation: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
