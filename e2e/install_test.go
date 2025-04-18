package e2e

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/lxd"
	"github.com/replicatedhq/embedded-cluster/pkg/certs"
)

func TestSingleNodeInstallation(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "ubuntu-jammy",
		LicensePath:  "licenses/multi-node-disabled-license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	installSingleNodeWithOptions(t, tc, installOptions{
		adminConsolePort: "30002",
	})

	isMultiNodeEnabled := "false"
	testArgs := []string{isMultiNodeEnabled}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationState(t, tc)

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line := []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: resetting admin console password", time.Now().Format(time.RFC3339))
	newPassword := "newpass"
	line = []string{"embedded-cluster", "admin-console", "reset-password", newPassword}
	_, _, err := tc.RunCommandOnNode(0, line)
	require.NoError(t, err, "unable to reset admin console password")

	t.Logf("%s: logging in with the new password", time.Now().Format(time.RFC3339))
	_, _, err = tc.RunPlaywrightTest("login-with-custom-password", newPassword)
	require.NoError(t, err, "unable to login with the new password")

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstallationAlmaLinux8(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "almalinux-8",
		LicensePath:  "licenses/multi-node-disabled-license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
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

	installSingleNode(t, tc)

	isMultiNodeEnabled := "false"
	testArgs := []string{isMultiNodeEnabled}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationState(t, tc)

	t.Logf("%s: validating firewalld", time.Now().Format(time.RFC3339))
	line = []string{"firewalld-validate.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to validate firewalld: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: resetting firewalld", time.Now().Format(time.RFC3339))
	line = []string{"firewalld-reset.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to reset firewalld: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstallationDebian12(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/multi-node-disabled-license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	installSingleNode(t, tc)

	isMultiNodeEnabled := "false"
	testArgs := []string{isMultiNodeEnabled}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationState(t, tc)

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line := []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstallationDebian11(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bullseye",
		LicensePath:  "licenses/multi-node-disabled-license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	installSingleNode(t, tc)

	isMultiNodeEnabled := "false"
	testArgs := []string{isMultiNodeEnabled}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationState(t, tc)

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line := []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstallationCentos9Stream(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "centos-9",
		LicensePath:  "licenses/multi-node-disabled-license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	t.Logf("%s: installing tar", time.Now().Format(time.RFC3339))
	line := []string{"yum-install-tar.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	installSingleNode(t, tc)

	isMultiNodeEnabled := "false"
	testArgs := []string{isMultiNodeEnabled}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationState(t, tc)

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestHostPreflightCustomSpec(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

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

	t.Logf("%s: downloading failing-preflights embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"vandoor-prepare.sh", fmt.Sprintf("appver-%s-failing-preflights", os.Getenv("SHORT_SHA")), LicenseID, "false"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to download embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

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

	t.Logf("%s: downloading warning-preflights embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"vandoor-prepare.sh", fmt.Sprintf("appver-%s-warning-preflights", os.Getenv("SHORT_SHA")), LicenseID, "false"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to download embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running embedded-cluster preflights on node 0", time.Now().Format(time.RFC3339))
	line = []string{"embedded-preflight.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestHostPreflightInBuiltSpec(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "centos-9",
		LicensePath:  "licenses/license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
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
	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        4,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	installSingleNode(t, tc)
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

	checkInstallationState(t, tc)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestInstallFromReplicatedApp(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:      t,
		Nodes:  1,
		Distro: "debian-bookworm",
	})
	defer tc.Cleanup()

	t.Logf("%s: downloading embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"vandoor-prepare.sh", fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")), LicenseID, "false"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to download embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	installSingleNode(t, tc)

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationState(t, tc)

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeUpgradePreviousStable(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:      t,
		Nodes:  1,
		Distro: "debian-bookworm",
	})
	defer tc.Cleanup()

	t.Logf("%s: downloading embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	initialVersion := fmt.Sprintf("appver-%s-previous-stable", os.Getenv("SHORT_SHA"))
	line := []string{"vandoor-prepare.sh", initialVersion, LicenseID, "false"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to download embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-install.sh", "ui", initialVersion}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    initialVersion,
		k8sVersion: k8sVersionPreviousStable(),
	})

	appUpgradeVersion := fmt.Sprintf("appver-%s-noop", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: re-installing kots cli on node 0", time.Now().Format(time.RFC3339))
	line = []string{"install-kots-cli.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install kots cli on node 0: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version: appUpgradeVersion,
	})

	appUpgradeVersion = fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster a second time", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after second upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestUpgradeFromReplicatedApp(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:      t,
		Nodes:  1,
		Distro: "debian-bookworm",
	})
	defer tc.Cleanup()

	initialVersion := fmt.Sprintf("appver-%s-previous-k0s", os.Getenv("SHORT_SHA"))
	t.Logf("%s: downloading embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"vandoor-prepare.sh", initialVersion, LicenseID, "false"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to download embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-install.sh", "ui", initialVersion}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    initialVersion,
		k8sVersion: k8sVersionPrevious(),
	})

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	line = []string{"collect-support-bundle-host-in-cluster.sh"}
	stdout, stderr, err := tc.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to collect host support bundle: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestUpgradeEC18FromReplicatedApp(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	withEnv := map[string]string{"KUBECONFIG": "/var/lib/k0s/pki/admin.conf"}

	tc := docker.NewCluster(&docker.ClusterInput{
		T:      t,
		Nodes:  4,
		Distro: "debian-bookworm",
		K0sDir: "/var/lib/k0s",
	})
	defer tc.Cleanup(withEnv)

	appVer := fmt.Sprintf("appver-%s-1.8.0-k8s-1.28", os.Getenv("SHORT_SHA"))

	t.Logf("%s: downloading embedded-cluster %s on node 0", appVer, time.Now().Format(time.RFC3339))
	line := []string{"vandoor-prepare.sh", appVer, LicenseID, "false"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to download embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: downloading embedded-cluster %s on worker node", appVer, time.Now().Format(time.RFC3339))
	line = []string{"vandoor-prepare.sh", appVer, LicenseID, "false"}
	if stdout, stderr, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to download embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: installing embedded-cluster %s on node 0", appVer, time.Now().Format(time.RFC3339))
	line = []string{"single-node-install.sh", "ui", appVer}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to install embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	if err := tc.SetupPlaywright(withEnv); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-ec18-app-version"); err != nil {
		t.Fatalf("fail to run playwright test deploy-ec18-app-version: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunPlaywrightTest("get-ec18-join-worker-command")
	if err != nil {
		t.Fatalf("fail to generate worker join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v: %s: %s", err, stdout, stderr)
	}
	t.Log("worker join token command:", command)

	t.Logf("%s: joining worker node to the cluster as a worker", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(1, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join worker node to the cluster as a worker: %v: %s: %s", err, stdout, stderr)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, stderr, err = tc.RunCommandOnNode(0, []string{"wait-for-ready-nodes.sh", "2"}, withEnv)
	if err != nil {
		t.Fatalf("fail to wait for ready nodes: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    appVer,
		k8sVersion: "v1.28.11",
		withEnv:    withEnv,
	})

	appUpgradeVersion := fmt.Sprintf("appver-%s-noop", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: re-installing kots cli on node 0", time.Now().Format(time.RFC3339))
	line = []string{"install-kots-cli.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install kots cli on node 0: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version: appUpgradeVersion,
		withEnv: withEnv,
	})

	// Download embedded-cluster on additional nodes
	t.Logf("%s: downloading embedded-cluster %s on additional nodes", time.Now().Format(time.RFC3339), appUpgradeVersion)
	line = []string{"vandoor-prepare.sh", appUpgradeVersion, LicenseID, "false"}
	if stdout, stderr, err := tc.RunCommandOnNode(2, line); err != nil {
		t.Fatalf("fail to download embedded-cluster on node 2: %v: %s: %s", err, stdout, stderr)
	}
	if stdout, stderr, err := tc.RunCommandOnNode(3, line); err != nil {
		t.Fatalf("fail to download embedded-cluster on node 3: %v: %s: %s", err, stdout, stderr)
	}

	// Join the additional nodes to the cluster
	t.Logf("%s: joining additional controller and worker node to the cluster after upgrade", time.Now().Format(time.RFC3339))
	joinControllerNode(t, tc, 2)
	joinWorkerNode(t, tc, 3)

	// wait for all nodes to report as ready
	waitForNodes(t, tc, 4, withEnv)

	// Check worker profiles for the joined nodes
	checkWorkerProfile(t, tc, 2)
	checkWorkerProfile(t, tc, 3)

	appUpgradeVersion = fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster a second time", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after second upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	// wait for all nodes to report as ready after upgrade
	waitForNodes(t, tc, 4, withEnv)

	// use upgraded binaries to run the reset command
	// TODO: this is a temporary workaround and should eventually be a feature of EC

	t.Logf("%s: downloading embedded-cluster %s on node 0", time.Now().Format(time.RFC3339), appUpgradeVersion)
	line = []string{"vandoor-prepare.sh", appUpgradeVersion, LicenseID, "false"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to download embedded-cluster version %s on node 0: %v: %s: %s", appUpgradeVersion, err, stdout, stderr)
	}

	t.Logf("%s: downloading embedded-cluster %s on worker node 1", time.Now().Format(time.RFC3339), appUpgradeVersion)
	line = []string{"vandoor-prepare.sh", appUpgradeVersion, LicenseID, "false"}
	if stdout, stderr, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to download embedded-cluster version %s on worker node: %v: %s: %s", appUpgradeVersion, err, stdout, stderr)
	}

	t.Logf("%s: resetting worker node 1", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(1, line, withEnv); err != nil {
		t.Fatalf("fail to reset worker node: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: resetting worker node 3", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(3, line, withEnv); err != nil {
		t.Fatalf("fail to reset worker node: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: resetting controller node 2", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(2, line, withEnv); err != nil {
		t.Fatalf("fail to reset controller node: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: resetting node 0", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to reset node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestResetAndReinstall(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	installSingleNode(t, tc)

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationState(t, tc)

	t.Logf("%s: resetting the installation", time.Now().Format(time.RFC3339))
	line := []string{"reset-installation.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to reset the installation: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: waiting for nodes to reboot", time.Now().Format(time.RFC3339))
	time.Sleep(30 * time.Second)

	installSingleNode(t, tc)

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationState(t, tc)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestResetAndReinstallAirgap(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	t.Logf("%s: downloading airgap file", time.Now().Format(time.RFC3339))
	airgapBundlePath := "/tmp/airgap-bundle.tar.gz"
	err := downloadAirgapBundle(t, fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")), airgapBundlePath, AirgapLicenseID)
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

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")), k8sVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: resetting the installation", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to reset the installation: %v", err)
	}

	t.Logf("%s: waiting for nodes to reboot", time.Now().Format(time.RFC3339))
	time.Sleep(30 * time.Second)

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")), k8sVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestOldVersionUpgrade(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	withEnv := map[string]string{"KUBECONFIG": "/var/lib/k0s/pki/admin.conf"}

	tc := docker.NewCluster(&docker.ClusterInput{
		T:      t,
		Nodes:  1,
		Distro: "debian-bookworm",
		K0sDir: "/var/lib/k0s",
	})
	defer tc.Cleanup(withEnv)

	t.Logf("%s: downloading embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"vandoor-prepare.sh", fmt.Sprintf("appver-%s-pre-minio-removal", os.Getenv("SHORT_SHA")), LicenseID}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to download embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"pre-minio-removal-install.sh", "cli"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to install embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-pre-minio-removal-installation-state.sh", fmt.Sprintf("%s-pre-minio-removal", os.Getenv("SHORT_SHA"))}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running kots upstream upgrade", time.Now().Format(time.RFC3339))
	line = []string{"kots-upstream-upgrade.sh", os.Getenv("SHORT_SHA")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to run kots upstream upgrade: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeAirgapUpgrade(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	initialVersion := fmt.Sprintf("appver-%s-previous-k0s", os.Getenv("SHORT_SHA"))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundle(t, initialVersion, airgapInstallBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                       t,
		Nodes:                   1,
		Image:                   "debian/12",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapInstallBundlePath,
		AirgapUpgradeBundlePath: airgapUpgradeBundlePath,
	})
	defer tc.Cleanup()

	// delete airgap bundles once they've been copied to the nodes
	if err := os.Remove(airgapInstallBundlePath); err != nil {
		t.Logf("failed to remove airgap install bundle: %v", err)
	}
	if err := os.Remove(airgapUpgradeBundlePath); err != nil {
		t.Logf("failed to remove airgap upgrade bundle: %v", err)
	}

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh", initialVersion, "--local-artifact-mirror-port", "50001"} // choose an alternate lam port
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	// remove the airgap bundle after installation
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}

	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", initialVersion, k8sVersionPrevious()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}
	// remove the airgap bundle after upgrade
	line = []string{"rm", "/assets/upgrade/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeAirgapUpgradeCustomCIDR(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	initialVersion := fmt.Sprintf("appver-%s-previous-k0s", os.Getenv("SHORT_SHA"))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundle(t, initialVersion, airgapInstallBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                       t,
		Nodes:                   1,
		Image:                   "debian/12",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapInstallBundlePath,
		AirgapUpgradeBundlePath: airgapUpgradeBundlePath,
	})
	defer tc.Cleanup()

	// delete airgap bundles once they've been copied to the nodes
	if err := os.Remove(airgapInstallBundlePath); err != nil {
		t.Logf("failed to remove airgap install bundle: %v", err)
	}
	if err := os.Remove(airgapUpgradeBundlePath); err != nil {
		t.Logf("failed to remove airgap upgrade bundle: %v", err)
	}

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh", initialVersion}
	line = append(line, "--cidr", "172.16.0.0/15")
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	// remove the airgap bundle after installation
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}

	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", initialVersion, k8sVersionPrevious()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}
	// remove the airgap bundle after upgrade
	line = []string{"rm", "/assets/upgrade/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	// ensure that the cluster is using the right IP ranges.
	t.Logf("%s: checking service and pod IP addresses", time.Now().Format(time.RFC3339))

	// we have used --cidr 172.16.0.0/15 during install time so pods are
	// expected to be in the 172.16.0.0/16 range while services are in the
	// 172.17.0.0/16 range.
	podregex := `172\.16\.[0-9]\+\.[0-9]\+`
	svcregex := `172\.17\.[0-9]\+\.[0-9]\+`

	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{"check-cidr-ranges.sh", podregex, svcregex}); err != nil {
		t.Log(stdout)
		t.Log(stderr)
		t.Fatalf("fail to check addresses on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestAirgapUpgradeFromEC18(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	withEnv := map[string]string{"KUBECONFIG": "/var/lib/k0s/pki/admin.conf"}

	appVer := fmt.Sprintf("appver-%s-1.8.0-k8s-1.28", os.Getenv("SHORT_SHA"))

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	airgapUpgrade2BundlePath := "/tmp/airgap-upgrade2-bundle.tar.gz"
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundle(t, appVer, airgapInstallBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s-noop", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgrade2BundlePath, AirgapLicenseID)
		},
	)

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                        t,
		Nodes:                    2,
		Image:                    "debian/12",
		WithProxy:                true,
		AirgapInstallBundlePath:  airgapInstallBundlePath,
		AirgapUpgradeBundlePath:  airgapUpgradeBundlePath,
		AirgapUpgrade2BundlePath: airgapUpgrade2BundlePath,
		LowercaseNodeNames:       true,
	})
	defer tc.Cleanup(withEnv)

	// delete airgap bundles once they've been copied to the nodes
	if err := os.Remove(airgapInstallBundlePath); err != nil {
		t.Logf("failed to remove airgap install bundle: %v", err)
	}
	if err := os.Remove(airgapUpgradeBundlePath); err != nil {
		t.Logf("failed to remove airgap upgrade bundle: %v", err)
	}
	if err := os.Remove(airgapUpgrade2BundlePath); err != nil {
		t.Logf("failed to remove airgap upgrade bundle: %v", err)
	}

	// upgrade airgap bundle is only needed on the first node
	line := []string{"rm", "/assets/ec-release-upgrade.tgz"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to remove upgrade airgap bundle on node %s: %v", tc.Nodes[1], err)
	}

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line = []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh", appVer}
	if _, _, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	// remove the airgap bundle after installation
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}

	if err := tc.SetupPlaywright(withEnv); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := tc.RunPlaywrightTest("deploy-ec18-app-version"); err != nil {
		t.Fatalf("fail to run playwright test deploy-ec18-app-version: %v", err)
	}

	// generate worker node join command.
	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunPlaywrightTest("get-ec18-join-worker-command")
	if err != nil {
		t.Fatalf("fail to generate worker join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	workerCommand, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("worker join token command:", workerCommand)

	// join the worker node
	t.Logf("%s: preparing embedded cluster airgap files on worker node", time.Now().Format(time.RFC3339))
	line = []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to prepare airgap files on worker node: %v", err)
	}
	t.Logf("%s: joining worker node to the cluster", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunCommandOnNode(1, strings.Split(workerCommand, " ")); err != nil {
		t.Fatalf("fail to join worker node to the cluster: %v", err)
	}
	// remove artifacts after joining to save space
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on worker node: %v", err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on worker node: %v", err)
	}
	line = []string{"rm", "/var/lib/embedded-cluster/bin/embedded-cluster"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on node %s: %v", tc.Nodes[0], err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = tc.RunCommandOnNode(0, []string{"wait-for-ready-nodes.sh", "2"}, withEnv)
	if err != nil {
		t.Log(stdout)
		t.Fatalf("fail to wait for ready nodes: %v", err)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{
		"check-airgap-installation-state.sh",
		// the initially installed version is 1.8.0+k8s-1.28
		// the '+' character is problematic in the regex used to validate the version, so we use '.' instead
		appVer,
		"v1.28.11"}
	if _, _, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}
	// remove the airgap bundle after upgrade
	line = []string{"rm", "/assets/upgrade/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-noop", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after noop upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", appUpgradeVersion, k8sVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running second airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update2.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}
	// remove the airgap bundle and binary after upgrade
	line = []string{"rm", "/assets/upgrade2/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster-upgrade2"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster-upgrade2 binary on node %s: %v", tc.Nodes[0], err)
	}

	appUpgradeVersion = fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster a second time", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after second upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	// TODO: reset fails with the following error:
	//   error: could not reset k0s: exit status 1, time="2024-10-17 22:44:52" level=warning msg="To ensure a full reset, a node reboot is recommended."
	//   Error: errors received during clean-up: [failed to delete /run/k0s. err: unlinkat /run/k0s/containerd/io.containerd.grpc.v1.cri/sandboxes/.../shm: device or resource busy]

	// t.Logf("%s: resetting worker node", time.Now().Format(time.RFC3339))
	// line = []string{"reset-installation.sh"}
	// if stdout, stderr, err := tc.RunCommandOnNode(1, line, withEnv); err != nil {
	// 	t.Fatalf("fail to reset worker node: %v: %s: %s", err, stdout, stderr)
	// }

	// // use upgrade binary for reset
	// withUpgradeBin := map[string]string{"EMBEDDED_CLUSTER_BIN": "embedded-cluster-upgrade"}

	// t.Logf("%s: resetting node 0", time.Now().Format(time.RFC3339))
	// line = []string{"reset-installation.sh"}
	// if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv, withUpgradeBin); err != nil {
	// 	t.Fatalf("fail to reset node 0: %v: %s: %s", err, stdout, stderr)
	// }

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestMultiNodeAirgapUpgradeSameK0s(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")), airgapInstallBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                       t,
		Nodes:                   2,
		Image:                   "debian/12",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapInstallBundlePath,
		AirgapUpgradeBundlePath: airgapUpgradeBundlePath,
	})
	defer tc.Cleanup()

	// delete airgap bundles once they've been copied to the nodes
	if err := os.Remove(airgapInstallBundlePath); err != nil {
		t.Logf("failed to remove airgap install bundle: %v", err)
	}
	if err := os.Remove(airgapUpgradeBundlePath); err != nil {
		t.Logf("failed to remove airgap upgrade bundle: %v", err)
	}

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	// upgrade airgap bundle is only needed on the first node
	line := []string{"rm", "/assets/ec-release-upgrade.tgz"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to remove upgrade airgap bundle on node %s: %v", tc.Nodes[1], err)
	}

	t.Logf("%s: preparing embedded cluster airgap files on node 0", time.Now().Format(time.RFC3339))
	line = []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	// remove artifacts after installation to save space
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on node %s: %v", tc.Nodes[0], err)
	}
	line = []string{"rm", "/var/lib/embedded-cluster/bin/embedded-cluster"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on node %s: %v", tc.Nodes[0], err)
	}

	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	// generate worker node join command.
	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunPlaywrightTest("get-join-worker-command")
	if err != nil {
		t.Fatalf("fail to generate worker join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	workerCommand, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("worker join token command:", workerCommand)

	// join the worker node
	t.Logf("%s: preparing embedded cluster airgap files on worker node", time.Now().Format(time.RFC3339))
	line = []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to prepare airgap files on worker node: %v", err)
	}
	t.Logf("%s: joining worker node to the cluster", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunCommandOnNode(1, strings.Split(workerCommand, " ")); err != nil {
		t.Fatalf("fail to join worker node to the cluster: %v", err)
	}
	// remove artifacts after joining to save space
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on worker node: %v", err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on worker node: %v", err)
	}
	line = []string{"rm", "/var/lib/embedded-cluster/bin/embedded-cluster"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on node %s: %v", tc.Nodes[0], err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = tc.RunCommandOnNode(0, []string{"wait-for-ready-nodes.sh", "2"})
	if err != nil {
		t.Log(stdout)
		t.Fatalf("fail to wait for ready nodes: %v", err)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")), k8sVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}
	// remove the airgap bundle and binary after upgrade
	line = []string{"rm", "/assets/upgrade/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster-upgrade"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster-upgrade binary on node %s: %v", tc.Nodes[0], err)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestMultiNodeAirgapUpgrade(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	initialVersion := fmt.Sprintf("appver-%s-previous-k0s", os.Getenv("SHORT_SHA"))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundle(t, initialVersion, airgapInstallBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                       t,
		Nodes:                   2,
		Image:                   "debian/12",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapInstallBundlePath,
		AirgapUpgradeBundlePath: airgapUpgradeBundlePath,
	})
	defer tc.Cleanup()

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	// delete airgap bundles once they've been copied to the nodes
	if err := os.Remove(airgapInstallBundlePath); err != nil {
		t.Logf("failed to remove airgap install bundle: %v", err)
	}
	if err := os.Remove(airgapUpgradeBundlePath); err != nil {
		t.Logf("failed to remove airgap upgrade bundle: %v", err)
	}

	// upgrade airgap bundle is only needed on the first node
	line := []string{"rm", "/assets/ec-release-upgrade.tgz"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to remove upgrade airgap bundle on node %s: %v", tc.Nodes[1], err)
	}

	t.Logf("%s: preparing embedded cluster airgap files on node 0", time.Now().Format(time.RFC3339))
	line = []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh", initialVersion, "--local-artifact-mirror-port", "50001"} // choose an alternate lam port
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	// remove the airgap bundle and binary after installation
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on node %s: %v", tc.Nodes[0], err)
	}

	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	// generate worker node join command.
	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunPlaywrightTest("get-join-worker-command")
	if err != nil {
		t.Fatalf("fail to generate worker join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	workerCommand, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("worker join token command:", workerCommand)

	// join the worker node
	t.Logf("%s: preparing embedded cluster airgap files on worker node", time.Now().Format(time.RFC3339))
	line = []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to prepare airgap files on worker node: %v", err)
	}
	t.Logf("%s: joining worker node to the cluster", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunCommandOnNode(1, strings.Split(workerCommand, " ")); err != nil {
		t.Fatalf("fail to join worker node to the cluster: %v", err)
	}
	// remove the airgap bundle and binary after joining
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on worker node: %v", err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on worker node: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = tc.RunCommandOnNode(0, []string{"wait-for-ready-nodes.sh", "2"})
	if err != nil {
		t.Log(stdout)
		t.Fatalf("fail to wait for ready nodes: %v", err)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", initialVersion, k8sVersionPrevious()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}
	// remove the airgap bundle and binary after upgrade
	line = []string{"rm", "/assets/upgrade/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster-upgrade"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster-upgrade binary on node %s: %v", tc.Nodes[0], err)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestMultiNodeAirgapUpgradePreviousStable(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	airgapUpgrade2BundlePath := "/tmp/airgap-upgrade2-bundle.tar.gz"
	initialVersion := fmt.Sprintf("appver-%s-previous-stable", os.Getenv("SHORT_SHA"))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundle(t, initialVersion, airgapInstallBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s-noop", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgrade2BundlePath, AirgapLicenseID)
		},
	)

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                        t,
		Nodes:                    2,
		Image:                    "debian/12",
		WithProxy:                true,
		AirgapInstallBundlePath:  airgapInstallBundlePath,
		AirgapUpgradeBundlePath:  airgapUpgradeBundlePath,
		AirgapUpgrade2BundlePath: airgapUpgrade2BundlePath,
		LowercaseNodeNames:       true,
	})
	defer tc.Cleanup()

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	// delete airgap bundles once they've been copied to the nodes
	if err := os.Remove(airgapInstallBundlePath); err != nil {
		t.Logf("failed to remove airgap install bundle: %v", err)
	}
	if err := os.Remove(airgapUpgradeBundlePath); err != nil {
		t.Logf("failed to remove airgap upgrade bundle: %v", err)
	}
	if err := os.Remove(airgapUpgrade2BundlePath); err != nil {
		t.Logf("failed to remove airgap upgrade bundle: %v", err)
	}

	// upgrade airgap bundle is only needed on the first node
	line := []string{"rm", "/assets/ec-release-upgrade.tgz"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to remove upgrade airgap bundle on node %s: %v", tc.Nodes[1], err)
	}

	t.Logf("%s: preparing embedded cluster airgap files on node 0", time.Now().Format(time.RFC3339))
	line = []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh", initialVersion, "--local-artifact-mirror-port", "50001"} // choose an alternate lam port
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	// remove the airgap bundle and binary after installation
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on node %s: %v", tc.Nodes[0], err)
	}

	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	// generate worker node join command.
	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunPlaywrightTest("get-join-worker-command")
	if err != nil {
		t.Fatalf("fail to generate worker join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	workerCommand, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("worker join token command:", workerCommand)

	// join the worker node
	t.Logf("%s: preparing embedded cluster airgap files on worker node", time.Now().Format(time.RFC3339))
	line = []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to prepare airgap files on worker node: %v", err)
	}
	t.Logf("%s: joining worker node to the cluster", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunCommandOnNode(1, strings.Split(workerCommand, " ")); err != nil {
		t.Fatalf("fail to join worker node to the cluster: %v", err)
	}
	// remove the airgap bundle and binary after joining
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on worker node: %v", err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster"}
	if _, _, err := tc.RunCommandOnNode(1, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on worker node: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = tc.RunCommandOnNode(0, []string{"wait-for-ready-nodes.sh", "2"})
	if err != nil {
		t.Log(stdout)
		t.Fatalf("fail to wait for ready nodes: %v", err)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", initialVersion, k8sVersionPreviousStable()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}
	// remove the airgap bundle and binary after upgrade
	line = []string{"rm", "/assets/upgrade/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster-upgrade"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster-upgrade binary on node %s: %v", tc.Nodes[0], err)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-noop", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after noop upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", appUpgradeVersion, k8sVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running second airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update2.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}
	// remove the airgap bundle and binary after upgrade
	line = []string{"rm", "/assets/upgrade2/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster-upgrade2"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster-upgrade2 binary on node %s: %v", tc.Nodes[0], err)
	}

	appUpgradeVersion = fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster a second time", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after second upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// This test creates 4 nodes, installs on the first one and then generate 2 join tokens
// for controllers and one join token for worker nodes. Joins the nodes as HA and then waits
// for them to report ready. Runs additional high availability validations afterwards.
func TestMultiNodeHAInstallation(t *testing.T) {
	tc := docker.NewCluster(&docker.ClusterInput{
		T:                      t,
		Nodes:                  4,
		Distro:                 "debian-bookworm",
		LicensePath:            "licenses/license.yaml",
		ECBinaryPath:           "../output/bin/embedded-cluster",
		SupportBundleNodeIndex: 2,
	})
	defer tc.Cleanup()

	installSingleNode(t, tc)

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
	line := []string{"check-post-ha-state.sh", os.Getenv("SHORT_SHA"), k8sVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post ha state: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	bin := "embedded-cluster"
	t.Logf("%s: resetting controller node 0", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunCommandOnNode(0, []string{bin, "reset", "--yes"})
	if err != nil {
		t.Fatalf("fail to remove controller node 0: %v: %s: %s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "You must maintain at least three controller-test nodes in a high-availability cluster") {
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

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(2, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// This test creates 4 airgap nodes, installs on the first one and then generate 2 join tokens
// for controllers and one join token for worker nodes. Joins the nodes as airgap HA and then waits
// for them to report ready. Runs additional high availability validations afterwards.
func TestMultiNodeAirgapHAInstallation(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")), airgapInstallBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                       t,
		Nodes:                   4,
		Image:                   "debian/12",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapInstallBundlePath,
		AirgapUpgradeBundlePath: airgapUpgradeBundlePath,
		SupportBundleNodeIndex:  2,
	})
	defer tc.Cleanup()

	// delete airgap bundles once they've been copied to the nodes
	if err := os.Remove(airgapInstallBundlePath); err != nil {
		t.Logf("failed to remove airgap install bundle: %v", err)
	}

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	// install "expect" dependency on node 3 as that's where the HA join command will run.
	tc.InstallTestDependenciesDebian(t, 3, true)

	t.Logf("%s: preparing embedded cluster airgap files on node 0", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	checkWorkerProfile(t, tc, 0)

	// remove artifacts after installation to save space
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}
	// do not remove the embedded-cluster binary as it is used for reset

	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")), k8sVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	// join a worker
	joinWorkerNodeWithOptions(t, tc, 1, joinOptions{isAirgap: true})
	checkWorkerProfile(t, tc, 1)

	// join a controller
	joinControllerNodeWithOptions(t, tc, 2, joinOptions{isAirgap: true})
	checkWorkerProfile(t, tc, 2)

	// join another controller in HA mode
	joinControllerNodeWithOptions(t, tc, 3, joinOptions{isAirgap: true, isHA: true})
	checkWorkerProfile(t, tc, 3)

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 4, nil)

	t.Logf("%s: checking installation state after enabling high availability", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-post-ha-state.sh", os.Getenv("SHORT_SHA"), k8sVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post ha state: %v", err)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}
	// remove the airgap bundle and binary after upgrade
	line = []string{"rm", "/assets/upgrade/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	bin := "embedded-cluster"
	t.Logf("%s: resetting controller node 0 with bin %q", time.Now().Format(time.RFC3339), bin)
	stdout, stderr, err := tc.RunCommandOnNode(0, []string{bin, "reset", "--yes"})
	if err != nil {
		t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
		t.Fatalf("fail to remove controller node 0 %s:", err)
	}
	if !strings.Contains(stdout, "You must maintain at least three controller-test nodes in a high-availability cluster") {
		t.Errorf("reset output does not contain the ha warning")
		t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
	}

	stdout, _, err = tc.RunCommandOnNode(2, []string{"check-nodes-removed.sh", "3"})
	if err != nil {
		t.Fatalf("fail to check nodes removed: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking nllb", time.Now().Format(time.RFC3339))
	line = []string{"check-nllb.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(2, line); err != nil {
		t.Fatalf("fail to check nllb: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(2, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestInstallSnapshotFromReplicatedApp(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:      t,
		Nodes:  1,
		Distro: "debian-bookworm",
	})
	defer tc.Cleanup()

	t.Logf("%s: downloading embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"vandoor-prepare.sh", fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")), SnapshotLicenseID, "false"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to download embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	installSingleNode(t, tc)

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationState(t, tc)

	t.Logf("%s: ensuring velero is installed", time.Now().Format(time.RFC3339))
	line = []string{"check-velero-state.sh", os.Getenv("SHORT_SHA")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check velero state: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// TestCustomCIDR tests the installation with an alternate CIDR range
func TestCustomCIDR(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        4,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()
	t.Log("non-proxied infrastructure created")

	installSingleNodeWithOptions(t, tc, installOptions{
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

	checkInstallationState(t, tc)

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

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	installSingleNode(t, tc)

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationState(t, tc)

	appUpgradeVersion := fmt.Sprintf("appver-%s-noop", os.Getenv("SHORT_SHA"))
	skipClusterUpgradeCheck := "true"
	testArgs := []string{appUpgradeVersion, skipClusterUpgradeCheck}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version: appUpgradeVersion,
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestFiveNodesAirgapUpgrade(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	initialVersion := fmt.Sprintf("appver-%s-previous-k0s", os.Getenv("SHORT_SHA"))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundle(t, initialVersion, airgapInstallBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                       t,
		Nodes:                   5,
		Image:                   "debian/12",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapInstallBundlePath,
		AirgapUpgradeBundlePath: airgapUpgradeBundlePath,
	})
	defer tc.Cleanup()

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	// delete airgap bundles once they've been copied to the nodes
	os.Remove(airgapInstallBundlePath)
	os.Remove(airgapUpgradeBundlePath)

	t.Logf("%s: preparing and installing embedded cluster on node 0", time.Now().Format(time.RFC3339))
	installCommands := [][]string{
		{"airgap-prepare.sh"},
		{"single-node-airgap-install.sh", initialVersion},
		{"rm", "/assets/release.airgap"},
		{"rm", "/usr/local/bin/embedded-cluster"},
	}
	if err := tc.RunCommandsOnNode(0, installCommands); err != nil {
		t.Fatalf("failed to install on node %s: %v", tc.Nodes[0], err)
	}

	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	// generate controller node join command.
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunPlaywrightTest("get-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	controllerCommand, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("controller join token command:", controllerCommand)

	// bypass ha prompt
	controllerCommand = strings.Replace(controllerCommand, "join", "join --no-ha", 1)

	// join the controller nodes
	joinCommandsSequence := [][]string{
		{"rm", "/assets/ec-release-upgrade.tgz"},
		{"airgap-prepare.sh"},
		strings.Split(controllerCommand, " "),
		{"rm", "/assets/release.airgap"},
		{"rm", "/usr/local/bin/embedded-cluster"},
	}
	runInParallelOffset(t, time.Second*30,
		func(t *testing.T) error {
			err := tc.RunCommandsOnNode(1, joinCommandsSequence)
			if err != nil {
				return fmt.Errorf("unable to join node 1: %w", err)
			}
			return nil
		}, func(t *testing.T) error {
			err := tc.RunCommandsOnNode(2, joinCommandsSequence)
			if err != nil {
				return fmt.Errorf("unable to join node 2: %w", err)
			}
			return nil
		}, func(t *testing.T) error {
			err := tc.RunCommandsOnNode(3, joinCommandsSequence)
			if err != nil {
				return fmt.Errorf("unable to join node 3: %w", err)
			}
			return nil
		}, func(t *testing.T) error {
			err := tc.RunCommandsOnNode(4, joinCommandsSequence)
			if err != nil {
				return fmt.Errorf("unable to join node 4: %w", err)
			}
			return nil
		},
	)

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 5, nil)

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line := []string{"check-airgap-installation-state.sh", initialVersion, k8sVersionPrevious()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	upgradeCommands := [][]string{
		{"airgap-update.sh"},
		{"rm", "/assets/upgrade/release.airgap"},
		{"rm", "/usr/local/bin/embedded-cluster-upgrade"},
	}
	if err := tc.RunCommandsOnNode(0, upgradeCommands); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	testArgs := []string{fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))}
	if _, _, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestInstallWithPrivateCAs(t *testing.T) {
	RequireEnvVars(t, []string{"SHORT_SHA"})

	input := &lxd.ClusterInput{
		T:                   t,
		Nodes:               1,
		Image:               "ubuntu/jammy",
		LicensePath:         "licenses/license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	}
	tc := lxd.NewCluster(input)
	defer tc.Cleanup()

	certBuilder, err := certs.NewBuilder()
	require.NoError(t, err, "unable to create new cert builder")
	crtContent, _, err := certBuilder.Generate()
	require.NoError(t, err, "unable to build test certificate")

	tmpfile, err := os.CreateTemp("", "test-temp-cert-*.crt")
	require.NoError(t, err, "unable to create temp file")
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString(crtContent)
	require.NoError(t, err, "unable to write to temp file")
	tmpfile.Close()

	lxd.CopyFileToNode(input, tc.Nodes[0], lxd.File{
		SourcePath: tmpfile.Name(),
		DestPath:   "/tmp/ca.crt",
		Mode:       0666,
	})

	installSingleNodeWithOptions(t, tc, installOptions{
		privateCA: "/tmp/ca.crt",
	})

	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	checkInstallationState(t, tc)

	t.Logf("checking if the configmap was created with the right values")
	line := []string{"kubectl", "get", "cm", "kotsadm-private-cas", "-n", "kotsadm", "-o", "json"}
	stdout, _, err := tc.RunCommandOnNode(0, line, lxd.WithECShellEnv("/var/lib/embedded-cluster"))
	require.NoError(t, err, "unable get kotsadm-private-cas configmap")

	var cm corev1.ConfigMap
	err = json.Unmarshal([]byte(stdout), &cm)
	require.NoErrorf(t, err, "unable to unmarshal output to configmap: %q", stdout)
	require.Contains(t, cm.Data, "ca_0.crt", "index ca_0.crt not found in ca secret")
	require.Equal(t, crtContent, cm.Data["ca_0.crt"], "content mismatch")

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestInstallWithConfigValues(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "almalinux-8",
		LicensePath:  "licenses/license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
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
	_, _, err := tc.RunCommandOnNode(0, []string{"mkdir", "-p", "/assets"})
	if err != nil {
		t.Fatalf("fail to create config values file directory: %v", err)
	}
	_, _, err = tc.RunCommandOnNode(0, []string{"echo", "'" + configValuesFileB64 + "'", "|", "base64", "-d", ">", "/assets/config-values.yaml"})
	if err != nil {
		t.Fatalf("fail to create config values file: %v", err)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		configValuesFile: "/assets/config-values.yaml",
	})

	t.Logf("%s: checking config values", time.Now().Format(time.RFC3339))
	line = []string{"check-config-values.sh", hostname, password}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check config values: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationState(t, tc)

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion, "", hostname}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking config values after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-config-values.sh", "updated-hostname.com", "updated password"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check config values: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeAirgapUpgradeConfigValues(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	initialVersion := fmt.Sprintf("appver-%s-previous-k0s", os.Getenv("SHORT_SHA"))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundle(t, initialVersion, airgapInstallBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                       t,
		Nodes:                   1,
		Image:                   "debian/12",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapInstallBundlePath,
		AirgapUpgradeBundlePath: airgapUpgradeBundlePath,
	})
	defer tc.Cleanup()

	// delete airgap bundles once they've been copied to the nodes
	if err := os.Remove(airgapInstallBundlePath); err != nil {
		t.Logf("failed to remove airgap install bundle: %v", err)
	}
	if err := os.Remove(airgapUpgradeBundlePath); err != nil {
		t.Logf("failed to remove airgap upgrade bundle: %v", err)
	}

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
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
	_, _, err := tc.RunCommandOnNode(0, []string{"mkdir", "-p", "/assets"})
	if err != nil {
		t.Fatalf("fail to create config values file directory: %v", err)
	}
	_, _, err = tc.RunCommandOnNode(0, []string{"sh", "-c", "echo '" + configValuesFileB64 + "' | base64 -d > /assets/config-values.yaml"})
	if err != nil {
		t.Fatalf("fail to create config values file: %v", err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh", initialVersion, "--local-artifact-mirror-port", "50001", "--config-values", "/assets/config-values.yaml"} // choose an alternate lam port
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	// remove the airgap bundle after installation
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", initialVersion, k8sVersionPrevious()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: checking config values", time.Now().Format(time.RFC3339))
	line = []string{"check-config-values.sh", hostname, password}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check config values: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}
	// remove the airgap bundle after upgrade
	line = []string{"rm", "/assets/upgrade/release.airgap"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion, "", hostname}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: checking config values after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-config-values.sh", "updated-hostname.com", "updated password"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check config values: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
