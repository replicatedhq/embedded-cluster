package e2e

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/cmx"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
	"github.com/stretchr/testify/require"
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
	checkNodeJoinCommand(t, tc, 0)

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeState(t, tc)

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

func TestSingleNodeUpgradePreviousStable(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:      t,
		Nodes:  1,
		Distro: "debian-bookworm",
	})
	defer tc.Cleanup()

	// Previous stable EC version with a -1 minor k0s version
	initialVersion := fmt.Sprintf("appver-%s-previous-stable", os.Getenv("SHORT_SHA"))

	downloadECReleaseWithOptions(t, tc, 0, downloadECReleaseOptions{
		version: initialVersion,
	})

	installSingleNodeWithOptions(t, tc, installOptions{
		version: initialVersion,
	})

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
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: re-installing kots cli on node 0", time.Now().Format(time.RFC3339))
	line := []string{"install-kots-cli.sh"}
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
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeState(t, tc)

	line = []string{"collect-support-bundle-host-in-cluster.sh"}
	stdout, stderr, err := tc.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to collect host support bundle: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// TestUpgradeFromReplicatedAppPreviousK0s step upgrades from k0s minor-3 to minor-2 to minor-1
func TestUpgradeFromReplicatedAppPreviousK0s(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:      t,
		Nodes:  1,
		Distro: "debian-bullseye",
	})
	defer tc.Cleanup()

	initialVersion := fmt.Sprintf("appver-%s-previous-k0s-3", os.Getenv("SHORT_SHA"))

	downloadECReleaseWithOptions(t, tc, 0, downloadECReleaseOptions{
		version: initialVersion,
	})

	installSingleNodeWithOptions(t, tc, installOptions{
		version: initialVersion,
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    initialVersion,
		k8sVersion: k8sVersionPrevious(3),
	})

	appUpgradeVersion := fmt.Sprintf("appver-%s-previous-k0s-2", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    appUpgradeVersion,
		k8sVersion: k8sVersionPrevious(2),
	})

	appUpgradeVersion = fmt.Sprintf("appver-%s-previous-k0s-1", os.Getenv("SHORT_SHA"))
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    appUpgradeVersion,
		k8sVersion: k8sVersionPrevious(1),
	})

	line := []string{"collect-support-bundle-host-in-cluster.sh"}
	stdout, stderr, err := tc.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to collect host support bundle: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeAirgapUpgradeSelinux(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := cmx.NewCluster(&cmx.ClusterInput{
		T:            t,
		Nodes:        1,
		Distribution: "almalinux",
		Version:      "8",
	})
	defer tc.Cleanup()

	t.Logf("%s: downloading airgap files on node 0", time.Now().Format(time.RFC3339))
	// Previous stable EC version with a -1 minor k0s version
	initialVersion := fmt.Sprintf("appver-%s-previous-stable", os.Getenv("SHORT_SHA"))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, initialVersion, AirgapInstallBundlePath, AirgapLicenseID)
		}, func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), AirgapUpgradeBundlePath, AirgapLicenseID)
		},
	)

	t.Logf("%s: installing policycoreutils-python-utils", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{"sudo dnf makecache --refresh && sudo dnf install -y policycoreutils-python-utils"}); err != nil {
		t.Fatalf("fail to install policycoreutils-python-utils on node %s: %v: %s: %s", tc.Nodes[0], err, stdout, stderr)
	}

	t.Logf("%s: airgapping cluster", time.Now().Format(time.RFC3339))
	if err := tc.Airgap(); err != nil {
		t.Fatalf("failed to airgap cluster: %v", err)
	}

	t.Logf("%s: setting selinux to Enforcing mode", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{"setenforce 1"}); err != nil {
		t.Fatalf("fail to set selinux to Enforcing mode %s: %v: %s: %s", tc.Nodes[0], err, stdout, stderr)
	}

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"/usr/local/bin/airgap-prepare.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v: %s: %s", tc.Nodes[0], err, stdout, stderr)
	}

	t.Logf("%s: correcting selinux label for embedded cluster binary directory", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{"sudo semanage fcontext -a -t bin_t \"/var/lib/embedded-cluster/bin(/.*)?\""}); err != nil {
		t.Fatalf("fail to correct selinux label for embedded cluster binary directory on node %s: %v: %s: %s", tc.Nodes[0], err, stdout, stderr)
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
	line = []string{"/usr/local/bin/check-airgap-installation-state.sh", initialVersion, k8sVersionPreviousStable()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	checkNodeJoinCommand(t, tc, 0)

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"/usr/local/bin/airgap-update.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs := []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeState(t, tc)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestMultiNodeAirgapUpgradePreviousStable(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

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
	initialVersion := fmt.Sprintf("appver-%s-previous-stable", os.Getenv("SHORT_SHA"))
	upgradeVersion := fmt.Sprintf("appver-%s-noop", os.Getenv("SHORT_SHA"))
	upgrade2Version := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, initialVersion, AirgapInstallBundlePath, AirgapLicenseID)
		},
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, upgradeVersion, AirgapUpgradeBundlePath, AirgapLicenseID)
		},
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, upgrade2Version, AirgapUpgrade2BundlePath, AirgapLicenseID)
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
		version:                 initialVersion,
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
	line = []string{"check-airgap-installation-state.sh", initialVersion, k8sVersionPreviousStable()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to run airgap update: %v: %s: %s", err, stdout, stderr)
	}

	testArgs := []string{upgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after noop upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", upgradeVersion, k8sVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running second airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update2.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to run airgap update: %v: %s: %s", err, stdout, stderr)
	}

	testArgs = []string{upgrade2Version}

	t.Logf("%s: upgrading cluster a second time", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{
		withEnv: withEnv,
	})

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
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeState(t, tc)

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
		node: 2,
		withEnv: map[string]string{
			"ALLOW_PENDING_PODS": "true",
		},
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstallationNoopUpgrade(t *testing.T) {
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
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version: appUpgradeVersion,
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestFiveNodesAirgapUpgrade(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := cmx.NewCluster(&cmx.ClusterInput{
		T:            t,
		Nodes:        5,
		Distribution: "ubuntu",
		Version:      "22.04",
		InstanceType: "r1.medium",
	})
	defer tc.Cleanup()

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	// Previous stable EC version with a -1 minor k0s version
	initialVersion := fmt.Sprintf("appver-%s-previous-stable", os.Getenv("SHORT_SHA"))
	upgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, initialVersion, AirgapInstallBundlePath, AirgapLicenseID)
		},
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, upgradeVersion, AirgapUpgradeBundlePath, AirgapLicenseID)
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
		version:  initialVersion,
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
	line = []string{"check-airgap-installation-state.sh", initialVersion, k8sVersionPreviousStable()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	testArgs := []string{fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))}
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeState(t, tc)

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
		t.Fatalf("fail to run playwright test deploy-upgrade: %v: %s: %s", err, stdout, stderr)
	}

	checkPostUpgradeState(t, tc)

	t.Logf("%s: checking config values after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-config-values.sh", "updated-hostname.com", "updated password"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check config values: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

//Temporarily disable network test until the reporting is enriched to the point where we can properly filter out domains as part of a CNAME chain
/*func TestSingleNodeNetworkReport(t *testing.T) {
	t.Parallel()
	RequireEnvVars(t, []string{"SHORT_SHA"})
	tc := cmx.NewCluster(&cmx.ClusterInput{
		T:            t,
		Nodes:        1,
		Distribution: "ubuntu",
		Version:      "22.04",
		InstanceType: "r1.medium",
	})
	defer tc.Cleanup()

	if err := tc.NPMInstallPlaywright(); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}

	downloadECRelease(t, tc, 0)

	// install kots cli before starting the network report.
	if err := tc.InstallKotsCLI(0); err != nil {
		t.Fatalf("fail to install kots cli on node 0: %v", err)
	}

	if err := tc.SetNetworkReport(true); err != nil {
		t.Fatalf("failed to enable network reporting: %v", err)
	}

	installSingleNode(t, tc)

	if err := tc.BypassKurlProxy(); err != nil {
		t.Fatalf("fail to bypass kurl-proxy: %v", err)
	}

	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationState(t, tc)
	checkNodeJoinCommand(t, tc, 0)

	// TODO: network events can came a few seconds to flow from cluster-provisioner, should look into ways to signal when a report has finished
	time.Sleep(20 * time.Second)

	if err := tc.SetNetworkReport(false); err != nil {
		t.Fatalf("failed to disable network reporting: %v", err)
	}

	networkEvents, err := tc.CollectNetworkReport()
	if err != nil {
		t.Fatalf("failed to collect network report: %v", err)
	}

	allowedDomains := map[string]struct{}{
		"ec-e2e-proxy.testcluster.net":          {},
		"ec-e2e-replicated-app.testcluster.net": {},
	}

	seenAllowedDomains := map[string]struct{}{}
	t.Log("Logged outbound external network accesses:")
	for _, ne := range networkEvents {
		if ne.DNSQueryName == "" {
			continue
		}

		// TODO: currently cmx reporting will return an ip as a domain, remove this once fixed
		if ip := net.ParseIP(ne.DNSQueryName); ip != nil {
			continue
		}

		_, allowed := allowedDomains[ne.DNSQueryName]
		// only print allowed domains once to reduce test output noise, but print every violation we see
		if allowed {
			if _, ok := seenAllowedDomains[ne.DNSQueryName]; !ok {
				t.Logf("%v - ALLOWED", ne.DNSQueryName)
				seenAllowedDomains[ne.DNSQueryName] = struct{}{}
			}
		} else {
			t.Logf("%v - UNALLOWED\n", ne.DNSQueryName)
			t.Logf("\tUnallowed event details: %+v", ne)
			t.Fail()
		}
	}
}*/
