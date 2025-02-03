package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/lxd"
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
	RequireEnvVars(t, requiredEnvVars)

	testArgs := []string{}
	for _, envVar := range requiredEnvVars {
		testArgs = append(testArgs, os.Getenv(envVar))
	}

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "snapshot-license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install.sh", "ui", os.Getenv("SHORT_SHA")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA"), k8sVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	if stdout, stderr, err := tc.RunPlaywrightTest("create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: resetting the installation", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to reset the installation: %v: %s: %s", err, stdout, stderr)
	}

	// wait for the cluster nodes to reboot
	tc.WaitForReady()

	t.Logf("%s: restoring the installation", time.Now().Format(time.RFC3339))
	line = append([]string{"restore-installation.exp"}, testArgs...)
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to restore the installation: %v: %s: %s", err, stdout, stderr)
	}

	line = []string{"collect-support-bundle-host-in-cluster.sh"}
	stdout, stderr, err := tc.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to collect host support bundle: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA"), k8sVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking post-restore state", time.Now().Format(time.RFC3339))
	line = []string{"check-post-restore.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post-restore state: %v: %s: %s", err, stdout, stderr)
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

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeLegacyDisasterRecovery(t *testing.T) {
	t.Parallel()

	requiredEnvVars := []string{
		"DR_AWS_S3_ENDPOINT",
		"DR_AWS_S3_REGION",
		"DR_AWS_S3_BUCKET",
		"DR_AWS_S3_PREFIX",
		"DR_AWS_ACCESS_KEY_ID",
		"DR_AWS_SECRET_ACCESS_KEY",
	}
	RequireEnvVars(t, requiredEnvVars)

	testArgs := []string{}
	for _, envVar := range requiredEnvVars {
		testArgs = append(testArgs, os.Getenv(envVar))
	}

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "snapshot-license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster-legacydr",
	})
	defer tc.Cleanup()

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install.sh", "ui", os.Getenv("SHORT_SHA")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	if err := tc.SetupPlaywright(); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := tc.RunPlaywrightTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	appVersion := fmt.Sprintf("appver-%s-legacydr", os.Getenv("SHORT_SHA"))

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", appVersion, k8sVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	if stdout, stderr, err := tc.RunPlaywrightTest("create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: resetting the installation", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to reset the installation: %v: %s: %s", err, stdout, stderr)
	}

	// wait for the cluster nodes to reboot
	tc.WaitForReady()

	t.Logf("%s: restoring the installation", time.Now().Format(time.RFC3339))
	line = append([]string{"restore-installation.exp"}, testArgs...)
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to restore the installation: %v: %s: %s", err, stdout, stderr)
	}

	line = []string{"collect-support-bundle-host-in-cluster.sh"}
	stdout, stderr, err := tc.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to collect host support bundle: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", appVersion, k8sVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeDisasterRecoveryWithProxy(t *testing.T) {
	t.Parallel()
	if SkipProxyTest() {
		t.Skip("skipping test for k0s versions < 1.29.0")
	}

	requiredEnvVars := []string{
		"DR_AWS_S3_ENDPOINT",
		"DR_AWS_S3_REGION",
		"DR_AWS_S3_BUCKET",
		"DR_AWS_S3_PREFIX",
		"DR_AWS_ACCESS_KEY_ID",
		"DR_AWS_SECRET_ACCESS_KEY",
	}
	RequireEnvVars(t, requiredEnvVars)

	testArgs := []string{}
	for _, envVar := range requiredEnvVars {
		testArgs = append(testArgs, os.Getenv(envVar))
	}

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                   t,
		Nodes:               1,
		Image:               "debian/12",
		WithProxy:           true,
		LicensePath:         "snapshot-license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	tc.InstallTestDependenciesDebian(t, 0, true)

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install.sh", "ui", os.Getenv("SHORT_SHA")}
	line = append(line, "--http-proxy", lxd.HTTPProxy)
	line = append(line, "--https-proxy", lxd.HTTPProxy)
	line = append(line, "--no-proxy", strings.Join(tc.IPs, ","))
	if _, _, err := tc.RunCommandOnNode(0, line, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA"), k8sVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	if _, _, err := tc.RunPlaywrightTest("create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v", err)
	}

	t.Logf("%s: resetting the installation", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to reset the installation: %v", err)
	}

	t.Logf("%s: waiting for nodes to reboot", time.Now().Format(time.RFC3339))
	time.Sleep(30 * time.Second)

	t.Logf("%s: restoring the installation", time.Now().Format(time.RFC3339))
	line = append([]string{"restore-installation.exp"}, testArgs...)
	line = append(line, "--http-proxy", lxd.HTTPProxy)
	line = append(line, "--https-proxy", lxd.HTTPProxy)
	line = append(line, "--no-proxy", strings.Join(tc.IPs, ","))
	if _, _, err := tc.RunCommandOnNode(0, line, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to restore the installation: %v", err)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA"), k8sVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: checking post-restore state", time.Now().Format(time.RFC3339))
	line = []string{"check-post-restore.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post-restore state: %v: %s: %s", err, stdout, stderr)
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
	RequireEnvVars(t, requiredEnvVars)

	testArgs := []string{}
	for _, envVar := range requiredEnvVars {
		testArgs = append(testArgs, os.Getenv(envVar))
	}

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "snapshot-license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install.sh", "ui", os.Getenv("SHORT_SHA")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA"), k8sVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	if stdout, stderr, err := tc.RunPlaywrightTest("create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: resetting the installation", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to reset the installation: %v: %s: %s", err, stdout, stderr)
	}

	// wait for the cluster nodes to reboot
	tc.WaitForReady()

	t.Logf("%s: restoring the installation", time.Now().Format(time.RFC3339))
	line = append([]string{"resume-restore.exp"}, testArgs...)
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to restore the installation: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA"), k8sVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeAirgapDisasterRecovery(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA", "AIRGAP_SNAPSHOT_LICENSE_ID"})

	requiredEnvVars := []string{
		"DR_AWS_S3_ENDPOINT",
		"DR_AWS_S3_REGION",
		"DR_AWS_S3_BUCKET",
		"DR_AWS_S3_PREFIX_AIRGAP",
		"DR_AWS_ACCESS_KEY_ID",
		"DR_AWS_SECRET_ACCESS_KEY",
	}
	RequireEnvVars(t, requiredEnvVars)

	testArgs := []string{}
	for _, envVar := range requiredEnvVars {
		testArgs = append(testArgs, os.Getenv(envVar))
	}

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s-previous-k0s", os.Getenv("SHORT_SHA")), airgapInstallBundlePath, os.Getenv("AIRGAP_SNAPSHOT_LICENSE_ID"))
		}, func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath, os.Getenv("AIRGAP_SNAPSHOT_LICENSE_ID"))
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

	// install "curl" dependency on node 0 for app version checks.
	tc.InstallTestDependenciesDebian(t, 0, true)

	// delete airgap bundles once they've been copied to the nodes
	if err := os.Remove(airgapInstallBundlePath); err != nil {
		t.Logf("failed to remove airgap install bundle: %v", err)
	}
	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh", os.Getenv("SHORT_SHA"), "--proxy"}
	line = append(line, "--pod-cidr", "10.128.0.0/20")
	line = append(line, "--service-cidr", "10.129.0.0/20")
	if _, _, err := tc.RunCommandOnNode(0, line, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	if _, _, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}
	if _, _, err := tc.RunPlaywrightTest("create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v", err)
	}
	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", fmt.Sprintf("appver-%s-previous-k0s", os.Getenv("SHORT_SHA")), k8sVersionPrevious()}
	stdout, _, err := tc.RunCommandOnNode(0, line)
	if err != nil {
		t.Log(stdout)
		t.Fatalf("fail to check installation state: %v", err)
	}
	// ensure that the cluster is using the right IP ranges.
	t.Logf("%s: checking service and pod IP addresses", time.Now().Format(time.RFC3339))
	stdout, _, err = tc.RunCommandOnNode(0, []string{"check-cidr-ranges.sh", "^10.128.[0-9]*.[0-9]", "^10.129.[0-9]*.[0-9]"})
	if err != nil {
		t.Log(stdout)
		t.Fatalf("fail to check addresses on node %s: %v", tc.Nodes[0], err)
	}
	t.Logf("%s: resetting the installation", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to reset the installation: %v", err)
	}

	t.Logf("%s: waiting for nodes to reboot", time.Now().Format(time.RFC3339))
	time.Sleep(30 * time.Second)

	tc.InstallTestDependenciesDebian(t, 0, true)
	t.Logf("%s: restoring the installation", time.Now().Format(time.RFC3339))
	testArgs = append(testArgs, "--pod-cidr", "10.128.0.0/20", "--service-cidr", "10.129.0.0/20")
	line = append([]string{"restore-installation-airgap.exp"}, testArgs...)
	if _, _, err := tc.RunCommandOnNode(0, line, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to restore the installation: %v", err)
	}
	t.Logf("%s: checking installation state after restoring app", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", fmt.Sprintf("appver-%s-previous-k0s", os.Getenv("SHORT_SHA")), k8sVersionPrevious()}
	if _, _, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: checking post-restore state", time.Now().Format(time.RFC3339))
	line = []string{"check-post-restore.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post-restore state: %v: %s: %s", err, stdout, stderr)
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
	testArgs = []string{appUpgradeVersion}

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

func TestMultiNodeHADisasterRecovery(t *testing.T) {
	t.Parallel()

	requiredEnvVars := []string{
		"DR_AWS_S3_ENDPOINT",
		"DR_AWS_S3_REGION",
		"DR_AWS_S3_BUCKET",
		"DR_AWS_S3_PREFIX",
		"DR_AWS_ACCESS_KEY_ID",
		"DR_AWS_SECRET_ACCESS_KEY",
	}
	RequireEnvVars(t, requiredEnvVars)

	testArgs := []string{}
	for _, envVar := range requiredEnvVars {
		testArgs = append(testArgs, os.Getenv(envVar))
	}

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        3,
		Distro:       "debian-bookworm",
		LicensePath:  "snapshot-license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install.sh", "ui", os.Getenv("SHORT_SHA")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// join a controller
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunPlaywrightTest("get-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v: %s: %s", err, stdout, stderr)
	}
	t.Log("controller join token command:", command)
	t.Logf("%s: joining node 1 to the cluster (controller)", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(1, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 1 as a controller: %v: %s: %s", err, stdout, stderr)
	}

	// join another controller in HA mode
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err = tc.RunPlaywrightTest("get-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err = findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v: %s: %s", err, stdout, stderr)
	}
	t.Log("controller join token command:", command)
	t.Logf("%s: joining node 2 to the cluster (controller) in ha mode", time.Now().Format(time.RFC3339))
	line = []string{"join-ha.exp", fmt.Sprintf("'%s'", command)} // pass join command as a single argument
	if stdout, stderr, err := tc.RunCommandOnNode(2, line); err != nil {
		t.Fatalf("fail to join node 2 as a controller in ha mode: %v: %s: %s", err, stdout, stderr)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, stderr, err = tc.RunCommandOnNode(0, []string{"wait-for-ready-nodes.sh", "3"})
	if err != nil {
		t.Fatalf("fail to wait for ready nodes: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after enabling high availability", time.Now().Format(time.RFC3339))
	line = []string{"check-post-ha-state.sh", os.Getenv("SHORT_SHA"), k8sVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post ha state: %v: %s: %s", err, stdout, stderr)
	}

	if stdout, stderr, err := tc.RunPlaywrightTest("create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v: %s: %s", err, stdout, stderr)
	}

	// reset the cluster
	runInParallel(t,
		func(t *testing.T) error {
			t.Logf("%s: resetting the installation on node 2", time.Now().Format(time.RFC3339))
			line = []string{"reset-installation.sh", "--force"}
			if stdout, stderr, err := tc.RunCommandOnNode(2, line); err != nil {
				return fmt.Errorf("fail to reset the installation on node 2: %v: %s: %s", err, stdout, stderr)
			}
			return nil
		}, func(t *testing.T) error {
			t.Logf("%s: resetting the installation on node 1", time.Now().Format(time.RFC3339))
			line = []string{"reset-installation.sh", "--force"}
			if stdout, stderr, err := tc.RunCommandOnNode(1, line); err != nil {
				return fmt.Errorf("fail to reset the installation on node 1: %v: %s: %s", err, stdout, stderr)
			}
			return nil
		}, func(t *testing.T) error {
			t.Logf("%s: resetting the installation on node 0", time.Now().Format(time.RFC3339))
			line = []string{"reset-installation.sh", "--force"}
			if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
				return fmt.Errorf("fail to reset the installation on node 0: %v: %s: %s", err, stdout, stderr)
			}
			return nil
		},
	)

	// wait for the cluster nodes to reboot
	tc.WaitForReady()

	// begin restoring the cluster
	t.Logf("%s: restoring the installation: phase 1", time.Now().Format(time.RFC3339))
	line = append([]string{"restore-multi-node-phase1.exp"}, testArgs...)
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to restore phase 1 of the installation: %v: %s: %s", err, stdout, stderr)
	}

	// restore phase 1 completes when the prompt for adding nodes is reached.
	// add the expected nodes to the cluster, then continue to phase 2.

	// join a controller
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err = tc.RunPlaywrightTest("get-restore-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err = findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v: %s: %s", err, stdout, stderr)
	}
	t.Log("controller join token command:", command)
	t.Logf("%s: joining node 1 to the cluster (controller)", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(1, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 1 as a controller: %v: %s: %s", err, stdout, stderr)
	}

	// join another controller in non-HA mode
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err = tc.RunPlaywrightTest("get-restore-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err = findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v: %s: %s", err, stdout, stderr)
	}
	t.Log("controller join token command:", command)
	t.Logf("%s: joining node 2 to the cluster (controller)", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(2, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 2 as a controller: %v: %s: %s", err, stdout, stderr)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, stderr, err = tc.RunCommandOnNode(0, []string{"wait-for-ready-nodes.sh", "3", "true"})
	if err != nil {
		t.Fatalf("fail to wait for ready nodes: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: restoring the installation: phase 2", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{"restore-multi-node-phase2.exp"}); err != nil {
		t.Fatalf("fail to restore phase 2 of the installation: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after restoring the high availability backup", time.Now().Format(time.RFC3339))
	line = []string{"check-post-ha-state.sh", os.Getenv("SHORT_SHA"), k8sVersion(), "true"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post ha state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking post-restore state", time.Now().Format(time.RFC3339))
	line = []string{"check-post-restore.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post-restore state: %v: %s: %s", err, stdout, stderr)
	}

	appUpgradeVersion := fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA"))
	testArgs = []string{appUpgradeVersion}

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

func TestMultiNodeAirgapHADisasterRecovery(t *testing.T) {
	t.Parallel()

	requiredEnvVars := []string{
		"DR_AWS_S3_ENDPOINT",
		"DR_AWS_S3_REGION",
		"DR_AWS_S3_BUCKET",
		"DR_AWS_S3_PREFIX_AIRGAP",
		"DR_AWS_ACCESS_KEY_ID",
		"DR_AWS_SECRET_ACCESS_KEY",
	}
	RequireEnvVars(t, requiredEnvVars)

	testArgs := []string{}
	for _, envVar := range requiredEnvVars {
		testArgs = append(testArgs, os.Getenv(envVar))
	}

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s-previous-k0s", os.Getenv("SHORT_SHA")), airgapInstallBundlePath, os.Getenv("AIRGAP_SNAPSHOT_LICENSE_ID"))
		}, func(t *testing.T) error {
			return downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath, os.Getenv("AIRGAP_SNAPSHOT_LICENSE_ID"))
		},
	)

	// Use an alternate data directory
	withEnv := map[string]string{
		"EMBEDDED_CLUSTER_BASE_DIR": "/var/lib/ec",
	}

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:                       t,
		Nodes:                   3,
		Image:                   "debian/12",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapInstallBundlePath,
		AirgapUpgradeBundlePath: airgapUpgradeBundlePath,
	})
	defer tc.Cleanup(withEnv)

	// install "expect" dependency on node 0 as that's where the restore process will be initiated.
	// install "expect" dependency on node 2 as that's where the HA join command will run.
	tc.InstallTestDependenciesDebian(t, 0, true)
	tc.InstallTestDependenciesDebian(t, 2, true)

	// delete airgap bundles once they've been copied to the nodes
	if err := os.Remove(airgapInstallBundlePath); err != nil {
		t.Logf("failed to remove airgap install bundle: %v", err)
	}

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh", os.Getenv("SHORT_SHA"), "--proxy", "--data-dir", "/var/lib/ec"}
	if _, _, err := tc.RunCommandOnNode(0, line, withEnv, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if err := tc.SetupPlaywright(withEnv); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := tc.RunPlaywrightTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	// join a controller
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunPlaywrightTest("get-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("controller join token command:", command)
	t.Logf("%s: preparing embedded cluster airgap files on node 1", time.Now().Format(time.RFC3339))
	line = []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(1, line, withEnv); err != nil {
		t.Fatalf("fail to prepare airgap files on node 1: %v", err)
	}
	t.Logf("%s: joining node 1 to the cluster (controller)", time.Now().Format(time.RFC3339))
	line = strings.Split(command, " ")
	if _, _, err := tc.RunCommandOnNode(1, line, withEnv); err != nil {
		t.Fatalf("fail to join node 1 as a controller: %v", err)
	}

	// join another controller in HA mode
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err = tc.RunPlaywrightTest("get-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err = findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("controller join token command:", command)
	t.Logf("%s: preparing embedded cluster airgap files on node 2", time.Now().Format(time.RFC3339))
	line = []string{"airgap-prepare.sh"}
	if _, _, err := tc.RunCommandOnNode(2, line, withEnv); err != nil {
		t.Fatalf("fail to prepare airgap files on node 2: %v", err)
	}
	t.Logf("%s: joining node 2 to the cluster (controller) in ha mode", time.Now().Format(time.RFC3339))
	line = append([]string{"join-ha.exp"}, []string{command}...)
	if _, _, err := tc.RunCommandOnNode(2, line, withEnv); err != nil {
		t.Fatalf("fail to join node 2 as a controller in ha mode: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	line = []string{"wait-for-ready-nodes.sh", "3"}
	stdout, _, err = tc.RunCommandOnNode(0, line, withEnv)
	if err != nil {
		t.Log(stdout)
		t.Fatalf("fail to wait for ready nodes: %v", err)
	}

	t.Logf("%s: checking installation state after enabling high availability", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-post-ha-state.sh", os.Getenv("SHORT_SHA"), k8sVersion()}
	if _, _, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check post ha state: %v", err)
	}

	if _, _, err := tc.RunPlaywrightTest("create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v", err)
	}

	// reset the cluster
	runInParallel(t,
		func(t *testing.T) error {
			t.Logf("%s: resetting the installation on node 2", time.Now().Format(time.RFC3339))
			line = []string{"reset-installation.sh", "--force"}
			if _, _, err := tc.RunCommandOnNode(2, line, withEnv); err != nil {
				return fmt.Errorf("fail to reset the installation on node 2: %v", err)
			}
			return nil
		}, func(t *testing.T) error {
			t.Logf("%s: resetting the installation on node 1", time.Now().Format(time.RFC3339))
			line = []string{"reset-installation.sh", "--force"}
			if _, _, err := tc.RunCommandOnNode(1, line, withEnv); err != nil {
				return fmt.Errorf("fail to reset the installation on node 1: %v", err)
			}
			return nil
		}, func(t *testing.T) error {
			t.Logf("%s: resetting the installation on node 0", time.Now().Format(time.RFC3339))
			line = []string{"reset-installation.sh", "--force"}
			if _, _, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
				return fmt.Errorf("fail to reset the installation on node 0: %v", err)
			}
			return nil
		},
	)

	// wait for reboot
	t.Logf("%s: waiting for nodes to reboot", time.Now().Format(time.RFC3339))
	time.Sleep(60 * time.Second)

	runInParallel(t,
		func(t *testing.T) error {
			t.Logf("%s: checking that /var/lib/ec is empty on node 0", time.Now().Format(time.RFC3339))
			line := []string{"check-directory-empty.sh", "/var/lib/ec"}
			if _, _, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
				return fmt.Errorf("fail to check that /var/lib/ec is empty: %v", err)
			}
			return nil
		}, func(t *testing.T) error {
			t.Logf("%s: checking that /var/lib/ec is empty on node 1", time.Now().Format(time.RFC3339))
			line := []string{"check-directory-empty.sh", "/var/lib/ec"}
			if _, _, err := tc.RunCommandOnNode(1, line, withEnv); err != nil {
				return fmt.Errorf("fail to check that /var/lib/ec is empty: %v", err)
			}
			return nil
		}, func(t *testing.T) error {
			t.Logf("%s: checking that /var/lib/ec is empty on node 2", time.Now().Format(time.RFC3339))
			line := []string{"check-directory-empty.sh", "/var/lib/ec"}
			if _, _, err := tc.RunCommandOnNode(2, line, withEnv); err != nil {
				return fmt.Errorf("fail to check that /var/lib/ec is empty: %v", err)
			}
			return nil
		},
	)

	// begin restoring the cluster
	t.Logf("%s: restoring the installation: phase 1", time.Now().Format(time.RFC3339))
	line = append([]string{"restore-multi-node-airgap-phase1.exp"}, testArgs...)
	if _, _, err := tc.RunCommandOnNode(0, line, withEnv, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to restore phase 1 of the installation: %v", err)
	}

	// restore phase 1 completes when the prompt for adding nodes is reached.
	// add the expected nodes to the cluster, then continue to phase 2.

	// join a controller
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err = tc.RunPlaywrightTest("get-restore-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err = findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("controller join token command:", command)
	t.Logf("%s: joining node 1 to the cluster (controller)", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunCommandOnNode(1, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 1 as a controller: %v", err)
	}

	// join another controller in non-HA mode
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err = tc.RunPlaywrightTest("get-restore-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err = findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("controller join token command:", command)
	t.Logf("%s: joining node 2 to the cluster (controller)", time.Now().Format(time.RFC3339))
	if _, _, err := tc.RunCommandOnNode(2, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 2 as a controller: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = tc.RunCommandOnNode(0, []string{"wait-for-ready-nodes.sh", "3", "true"}, withEnv)
	if err != nil {
		t.Log(stdout)
		t.Fatalf("fail to wait for ready nodes: %v", err)
	}

	t.Logf("%s: restoring the installation: phase 2", time.Now().Format(time.RFC3339))
	line = []string{"restore-multi-node-airgap-phase2.exp"}
	if _, _, err := tc.RunCommandOnNode(0, line, withEnv, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to restore phase 2 of the installation: %v", err)
	}

	t.Logf("%s: checking installation state after restoring the high availability backup", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-post-ha-state.sh", os.Getenv("SHORT_SHA"), k8sVersion(), "true"}
	if _, _, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check post ha state: %v", err)
	}

	t.Logf("%s: checking post-restore state", time.Now().Format(time.RFC3339))
	line = []string{"check-post-restore.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check post-restore state: %v: %s: %s", err, stdout, stderr)
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
	testArgs = []string{appUpgradeVersion}

	t.Logf("%s: upgrading cluster", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-upgrade", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", k8sVersion(), ecUpgradeTargetVersion()}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check postupgrade state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
