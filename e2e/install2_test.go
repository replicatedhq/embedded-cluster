package e2e

import (
	"os"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
	"github.com/stretchr/testify/require"
)

func singleNodeInstalUpgradeTest(t *testing.T, tc *docker.Cluster) {
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install2.sh", "ui", os.Getenv("SHORT_SHA"), "--admin-console-port", "30002"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// TODO: check installation state

	//appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	//testArgs := []string{appUpgradeVersion}
	//
	//t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	//if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
	//	t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	//}

	// TODO: check installation state

	t.Logf("%s: resetting admin console password", time.Now().Format(time.RFC3339))
	newPassword := "newpass"
	line = []string{"embedded-cluster", "admin-console", "reset-password", newPassword}
	_, _, err := tc.RunCommandOnNode(0, line)
	require.NoError(t, err, "unable to reset admin console password")

	t.Logf("%s: logging in with the new password", time.Now().Format(time.RFC3339))
	_, _, err = tc.RunPlaywrightTest("login-with-custom-password", newPassword)
	require.NoError(t, err, "unable to login with the new password")
}

func TestSingleNodeInstall2UbuntuJammy(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "ubuntu-jammy",
		LicensePath:  "license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()
	singleNodeInstalUpgradeTest(t, tc)
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstall2AlmaLinux8(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "almalinux-8",
		LicensePath:  "license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	t.Logf("%s: installing tar", time.Now().Format(time.RFC3339))
	line := []string{"yum-install-tar.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	singleNodeInstalUpgradeTest(t, tc)
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
