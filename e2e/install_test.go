package e2e

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/cmx"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
)

func TestSingleNodeInstallation(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
		"EC_BINARY_PATH",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "ubuntu-jammy",
		LicensePath:  "licenses/multi-node-disabled-license.yaml",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()

	installSingleNodeWithOptions(t, tc, installOptions{
		version:          os.Getenv("APP_INSTALL_VERSION"),
		adminConsolePort: "30002",
	})

	isMultiNodeEnabled := "false"
	testArgs := []string{isMultiNodeEnabled}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	checkNodeJoinCommand(t, tc, 0)

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	t.Logf("%s: resetting admin console password", time.Now().Format(time.RFC3339))
	newPassword := "newpass"
	line := []string{"embedded-cluster", "admin-console", "reset-password", newPassword}
	_, _, err := tc.RunCommandOnNode(0, line)
	require.NoError(t, err, "unable to reset admin console password")

	t.Logf("%s: logging in with the new password", time.Now().Format(time.RFC3339))
	_, _, err = tc.RunPlaywrightTest("login-with-custom-password", newPassword)
	require.NoError(t, err, "unable to login with the new password")

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstallationAlmaLinux8(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
		"EC_BINARY_PATH",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "almalinux-8",
		LicensePath:  "licenses/multi-node-disabled-license.yaml",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()

	t.Logf("%s: installing tar", time.Now().Format(time.RFC3339))
	line := []string{"yum-install-tar.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: configuring firewalld", time.Now().Format(time.RFC3339))
	line = []string{"firewalld-configure.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to configure firewalld: %v: %s: %s", err, stdout, stderr)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	isMultiNodeEnabled := "false"
	testArgs := []string{isMultiNodeEnabled}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	checkNodeJoinCommand(t, tc, 0)

	t.Logf("%s: validating firewalld", time.Now().Format(time.RFC3339))
	line = []string{"firewalld-validate.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to validate firewalld: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	t.Logf("%s: resetting firewalld", time.Now().Format(time.RFC3339))
	line = []string{"firewalld-reset.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to reset firewalld: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstallationDebian12(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
		"EC_BINARY_PATH",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/multi-node-disabled-license.yaml",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()

	installSingleNodeWithOptions(t, tc, installOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	isMultiNodeEnabled := "false"
	testArgs := []string{isMultiNodeEnabled}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	checkNodeJoinCommand(t, tc, 0)

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstallationDebian11(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
		"EC_BINARY_PATH",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bullseye",
		LicensePath:  "licenses/multi-node-disabled-license.yaml",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()

	installSingleNodeWithOptions(t, tc, installOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	isMultiNodeEnabled := "false"
	testArgs := []string{isMultiNodeEnabled}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	checkNodeJoinCommand(t, tc, 0)

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstallationCentos9Stream(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
		"EC_BINARY_PATH",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "centos-9",
		LicensePath:  "licenses/multi-node-disabled-license.yaml",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()

	t.Logf("%s: installing tar", time.Now().Format(time.RFC3339))
	line := []string{"yum-install-tar.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	isMultiNodeEnabled := "false"
	testArgs := []string{isMultiNodeEnabled}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	checkNodeJoinCommand(t, tc, 0)

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestHostPreflightCustomSpec(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_FAILING_PREFLIGHTS_VERSION",
		"APP_WARNING_PREFLIGHTS_VERSION",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:      t,
		Nodes:  1,
		Distro: "centos-9",
	})
	defer tc.Cleanup()

	t.Logf("%s: installing test dependencies on node 0", time.Now().Format(time.RFC3339))
	line := []string{"yum", "install", "-y", "fio"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install dependencies on node 0: %v: %s: %s", err, stdout, stderr)
	}

	downloadECReleaseWithOptions(t, tc, 0, downloadECReleaseOptions{
		version: os.Getenv("APP_FAILING_PREFLIGHTS_VERSION"),
	})

	t.Logf("%s: moving embedded-cluster to /usr/local/bin/embedded-cluster-failing-preflights", time.Now().Format(time.RFC3339))
	line = []string{"mv", "/usr/local/bin/embedded-cluster", "/usr/local/bin/embedded-cluster-failing-preflights"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to move embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: removing the original license file", time.Now().Format(time.RFC3339))
	line = []string{"rm", "/assets/license.yaml"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove /assets/license.yaml on node 0: %v: %s: %s", err, stdout, stderr)
	}

	downloadECReleaseWithOptions(t, tc, 0, downloadECReleaseOptions{
		version: os.Getenv("APP_WARNING_PREFLIGHTS_VERSION"),
	})

	t.Logf("%s: running embedded-cluster preflights on node 0", time.Now().Format(time.RFC3339))
	line = []string{"embedded-preflight.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestHostPreflightInBuiltSpec(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"EC_BINARY_PATH",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "centos-9",
		LicensePath:  "licenses/license.yaml",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()

	t.Logf("%s: install single node with in-built host preflights", time.Now().Format(time.RFC3339))
	line := []string{"single-node-host-preflight-install.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster node with host preflights: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// This test creates 4 nodes, installs on the first one and then generate 2 join tokens
// for controllers and one join token for worker nodes. Joins the nodes and then waits
// for them to report ready.
func TestMultiNodeInstallation(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"K0S_INSTALL_VERSION",
		"EC_BINARY_PATH",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        4,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/license.yaml",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()

	installSingleNodeWithOptions(t, tc, installOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	checkWorkerProfile(t, tc, 0)

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// join a controller node
	joinControllerNode(t, tc, 1)
	checkWorkerProfile(t, tc, 1)

	// XXX If we are too aggressive joining nodes we can see the following error being
	// thrown by kotsadm on its log (and we get a 500 back):
	// "
	// failed to get controller role name: failed to get cluster config: failed to get
	// current installation: failed to list installations: etcdserver: leader changed
	// "
	t.Logf("node 1 joined, sleeping...")
	time.Sleep(30 * time.Second)

	// join another controller node
	joinControllerNode(t, tc, 2)
	checkWorkerProfile(t, tc, 2)

	// join a worker node
	joinWorkerNode(t, tc, 3)
	checkWorkerProfile(t, tc, 3)

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 4, nil)

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestInstallFromReplicatedApp(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:      t,
		Nodes:  1,
		Distro: "debian-bookworm",
	})
	defer tc.Cleanup()

	downloadECReleaseWithOptions(t, tc, 0, downloadECReleaseOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	installSingleNodeWithOptions(t, tc, installOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeUpgradePreviousStable(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_NOOP_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_NOOP_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:      t,
		Nodes:  1,
		Distro: "debian-bookworm",
	})
	defer tc.Cleanup()

	downloadECReleaseWithOptions(t, tc, 0, downloadECReleaseOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	installSingleNodeWithOptions(t, tc, installOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	appNoopVersion := os.Getenv("APP_NOOP_VERSION")
	testArgs := []string{appNoopVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: re-installing kots cli on node 0", time.Now().Format(time.RFC3339))
	line := []string{"install-kots-cli.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install kots cli on node 0: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_NOOP_VERSION"),
		k8sVersion: os.Getenv("K0S_NOOP_VERSION"),
	})

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster a second time", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestUpgradeFromReplicatedApp(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:      t,
		Nodes:  1,
		Distro: "debian-bookworm",
	})
	defer tc.Cleanup()

	downloadECReleaseWithOptions(t, tc, 0, downloadECReleaseOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	installSingleNodeWithOptions(t, tc, installOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	line := []string{"collect-support-bundle-host-in-cluster.sh"}
	stdout, stderr, err := tc.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to collect host support bundle: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestResetAndReinstall(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"K0S_INSTALL_VERSION",
		"EC_BINARY_PATH",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/license.yaml",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()

	installSingleNodeWithOptions(t, tc, installOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	resetInstallation(t, tc, 0)

	t.Logf("%s: waiting for nodes to reboot", time.Now().Format(time.RFC3339))
	time.Sleep(30 * time.Second)

	installSingleNodeWithOptions(t, tc, installOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestResetAndReinstallAirgap(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"K0S_INSTALL_VERSION",
	})

	tc := cmx.NewCluster(&cmx.ClusterInput{
		T:            t,
		Nodes:        1,
		Distribution: "ubuntu",
		Version:      "22.04",
	})
	defer tc.Cleanup()

	t.Logf("%s: downloading airgap file on node 0", time.Now().Format(time.RFC3339))
	err := downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_INSTALL_VERSION"), AirgapInstallBundlePath, AirgapLicenseID)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("%s: airgapping cluster", time.Now().Format(time.RFC3339))
	if err := tc.Airgap(); err != nil {
		t.Fatalf("failed to airgap cluster: %v", err)
	}

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}

	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v: %s: %s", tc.Nodes[0], err, stdout, stderr)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		isAirgap: true,
		version:  os.Getenv("APP_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	checkNodeJoinCommand(t, tc, 0)
	resetInstallation(t, tc, 0)

	t.Logf("%s: waiting for nodes to reboot", time.Now().Format(time.RFC3339))
	tc.WaitForReboot()

	installSingleNodeWithOptions(t, tc, installOptions{
		isAirgap: true,
		version:  os.Getenv("APP_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeAirgapUpgrade(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
	})

	tc := cmx.NewCluster(&cmx.ClusterInput{
		T:            t,
		Nodes:        1,
		Distribution: "ubuntu",
		Version:      "22.04",
	})
	defer tc.Cleanup()

	t.Logf("%s: downloading airgap files on node 0", time.Now().Format(time.RFC3339))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_INSTALL_VERSION"), AirgapInstallBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_UPGRADE_VERSION"), AirgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	t.Logf("%s: airgapping cluster", time.Now().Format(time.RFC3339))
	if err := tc.Airgap(); err != nil {
		t.Fatalf("failed to airgap cluster: %v", err)
	}

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v: %s: %s", tc.Nodes[0], err, stdout, stderr)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		isAirgap:                true,
		version:                 os.Getenv("APP_INSTALL_VERSION"),
		localArtifactMirrorPort: "50001", // choose an alternate lam port
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	checkNodeJoinCommand(t, tc, 0)

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeState(t, tc)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeAirgapUpgradeSelinux(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
	})

	tc := cmx.NewCluster(&cmx.ClusterInput{
		T:            t,
		Nodes:        1,
		Distribution: "almalinux",
		Version:      "8",
	})
	defer tc.Cleanup()

	t.Logf("%s: downloading airgap files on node 0", time.Now().Format(time.RFC3339))
	initialVersion := os.Getenv("APP_INSTALL_VERSION")
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, initialVersion, AirgapInstallBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_UPGRADE_VERSION"), AirgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	t.Logf("%s: airgapping cluster", time.Now().Format(time.RFC3339))
	if err := tc.Airgap(); err != nil {
		t.Fatalf("failed to airgap cluster: %v", err)
	}

	t.Logf("%s: creating /.autorelabel file for SELinux relabeling", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{"touch", "/.autorelabel"}); err != nil {
		t.Fatalf("fail to create /.autorelabel file on node %s: %v: %s: %s", tc.Nodes[0], err, stdout, stderr)
	}

	t.Logf("%s: rebooting VM for SELinux relabeling", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{"reboot"}); err != nil {
		t.Fatalf("fail to reboot node %s: %v: %s: %s", tc.Nodes[0], err, stdout, stderr)
	}

	t.Logf("%s: waiting for node to reboot", time.Now().Format(time.RFC3339))
	tc.WaitForReboot()

	t.Logf("%s: setting selinux to Enforcing mode", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{"setenforce 1"}); err != nil {
		t.Fatalf("fail to set selinux to Enforcing mode %s: %v: %s: %s", tc.Nodes[0], err, stdout, stderr)
	}

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"/usr/local/bin/airgap-prepare.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v: %s: %s", tc.Nodes[0], err, stdout, stderr)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		isAirgap:                true,
		version:                 initialVersion,
		localArtifactMirrorPort: "50001", // choose an alternate lam port
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"/usr/local/bin/check-airgap-installation-state.sh", initialVersion, k8sVersionPrevious()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	checkNodeJoinCommand(t, tc, 0)

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"/usr/local/bin/airgap-update.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeAirgapUpgradeCustomCIDR(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
	})

	tc := cmx.NewCluster(&cmx.ClusterInput{
		T:            t,
		Nodes:        1,
		Distribution: "ubuntu",
		Version:      "22.04",
	})
	defer tc.Cleanup()

	t.Logf("%s: downloading airgap files on node 0", time.Now().Format(time.RFC3339))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_INSTALL_VERSION"), AirgapInstallBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_UPGRADE_VERSION"), AirgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	t.Logf("%s: airgapping cluster", time.Now().Format(time.RFC3339))
	if err := tc.Airgap(); err != nil {
		t.Fatalf("failed to airgap cluster: %v", err)
	}

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v: %s: %s", tc.Nodes[0], err, stdout, stderr)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		isAirgap: true,
		version:  os.Getenv("APP_INSTALL_VERSION"),
		cidr:     "172.16.0.0/15",
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	// ensure that the cluster is using the right IP ranges.
	t.Logf("%s: checking service and pod IP addresses", time.Now().Format(time.RFC3339))

	// we have used --cidr 172.16.0.0/15 during install time so pods are
	// expected to be in the 172.16.0.0/16 range while services are in the
	// 172.17.0.0/16 range.
	podregex := `172\\.16\\.[0-9]\\+\\.[0-9]\\+`
	svcregex := `172\\.17\\.[0-9]\\+\\.[0-9]\\+`

	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{"check-cidr-ranges.sh", podregex, svcregex}); err != nil {
		t.Log(stdout)
		t.Log(stderr)
		t.Fatalf("fail to check addresses on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestMultiNodeAirgapUpgradeSameK0s(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
	})

	tc := cmx.NewCluster(&cmx.ClusterInput{
		T:            t,
		Nodes:        2,
		Distribution: "ubuntu",
		Version:      "22.04",
		InstanceType: "r1.medium",
	})
	defer tc.Cleanup()

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_INSTALL_VERSION"), AirgapInstallBundlePath, AirgapLicenseID)
		},
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_UPGRADE_VERSION"), AirgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	t.Logf("%s: airgapping cluster", time.Now().Format(time.RFC3339))
	if err := tc.Airgap(); err != nil {
		t.Fatalf("failed to airgap cluster: %v", err)
	}

	t.Logf("%s: preparing embedded cluster airgap files on node 0", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node 0: %v: %s: %s", err, stdout, stderr)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		isAirgap: true,
		version:  os.Getenv("APP_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// join a worker
	joinWorkerNode(t, tc, 1)

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 2, nil)

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestMultiNodeAirgapUpgrade(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
	})

	tc := cmx.NewCluster(&cmx.ClusterInput{
		T:            t,
		Nodes:        2,
		Distribution: "ubuntu",
		Version:      "22.04",
		InstanceType: "r1.medium",
	})
	defer tc.Cleanup()

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_INSTALL_VERSION"), AirgapInstallBundlePath, AirgapLicenseID)
		},
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_UPGRADE_VERSION"), AirgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	t.Logf("%s: airgapping cluster", time.Now().Format(time.RFC3339))
	if err := tc.Airgap(); err != nil {
		t.Fatalf("failed to airgap cluster: %v", err)
	}

	t.Logf("%s: preparing embedded cluster airgap files on node 0", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node 0: %v: %s: %s", err, stdout, stderr)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		isAirgap:                true,
		version:                 os.Getenv("APP_INSTALL_VERSION"),
		localArtifactMirrorPort: "50001", // choose an alternate lam port
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// join a worker
	joinWorkerNode(t, tc, 1)

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 2, nil)

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestMultiNodeAirgapUpgradePreviousStable(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_NOOP_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_NOOP_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
	})

	// Use an alternate data directory
	withEnv := map[string]string{
		"EMBEDDED_CLUSTER_BASE_DIR": "/var/lib/ec",
	}

	tc := cmx.NewCluster(&cmx.ClusterInput{
		T:            t,
		Nodes:        2,
		Distribution: "ubuntu",
		Version:      "22.04",
		InstanceType: "r1.medium",
	})
	defer tc.Cleanup(withEnv)

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_INSTALL_VERSION"), AirgapInstallBundlePath, AirgapLicenseID)
		},
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_NOOP_VERSION"), AirgapUpgradeBundlePath, AirgapLicenseID)
		},
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_UPGRADE_VERSION"), AirgapUpgrade2BundlePath, AirgapLicenseID)
		},
	)

	t.Logf("%s: airgapping cluster", time.Now().Format(time.RFC3339))
	if err := tc.Airgap(); err != nil {
		t.Fatalf("failed to airgap cluster: %v", err)
	}

	t.Logf("%s: preparing embedded cluster airgap files on node 0", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to prepare airgap files on node 0: %v: %s: %s", err, stdout, stderr)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		isAirgap:                true,
		version:                 os.Getenv("APP_INSTALL_VERSION"),
		localArtifactMirrorPort: "50001",
		dataDir:                 "/var/lib/ec",
		withEnv:                 withEnv,
	})

	if err := tc.SetupPlaywright(withEnv); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// join a worker
	joinWorkerNode(t, tc, 1)

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 2, withEnv)

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to run airgap update: %v: %s: %s", err, stdout, stderr)
	}

	appNoopVersion := os.Getenv("APP_NOOP_VERSION")
	testArgs := []string{appNoopVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after noop upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", os.Getenv("APP_NOOP_VERSION"), os.Getenv("K0S_NOOP_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running second airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update2.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to run airgap update: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster a second time", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
		withEnv:    withEnv,
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// This test creates 4 nodes, installs on the first one and then generate 2 join tokens
// for controllers and one join token for worker nodes. Joins the nodes as HA and then waits
// for them to report ready. Runs additional high availability validations afterwards.
func TestMultiNodeHAInstallation(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
		"EC_BINARY_PATH",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:                      t,
		Nodes:                  4,
		Distro:                 "debian-bookworm",
		LicensePath:            "licenses/license.yaml",
		ECBinaryPath:           os.Getenv("EC_BINARY_PATH"),
		SupportBundleNodeIndex: 2,
	})
	defer tc.Cleanup()

	installSingleNodeWithOptions(t, tc, installOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// join a worker
	joinWorkerNode(t, tc, 1)

	// join a controller
	joinControllerNode(t, tc, 2)

	// join another controller in HA mode
	joinControllerNodeWithOptions(t, tc, 3, joinOptions{isHA: true})

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 4, nil)

	t.Logf("%s: checking installation state after enabling high availability", time.Now().Format(time.RFC3339))
	line := []string{"check-post-ha-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post ha state: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	bin := "embedded-cluster"
	t.Logf("%s: resetting controller node 0", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunCommandOnNode(0, []string{bin, "reset", "--yes"})
	if err != nil {
		t.Fatalf("fail to remove controller node 0: %v: %s: %s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "High-availability is enabled and requires at least three controller-test nodes") {
		t.Errorf("reset output does not contain the ha warning")
		t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
	}

	stdout, stderr, err = tc.RunCommandOnNode(2, []string{"check-nodes-removed.sh", "3"})
	if err != nil {
		t.Fatalf("fail to check nodes removed: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking nllb", time.Now().Format(time.RFC3339))
	line = []string{"check-nllb.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(2, line); err != nil {
		t.Fatalf("fail to check nllb: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		node:       2,
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
		withEnv: map[string]string{
			"ALLOW_PENDING_PODS": "true",
		},
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// This test creates 4 airgap nodes, installs on the first one and then generate 2 join tokens
// for controllers and one join token for worker nodes. Joins the nodes as airgap HA and then waits
// for them to report ready. Runs additional high availability validations afterwards.
func TestMultiNodeAirgapHAInstallation(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
	})

	tc := cmx.NewCluster(&cmx.ClusterInput{
		T:                      t,
		Nodes:                  4,
		Distribution:           "ubuntu",
		Version:                "22.04",
		InstanceType:           "r1.medium",
		SupportBundleNodeIndex: 2,
	})
	defer tc.Cleanup()

	t.Logf("%s: downloading airgap files on nodes", time.Now().Format(time.RFC3339))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_INSTALL_VERSION"), AirgapInstallBundlePath, AirgapLicenseID)
		},
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_UPGRADE_VERSION"), AirgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	// install "expect" dependency on node 3 as that's where the HA join command will run.
	t.Logf("%s: installing expect package on node 3", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(3, []string{"apt-get", "install", "-y", "expect"}); err != nil {
		t.Fatalf("fail to install expect package on node 3: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: airgapping cluster", time.Now().Format(time.RFC3339))
	if err := tc.Airgap(); err != nil {
		t.Fatalf("failed to airgap cluster: %v", err)
	}

	t.Logf("%s: preparing embedded cluster airgap files on node 0", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node 0: %v: %s: %s", err, stdout, stderr)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		isAirgap: true,
		version:  os.Getenv("APP_INSTALL_VERSION"),
	})

	checkWorkerProfile(t, tc, 0)

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	// join a worker
	joinWorkerNode(t, tc, 1)
	checkWorkerProfile(t, tc, 1)

	// join a controller
	joinControllerNode(t, tc, 2)
	checkWorkerProfile(t, tc, 2)

	// join another controller in HA mode
	joinControllerNodeWithOptions(t, tc, 3, joinOptions{isHA: true})
	checkWorkerProfile(t, tc, 3)

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 4, nil)

	t.Logf("%s: checking installation state after enabling high availability", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-post-ha-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post ha state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	stdout, stderr, err := resetInstallationWithError(t, tc, 0, resetInstallationOptions{})
	if err != nil {
		t.Fatalf("fail to reset the installation on node 0: %v: %s: %s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "High-availability is enabled and requires at least three controller-test nodes") {
		t.Logf("reset output does not contain the ha warning: stdout: %s\nstderr: %s", stdout, stderr)
	}

	stdout, stderr, err = tc.RunCommandOnNode(2, []string{"check-nodes-removed.sh", "3"})
	if err != nil {
		t.Fatalf("fail to check nodes removed: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking nllb", time.Now().Format(time.RFC3339))
	line = []string{"check-nllb.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(2, line); err != nil {
		t.Fatalf("fail to check nllb: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		node:       2,
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
		withEnv: map[string]string{
			"ALLOW_PENDING_PODS": "true",
		},
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestInstallSnapshotFromReplicatedApp(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:      t,
		Nodes:  1,
		Distro: "debian-bookworm",
	})
	defer tc.Cleanup()

	downloadECReleaseWithOptions(t, tc, 0, downloadECReleaseOptions{
		version:   os.Getenv("APP_INSTALL_VERSION"),
		licenseID: SnapshotLicenseID,
	})

	installSingleNodeWithOptions(t, tc, installOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	t.Logf("%s: ensuring velero is installed", time.Now().Format(time.RFC3339))
	line := []string{"check-velero-state.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check velero state: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// TestCustomCIDR tests the installation with an alternate CIDR range
func TestCustomCIDR(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"K0S_INSTALL_VERSION",
		"EC_BINARY_PATH",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        4,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/license.yaml",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()
	t.Log("non-proxied infrastructure created")

	installSingleNodeWithOptions(t, tc, installOptions{
		version:     os.Getenv("APP_INSTALL_VERSION"),
		podCidr:     "10.128.0.0/20",
		serviceCidr: "10.129.0.0/20",
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// join a controller node
	joinControllerNode(t, tc, 1)

	// XXX If we are too aggressive joining nodes we can see the following error being
	// thrown by kotsadm on its log (and we get a 500 back):
	// "
	// failed to get controller role name: failed to get cluster config: failed to get
	// current installation: failed to list installations: etcdserver: leader changed
	// "
	t.Logf("node 1 joined, sleeping...")
	time.Sleep(30 * time.Second)

	// join another controller node
	joinControllerNode(t, tc, 2)

	// join a worker node
	joinWorkerNode(t, tc, 3)

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 4, nil)

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	// ensure that the cluster is using the right IP ranges.
	t.Logf("%s: checking service and pod IP addresses", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunCommandOnNode(0, []string{"check-cidr-ranges.sh", "^10.128.[0-9]*.[0-9]", "^10.129.[0-9]*.[0-9]"})
	if err != nil {
		t.Fatalf("fail to check addresses on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstallationNoopUpgrade(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_NOOP_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_NOOP_VERSION",
		"EC_BINARY_PATH",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/license.yaml",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()

	installSingleNodeWithOptions(t, tc, installOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	appUpgradeVersion := os.Getenv("APP_NOOP_VERSION")
	skipClusterUpgradeCheck := "true"
	testArgs := []string{appUpgradeVersion, skipClusterUpgradeCheck}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_NOOP_VERSION"),
		k8sVersion: os.Getenv("K0S_NOOP_VERSION"),
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestFiveNodesAirgapUpgrade(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
	})

	tc := cmx.NewCluster(&cmx.ClusterInput{
		T:            t,
		Nodes:        5,
		Distribution: "ubuntu",
		Version:      "22.04",
		InstanceType: "r1.medium",
	})
	defer tc.Cleanup()

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_INSTALL_VERSION"), AirgapInstallBundlePath, AirgapLicenseID)
		},
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_UPGRADE_VERSION"), AirgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	t.Logf("%s: airgapping cluster", time.Now().Format(time.RFC3339))
	if err := tc.Airgap(); err != nil {
		t.Fatalf("failed to airgap cluster: %v", err)
	}

	t.Logf("%s: preparing embedded cluster airgap files on node 0", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node 0: %v: %s: %s", err, stdout, stderr)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		isAirgap: true,
		version:  os.Getenv("APP_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// generate controller node join command.
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunPlaywrightTest("get-join-controller-commands")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	joinCommands, err := findJoinCommandsInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("controller join commands:", joinCommands)

	// bypass ha prompt
	for i := range joinCommands {
		joinCommands[i] = strings.Replace(joinCommands[i], "join", "join --no-ha", 1)
	}

	// join the controller nodes
	runInParallelOffset(t, time.Second*30,
		func(t *testing.T) error {
			for _, joinCommand := range joinCommands {
				stdout, stderr, err := tc.RunCommandOnNode(1, strings.Fields(joinCommand))
				if err != nil {
					return fmt.Errorf("unable to join node 1: %w: %s: %s", err, stdout, stderr)
				}
			}
			return nil
		}, func(t *testing.T) error {
			for _, joinCommand := range joinCommands {
				stdout, stderr, err := tc.RunCommandOnNode(2, strings.Fields(joinCommand))
				if err != nil {
					return fmt.Errorf("unable to join node 2: %w: %s: %s", err, stdout, stderr)
				}
			}
			return nil
		}, func(t *testing.T) error {
			for _, joinCommand := range joinCommands {
				stdout, stderr, err := tc.RunCommandOnNode(3, strings.Fields(joinCommand))
				if err != nil {
					return fmt.Errorf("unable to join node 3: %w: %s: %s", err, stdout, stderr)
				}
			}
			return nil
		}, func(t *testing.T) error {
			for _, joinCommand := range joinCommands {
				stdout, stderr, err := tc.RunCommandOnNode(4, strings.Fields(joinCommand))
				if err != nil {
					return fmt.Errorf("unable to join node 4: %w: %s: %s", err, stdout, stderr)
				}
			}
			return nil
		},
	)

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 5, nil)

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	testArgs := []string{os.Getenv("APP_UPGRADE_VERSION")}
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestInstallWithConfigValues(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
		"EC_BINARY_PATH",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "almalinux-8",
		LicensePath:  "licenses/license.yaml",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()

	t.Logf("%s: installing tar", time.Now().Format(time.RFC3339))
	line := []string{"yum-install-tar.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	hostname := uuid.New().String()
	password := uuid.New().String()

	// create a config values file on the node
	configValuesFileContent := fmt.Sprintf(`
apiVersion: kots.io/v1beta1
kind: ConfigValues
spec:
  values:
    hostname:
      value: %s
    pw:
      value: %s
`, hostname, password)
	configValuesFileB64 := base64.StdEncoding.EncodeToString([]byte(configValuesFileContent))
	t.Logf("%s: creating config values file", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunCommandOnNode(0, []string{"mkdir", "-p", "/assets"})
	if err != nil {
		t.Fatalf("fail to create config values file directory: %v: %s: %s", err, stdout, stderr)
	}
	stdout, stderr, err = tc.RunCommandOnNode(0, []string{"echo", "'" + configValuesFileB64 + "'", "|", "base64", "-d", ">", "/assets/config-values.yaml"})
	if err != nil {
		t.Fatalf("fail to create config values file: %v: %s: %s", err, stdout, stderr)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		version:          os.Getenv("APP_INSTALL_VERSION"),
		configValuesFile: "/assets/config-values.yaml",
	})

	t.Logf("%s: checking config values", time.Now().Format(time.RFC3339))
	line = []string{"check-config-values.sh", hostname, password}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check config values: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs := []string{appUpgradeVersion, "", hostname}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	t.Logf("%s: checking config values after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-config-values.sh", "updated-hostname.com", "updated password"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check config values: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeAirgapUpgradeConfigValues(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
	})

	tc := cmx.NewCluster(&cmx.ClusterInput{
		T:            t,
		Nodes:        1,
		Distribution: "ubuntu",
		Version:      "22.04",
	})
	defer tc.Cleanup()

	t.Logf("%s: downloading airgap files on node 0", time.Now().Format(time.RFC3339))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_INSTALL_VERSION"), AirgapInstallBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_UPGRADE_VERSION"), AirgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	t.Logf("%s: airgapping cluster", time.Now().Format(time.RFC3339))
	if err := tc.Airgap(); err != nil {
		t.Fatalf("failed to airgap cluster: %v", err)
	}

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v: %s: %s", tc.Nodes[0], err, stdout, stderr)
	}

	hostname := uuid.New().String()
	password := uuid.New().String()

	// create a config values file on the node
	configValuesFileContent := fmt.Sprintf(`
apiVersion: kots.io/v1beta1
kind: ConfigValues
spec:
  values:
    hostname:
      value: %s
    pw:
      value: %s
`, hostname, password)
	configValuesFileB64 := base64.StdEncoding.EncodeToString([]byte(configValuesFileContent))

	t.Logf("%s: creating config values file", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunCommandOnNode(0, []string{"sh", "-c", fmt.Sprintf("'echo %s | base64 -d > /assets/config-values.yaml'", configValuesFileB64)})
	if err != nil {
		t.Fatalf("fail to create config values file: %v: %s: %s", err, stdout, stderr)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		isAirgap:                true,
		version:                 os.Getenv("APP_INSTALL_VERSION"),
		localArtifactMirrorPort: "50001", // choose an alternate lam port
		configValuesFile:        "/assets/config-values.yaml",
	})

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking config values", time.Now().Format(time.RFC3339))
	line = []string{"check-config-values.sh", hostname, password}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check config values: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := os.Getenv("APP_UPGRADE_VERSION")
	testArgs := []string{appUpgradeVersion, "", hostname}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		ecVersion:  os.Getenv("EC_UPGRADE_VERSION"),
		k8sVersion: os.Getenv("K0S_UPGRADE_VERSION"),
	})

	t.Logf("%s: checking config values after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-config-values.sh", "updated-hostname.com", "updated password"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check config values: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
