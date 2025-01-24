package e2e

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/lxd"
	"github.com/stretchr/testify/require"
)

// TODO: Remove this in favor of singleNodeInstallUpgradeTest
func singleNodeInstallTest(t *testing.T, tc cluster.Cluster, additionalArgs []string) {
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install2.sh", "ui", os.Getenv("SHORT_SHA"), "--admin-console-port", "30002"}
	line = append(line, additionalArgs...)
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// TODO: check installation state
}

func singleNodeInstallUpgradeTest(t *testing.T, tc cluster.Cluster, additionalArgs []string) {
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install2.sh", "ui", os.Getenv("SHORT_SHA"), "--admin-console-port", "30002"}
	line = append(line, additionalArgs...)
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// TODO: check installation state

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

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
	singleNodeInstallTest(t, tc, nil)
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

	singleNodeInstallTest(t, tc, nil)
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstall2Debian11(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bullseye",
		LicensePath:  "license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()
	singleNodeInstallTest(t, tc, nil)
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstall2Debian12(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()
	singleNodeInstallTest(t, tc, nil)
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstall2UpgradeUbuntuJammy(t *testing.T) {
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
	singleNodeInstallUpgradeTest(t, tc, nil)
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstall2UpgradeAlmaLinux8(t *testing.T) {
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

	singleNodeInstallUpgradeTest(t, tc, nil)
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstall2UpgradeDebian11(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bullseye",
		LicensePath:  "license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()
	singleNodeInstallUpgradeTest(t, tc, nil)
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstall2UpgradeDebian12(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()
	singleNodeInstallUpgradeTest(t, tc, nil)
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeAirgapInstall2(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	t.Logf("%s: downloading airgap file", time.Now().Format(time.RFC3339))
	airgapBundlePath := "/tmp/airgap-bundle.tar.gz"
	err := downloadAirgapBundle(t, fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")), airgapBundlePath, os.Getenv("AIRGAP_LICENSE_ID"))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%s: creating airgap node", time.Now().Format(time.RFC3339))

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                       t,
		Nodes:                   1,
		Image:                   "debian/12",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapBundlePath,
	})
	defer tc.Cleanup()

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}

	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}

	singleNodeInstallTest(t, tc, []string{"--airgap-bundle", "/assets/release.airgap"})
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeAirgapInstall2Upgrade(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	t.Logf("%s: downloading airgap file", time.Now().Format(time.RFC3339))
	airgapBundlePath := "/tmp/airgap-bundle.tar.gz"
	err := downloadAirgapBundle(t, fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")), airgapBundlePath, os.Getenv("AIRGAP_LICENSE_ID"))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%s: creating airgap node", time.Now().Format(time.RFC3339))

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                       t,
		Nodes:                   1,
		Image:                   "debian/12",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapBundlePath,
	})
	defer tc.Cleanup()

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	if err = tc.RunCommandsOnNode(0, [][]string{{"curl", `"https://kots.io/install"`, "|", "bash"}}, map[string]string{
		"http_proxy":  lxd.HTTPProxy,
		"https_proxy": lxd.HTTPProxy,
	}); err != nil {
		t.Fatalf("failed to install kots on node 0")
	}

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}

	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}

	singleNodeInstallUpgradeTest(t, tc, []string{"--airgap-bundle", "/assets/release.airgap"})
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
