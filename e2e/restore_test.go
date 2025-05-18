package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/cmx"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/lxd"
)

func TestSingleNodeDisasterRecovery(t *testing.T) {
	t.Parallel()

	requiredEnvVars := []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
		"DR_S3_ENDPOINT",
		"DR_S3_REGION",
		"DR_S3_BUCKET",
		"DR_S3_PREFIX",
		"DR_ACCESS_KEY_ID",
		"DR_SECRET_ACCESS_KEY",
		"EC_BINARY_PATH",
	}
	RequireEnvVars(t, requiredEnvVars)

	testArgs := []string{
		os.Getenv("DR_S3_ENDPOINT"),
		os.Getenv("DR_S3_REGION"),
		os.Getenv("DR_S3_BUCKET"),
		os.Getenv("DR_S3_PREFIX"),
		os.Getenv("DR_ACCESS_KEY_ID"),
		os.Getenv("DR_SECRET_ACCESS_KEY"),
	}

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/snapshot-license.yaml",
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

	if stdout, stderr, err := tc.RunPlaywrightTest("create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v: %s: %s", err, stdout, stderr)
	}

	resetInstallation(t, tc, 0)

	// wait for the cluster nodes to reboot
	tc.WaitForReady()

	t.Logf("%s: restoring the installation", time.Now().Format(time.RFC3339))
	line := append([]string{"restore-installation.exp"}, testArgs...)
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to restore the installation: %v: %s: %s", err, stdout, stderr)
	}

	line = []string{"collect-support-bundle-host-in-cluster.sh"}
	stdout, stderr, err := tc.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to collect host support bundle: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	t.Logf("%s: checking post-restore state", time.Now().Format(time.RFC3339))
	line = []string{"check-post-restore.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post-restore state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: validating restored app", time.Now().Format(time.RFC3339))
	if err := tc.SetupPlaywright(); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if stdout, stderr, err := tc.RunPlaywrightTest("validate-restore-app"); err != nil {
		t.Fatalf("fail to run playwright test validate-restore-app: %v: %s: %s", err, stdout, stderr)
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

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeLegacyDisasterRecovery(t *testing.T) {
	t.Parallel()

	requiredEnvVars := []string{
		"APP_INSTALL_VERSION",
		"K0S_INSTALL_VERSION",
		"DR_S3_ENDPOINT",
		"DR_S3_REGION",
		"DR_S3_BUCKET",
		"DR_S3_PREFIX",
		"DR_ACCESS_KEY_ID",
		"DR_SECRET_ACCESS_KEY",
	}
	RequireEnvVars(t, requiredEnvVars)

	testArgs := []string{
		os.Getenv("DR_S3_ENDPOINT"),
		os.Getenv("DR_S3_REGION"),
		os.Getenv("DR_S3_BUCKET"),
		os.Getenv("DR_S3_PREFIX"),
		os.Getenv("DR_ACCESS_KEY_ID"),
		os.Getenv("DR_SECRET_ACCESS_KEY"),
	}

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

	if err := tc.SetupPlaywright(); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.RunPlaywrightTest("create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v: %s: %s", err, stdout, stderr)
	}

	resetInstallation(t, tc, 0)

	// wait for the cluster nodes to reboot
	tc.WaitForReady()

	t.Logf("%s: restoring the installation", time.Now().Format(time.RFC3339))
	line := append([]string{"restore-installation.exp"}, testArgs...)
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to restore the installation: %v: %s: %s", err, stdout, stderr)
	}

	line = []string{"collect-support-bundle-host-in-cluster.sh"}
	stdout, stderr, err := tc.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to collect host support bundle: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	t.Logf("%s: validating restored app", time.Now().Format(time.RFC3339))
	t.Logf("%s: validating restored app", time.Now().Format(time.RFC3339))
	if err := tc.SetupPlaywright(); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if stdout, stderr, err := tc.RunPlaywrightTest("validate-restore-app"); err != nil {
		t.Fatalf("fail to run playwright test validate-restore-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeDisasterRecoveryWithProxy(t *testing.T) {
	t.Parallel()
	if SkipProxyTest() {
		t.Skip("skipping test for k0s versions < 1.29.0")
	}

	requiredEnvVars := []string{
		"APP_INSTALL_VERSION",
		"K0S_INSTALL_VERSION",
		"DR_S3_ENDPOINT",
		"DR_S3_REGION",
		"DR_S3_BUCKET",
		"DR_S3_PREFIX",
		"DR_ACCESS_KEY_ID",
		"DR_SECRET_ACCESS_KEY",
		"EC_BINARY_PATH",
	}
	RequireEnvVars(t, requiredEnvVars)

	testArgs := []string{
		os.Getenv("DR_S3_ENDPOINT"),
		os.Getenv("DR_S3_REGION"),
		os.Getenv("DR_S3_BUCKET"),
		os.Getenv("DR_S3_PREFIX"),
		os.Getenv("DR_ACCESS_KEY_ID"),
		os.Getenv("DR_SECRET_ACCESS_KEY"),
	}

	tc := lxd.NewCluster(&lxd.ClusterInput{
		T:            t,
		Nodes:        1,
		Image:        "debian/12",
		WithProxy:    true,
		LicensePath:  "licenses/snapshot-license.yaml",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()

	tc.InstallTestDependenciesDebian(t, 0, true)

	// install kots cli before configuring the proxy.
	t.Logf("%s: installing kots cli on node 0", time.Now().Format(time.RFC3339))
	line := []string{"install-kots-cli.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, lxd.WithProxyEnv(tc.IPs)); err != nil {
		t.Fatalf("fail to install kots cli on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: reconfiguring squid to only allow whitelist access", time.Now().Format(time.RFC3339))
	line = []string{"enable-squid-whitelist.sh"}
	if _, _, err := tc.RunCommandOnProxyNode(t, line); err != nil {
		t.Fatalf("failed to reconfigure squid: %v", err)
	}

	t.Cleanup(func() {
		failOnProxyTCPDenied(t, tc)
	})

	installSingleNodeWithOptions(t, tc, installOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		httpProxy:  lxd.HTTPProxy,
		httpsProxy: lxd.HTTPProxy,
		noProxy:    strings.Join(tc.IPs, ","),
		withEnv:    lxd.WithProxyEnv(tc.IPs),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.RunPlaywrightTest("create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v: %s: %s", err, stdout, stderr)
	}

	resetInstallation(t, tc, 0)

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

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	t.Logf("%s: checking post-restore state", time.Now().Format(time.RFC3339))
	line = []string{"check-post-restore.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post-restore state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: validating restored app", time.Now().Format(time.RFC3339))
	if err := tc.SetupPlaywright(); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if stdout, stderr, err := tc.RunPlaywrightTest("validate-restore-app"); err != nil {
		t.Fatalf("fail to run playwright test validate-restore-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeResumeDisasterRecovery(t *testing.T) {
	t.Parallel()

	requiredEnvVars := []string{
		"APP_INSTALL_VERSION",
		"K0S_INSTALL_VERSION",
		"DR_S3_ENDPOINT",
		"DR_S3_REGION",
		"DR_S3_BUCKET",
		"DR_S3_PREFIX",
		"DR_ACCESS_KEY_ID",
		"DR_SECRET_ACCESS_KEY",
		"EC_BINARY_PATH",
	}
	RequireEnvVars(t, requiredEnvVars)

	testArgs := []string{
		os.Getenv("DR_S3_ENDPOINT"),
		os.Getenv("DR_S3_REGION"),
		os.Getenv("DR_S3_BUCKET"),
		os.Getenv("DR_S3_PREFIX"),
		os.Getenv("DR_ACCESS_KEY_ID"),
		os.Getenv("DR_SECRET_ACCESS_KEY"),
	}

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/snapshot-license.yaml",
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

	if stdout, stderr, err := tc.RunPlaywrightTest("create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v: %s: %s", err, stdout, stderr)
	}

	resetInstallation(t, tc, 0)

	// wait for the cluster nodes to reboot
	tc.WaitForReady()

	t.Logf("%s: restoring the installation", time.Now().Format(time.RFC3339))
	line := append([]string{"resume-restore.exp"}, testArgs...)
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to restore the installation: %v: %s: %s", err, stdout, stderr)
	}

	checkInstallationStateWithOptions(t, tc, installationStateOptions{
		version:    os.Getenv("APP_INSTALL_VERSION"),
		k8sVersion: os.Getenv("K0S_INSTALL_VERSION"),
	})

	t.Logf("%s: checking post-restore state", time.Now().Format(time.RFC3339))
	line = []string{"check-post-restore.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post-restore state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: validating restored app", time.Now().Format(time.RFC3339))
	if err := tc.SetupPlaywright(); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if stdout, stderr, err := tc.RunPlaywrightTest("validate-restore-app"); err != nil {
		t.Fatalf("fail to run playwright test validate-restore-app: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeAirgapDisasterRecovery(t *testing.T) {
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
		InstanceType: "r1.medium",
	})
	defer tc.Cleanup()

	t.Logf("%s: deploying minio on node 0", time.Now().Format(time.RFC3339))
	minio, err := tc.DeployMinio(0)
	if err != nil {
		t.Fatalf("failed to deploy minio on node 0: %v", err)
	}

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_INSTALL_VERSION"), AirgapInstallBundlePath, AirgapSnapshotLicenseID)
		},
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_UPGRADE_VERSION"), AirgapUpgradeBundlePath, AirgapSnapshotLicenseID)
		},
	)

	// install "expect" dependency for the restore process.
	t.Logf("%s: installing expect package on node 0", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{"apt-get", "install", "-y", "expect"}); err != nil {
		t.Fatalf("fail to install expect package on node 0: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: airgapping cluster", time.Now().Format(time.RFC3339))
	if err := tc.Airgap(); err != nil {
		t.Fatalf("failed to airgap cluster: %v", err)
	}

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node 0: %v: %s: %s", err, stdout, stderr)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		isAirgap:    true,
		version:     os.Getenv("APP_INSTALL_VERSION"),
		podCidr:     "10.128.0.0/20",
		serviceCidr: "10.129.0.0/20",
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// DR args to be used for backup and restore
	drArgs := []string{
		minio.Endpoint,
		minio.Region,
		minio.DefaultBucket,
		uuid.New().String(), // prefix
		minio.AccessKey,
		minio.SecretKey,
	}

	if stdout, stderr, err := tc.RunPlaywrightTest("create-backup", drArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	// ensure that the cluster is using the right IP ranges.
	t.Logf("%s: checking service and pod IP addresses", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{"check-cidr-ranges.sh", "^10.128.[0-9]*.[0-9]", "^10.129.[0-9]*.[0-9]"}); err != nil {
		t.Fatalf("fail to check addresses on node 0: %v: %s: %s", err, stdout, stderr)
	}

	resetInstallation(t, tc, 0)

	// wait for reboot
	t.Logf("%s: waiting for nodes to reboot", time.Now().Format(time.RFC3339))
	tc.WaitForReboot()

	// start minio
	t.Logf("%s: starting minio on node 0 after reboot", time.Now().Format(time.RFC3339))
	if err := tc.StartMinio(0, minio); err != nil {
		t.Fatalf("failed to start minio: %v", err)
	}

	t.Logf("%s: restoring the installation", time.Now().Format(time.RFC3339))
	drArgs = append(drArgs, "--pod-cidr", "10.128.0.0/20", "--service-cidr", "10.129.0.0/20")
	line = append([]string{"restore-installation-airgap.exp"}, drArgs...)
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to restore the installation: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after restoring app", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking post-restore state", time.Now().Format(time.RFC3339))
	line = []string{"check-post-restore.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post-restore state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: validating restored app", time.Now().Format(time.RFC3339))
	if err := tc.SetupPlaywright(); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if stdout, stderr, err := tc.RunPlaywrightTest("validate-restore-app"); err != nil {
		t.Fatalf("fail to run playwright test validate-restore-app: %v: %s: %s", err, stdout, stderr)
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

func TestMultiNodeHADisasterRecovery(t *testing.T) {
	t.Parallel()

	requiredEnvVars := []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
		"DR_S3_ENDPOINT",
		"DR_S3_REGION",
		"DR_S3_BUCKET",
		"DR_S3_PREFIX",
		"DR_ACCESS_KEY_ID",
		"DR_SECRET_ACCESS_KEY",
		"EC_BINARY_PATH",
	}
	RequireEnvVars(t, requiredEnvVars)

	testArgs := []string{
		os.Getenv("DR_S3_ENDPOINT"),
		os.Getenv("DR_S3_REGION"),
		os.Getenv("DR_S3_BUCKET"),
		os.Getenv("DR_S3_PREFIX"),
		os.Getenv("DR_ACCESS_KEY_ID"),
		os.Getenv("DR_SECRET_ACCESS_KEY"),
	}

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        3,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/snapshot-license.yaml",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()

	installSingleNodeWithOptions(t, tc, installOptions{
		version: os.Getenv("APP_INSTALL_VERSION"),
	})

	if stdout, stderr, err := tc.SetupPlaywrightAndRunTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// join a controller
	joinControllerNode(t, tc, 1)

	// join another controller in HA mode
	joinControllerNodeWithOptions(t, tc, 2, joinOptions{isHA: true})

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 3, nil)

	t.Logf("%s: checking installation state after enabling high availability", time.Now().Format(time.RFC3339))
	line := []string{"check-post-ha-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post ha state: %v: %s: %s", err, stdout, stderr)
	}

	if stdout, stderr, err := tc.RunPlaywrightTest("create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v: %s: %s", err, stdout, stderr)
	}

	// reset the cluster
	runInParallel(t,
		func(t *testing.T) error {
			stdout, stderr, err := resetInstallationWithError(t, tc, 2, resetInstallationOptions{force: true})
			if err != nil {
				return fmt.Errorf("fail to reset the installation on node 2: %v: %s: %s", err, stdout, stderr)
			}
			return nil
		}, func(t *testing.T) error {
			stdout, stderr, err := resetInstallationWithError(t, tc, 1, resetInstallationOptions{force: true})
			if err != nil {
				return fmt.Errorf("fail to reset the installation on node 1: %v: %s: %s", err, stdout, stderr)
			}
			return nil
		}, func(t *testing.T) error {
			stdout, stderr, err := resetInstallationWithError(t, tc, 0, resetInstallationOptions{force: true})
			if err != nil {
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
	joinControllerNodeWithOptions(t, tc, 1, joinOptions{isRestore: true})

	// join another controller in non-HA mode
	joinControllerNodeWithOptions(t, tc, 2, joinOptions{isRestore: true})

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 3, nil, "true")

	t.Logf("%s: restoring the installation: phase 2", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{"restore-multi-node-phase2.exp"}); err != nil {
		t.Fatalf("fail to restore phase 2 of the installation: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after restoring the high availability backup", time.Now().Format(time.RFC3339))
	line = []string{"check-post-ha-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION"), "true"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post ha state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking post-restore state", time.Now().Format(time.RFC3339))
	line = []string{"check-post-restore.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to check post-restore state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: validating restored app", time.Now().Format(time.RFC3339))
	if err := tc.SetupPlaywright(); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if stdout, stderr, err := tc.RunPlaywrightTest("validate-restore-app"); err != nil {
		t.Fatalf("fail to run playwright test validate-restore-app: %v: %s: %s", err, stdout, stderr)
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

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestMultiNodeAirgapHADisasterRecovery(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"APP_UPGRADE_VERSION",
		"K0S_INSTALL_VERSION",
		"K0S_UPGRADE_VERSION",
		"EC_UPGRADE_VERSION",
	})

	// Use an alternate data directory
	withEnv := map[string]string{
		"EMBEDDED_CLUSTER_BASE_DIR": "/var/lib/ec",
	}

	tc := cmx.NewCluster(&cmx.ClusterInput{
		T:            t,
		Nodes:        3,
		Distribution: "ubuntu",
		Version:      "22.04",
		InstanceType: "r1.medium",
	})
	defer tc.Cleanup(withEnv)

	t.Logf("%s: deploying minio on node 0", time.Now().Format(time.RFC3339))
	minio, err := tc.DeployMinio(0)
	if err != nil {
		t.Fatalf("failed to deploy minio on node 0: %v", err)
	}

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	runInParallel(t,
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_INSTALL_VERSION"), AirgapInstallBundlePath, AirgapSnapshotLicenseID)
		},
		func(t *testing.T) error {
			return downloadAirgapBundleOnNode(t, tc, 0, os.Getenv("APP_UPGRADE_VERSION"), AirgapUpgradeBundlePath, AirgapSnapshotLicenseID)
		},
	)

	// install "expect" dependency on node 0 as that's where the restore process will be initiated.
	t.Logf("%s: installing expect package on node 0", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, []string{"apt-get", "install", "-y", "expect"}); err != nil {
		t.Fatalf("fail to install expect package on node 0: %v: %s: %s", err, stdout, stderr)
	}
	// install "expect" dependency on node 2 as that's where the HA join command will run.
	t.Logf("%s: installing expect package on node 2", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(2, []string{"apt-get", "install", "-y", "expect"}); err != nil {
		t.Fatalf("fail to install expect package on node 2: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: airgapping cluster", time.Now().Format(time.RFC3339))
	if err := tc.Airgap(); err != nil {
		t.Fatalf("failed to airgap cluster: %v", err)
	}

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v: %s: %s", tc.Nodes[0], err, stdout, stderr)
	}

	installSingleNodeWithOptions(t, tc, installOptions{
		isAirgap: true,
		version:  os.Getenv("APP_INSTALL_VERSION"),
		dataDir:  "/var/lib/ec",
		withEnv:  withEnv,
	})

	if err := tc.SetupPlaywright(withEnv); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if stdout, stderr, err := tc.RunPlaywrightTest("deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v: %s: %s", err, stdout, stderr)
	}

	// join a controller
	joinControllerNodeWithOptions(t, tc, 1, joinOptions{
		withEnv: withEnv,
	})

	// join another controller in HA mode
	joinControllerNodeWithOptions(t, tc, 2, joinOptions{
		isHA:    true,
		withEnv: withEnv,
	})

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 3, withEnv)

	t.Logf("%s: checking installation state after enabling high availability", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-post-ha-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION")}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check post ha state: %v: %s: %s", err, stdout, stderr)
	}

	// DR args to be used for backup and restore
	drArgs := []string{
		minio.Endpoint,
		minio.Region,
		minio.DefaultBucket,
		uuid.New().String(), // prefix
		minio.AccessKey,
		minio.SecretKey,
	}

	// create a backup
	if stdout, stderr, err := tc.RunPlaywrightTest("create-backup", drArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v: %s: %s", err, stdout, stderr)
	}

	// reset the cluster
	runInParallel(t,
		func(t *testing.T) error {
			stdout, stderr, err := resetInstallationWithError(t, tc, 2, resetInstallationOptions{force: true, withEnv: withEnv})
			if err != nil {
				return fmt.Errorf("fail to reset the installation on node 2: %v: %s: %s", err, stdout, stderr)
			}
			return nil
		}, func(t *testing.T) error {
			stdout, stderr, err := resetInstallationWithError(t, tc, 1, resetInstallationOptions{force: true, withEnv: withEnv})
			if err != nil {
				return fmt.Errorf("fail to reset the installation on node 1: %v: %s: %s", err, stdout, stderr)
			}
			return nil
		}, func(t *testing.T) error {
			stdout, stderr, err := resetInstallationWithError(t, tc, 0, resetInstallationOptions{force: true, withEnv: withEnv})
			if err != nil {
				return fmt.Errorf("fail to reset the installation on node 0: %v: %s: %s", err, stdout, stderr)
			}
			return nil
		},
	)

	// wait for reboot
	t.Logf("%s: waiting for nodes to reboot", time.Now().Format(time.RFC3339))
	tc.WaitForReboot()

	// start minio
	t.Logf("%s: starting minio on node 0 after reboot", time.Now().Format(time.RFC3339))
	if err := tc.StartMinio(0, minio); err != nil {
		t.Fatalf("failed to start minio: %v", err)
	}

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
	line = append([]string{"restore-multi-node-airgap-phase1.exp"}, drArgs...)
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to restore phase 1 of the installation: %v: %s: %s", err, stdout, stderr)
	}

	// restore phase 1 completes when the prompt for adding nodes is reached.
	// add the expected nodes to the cluster, then continue to phase 2.

	// join a controller
	joinControllerNodeWithOptions(t, tc, 1, joinOptions{
		isRestore: true,
		withEnv:   withEnv,
	})

	// join another controller in non-HA mode
	joinControllerNodeWithOptions(t, tc, 2, joinOptions{
		isRestore: true,
		withEnv:   withEnv,
	})

	// wait for the nodes to report as ready.
	waitForNodes(t, tc, 3, withEnv, "true")

	t.Logf("%s: restoring the installation: phase 2", time.Now().Format(time.RFC3339))
	line = []string{"restore-multi-node-airgap-phase2.exp"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to restore phase 2 of the installation: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after restoring the high availability backup", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-post-ha-state.sh", os.Getenv("APP_INSTALL_VERSION"), os.Getenv("K0S_INSTALL_VERSION"), "true"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check post ha state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking post-restore state", time.Now().Format(time.RFC3339))
	line = []string{"check-post-restore.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, withEnv); err != nil {
		t.Fatalf("fail to check post-restore state: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: validating restored app", time.Now().Format(time.RFC3339))
	if err := tc.SetupPlaywright(withEnv); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if stdout, stderr, err := tc.RunPlaywrightTest("validate-restore-app"); err != nil {
		t.Fatalf("fail to run playwright test validate-restore-app: %v: %s: %s", err, stdout, stderr)
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
		withEnv:    withEnv,
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
