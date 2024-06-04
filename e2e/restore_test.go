package e2e

import (
	"fmt"
	"os"
	"sync"
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
		Image:               "debian/12",
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
		Image:               "debian/12",
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

func TestSingleNodeAirgapDisasterRecovery(t *testing.T) {
	t.Parallel()
	requiredEnvVars := []string{
		"DR_AWS_S3_ENDPOINT",
		"DR_AWS_S3_REGION",
		"DR_AWS_S3_BUCKET",
		"DR_AWS_S3_PREFIX_AIRGAP",
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

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		downloadAirgapBundle(t, fmt.Sprintf("appver-%s-previous-k0s", os.Getenv("SHORT_SHA")), airgapInstallBundlePath, os.Getenv("AIRGAP_SNAPSHOT_LICENSE_ID"))
		wg.Done()
	}()
	go func() {
		downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath, os.Getenv("AIRGAP_SNAPSHOT_LICENSE_ID"))
		wg.Done()
	}()
	wg.Wait()

	tc := cluster.NewTestCluster(&cluster.Input{
		T:                       t,
		Nodes:                   1,
		Image:                   "debian/12",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapInstallBundlePath,
		AirgapUpgradeBundlePath: airgapUpgradeBundlePath,
	})
	defer cleanupCluster(t, tc)

	// install "curl" dependency on node 0 for app version checks.
	t.Logf("%s: installing test dependencies on node 0", time.Now().Format(time.RFC3339))
	commands := [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "curl", "-y"},
	}
	withEnv := WithEnv(map[string]string{
		"http_proxy":  cluster.HTTPProxy,
		"https_proxy": cluster.HTTPProxy,
	})
	if err := RunCommandsOnNode(t, tc, 0, commands, withEnv); err != nil {
		t.Fatalf("fail to install test dependencies on node %s: %v", tc.Nodes[2], err)
	}

	// delete airgap bundles once they've been copied to the nodes
	if err := os.Remove(airgapInstallBundlePath); err != nil {
		t.Logf("failed to remove airgap install bundle: %v", err)
	}
	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh", "--proxy"}
	withEnv = WithEnv(map[string]string{
		"HTTP_PROXY":  cluster.HTTPProxy,
		"HTTPS_PROXY": cluster.HTTPProxy,
		"NO_PROXY":    "localhost,127.0.0.1,10.96.0.0/12,.svc,.local,.default,kubernetes,kotsadm-rqlite,kotsadm-api-node",
	})
	if _, _, err := RunCommandOnNode(t, tc, 0, line, withEnv); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	if err := setupPlaywright(t, tc); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := runPlaywrightTest(t, tc, "create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v", err)
	}
	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", fmt.Sprintf("%s-previous-k0s", os.Getenv("SHORT_SHA"))}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}
	t.Logf("%s: resetting the installation", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to reset the installation: %v", err)
	}
	t.Logf("%s: installing test dependencies on node 0", time.Now().Format(time.RFC3339))
	commands := [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "expect", "-y"},
	}
	withEnv = WithEnv(map[string]string{
		"http_proxy":  cluster.HTTPProxy,
		"https_proxy": cluster.HTTPProxy,
	})
	if err := RunCommandsOnNode(t, tc, 0, commands, withEnv); err != nil {
		t.Fatalf("fail to install test dependencies on node %s: %v", tc.Nodes[0], err)
	}
	t.Logf("%s: restoring the installation", time.Now().Format(time.RFC3339))
	line = append([]string{"restore-installation-airgap.exp"}, testArgs...)
	withEnv = WithEnv(map[string]string{
		"HTTP_PROXY":  cluster.HTTPProxy,
		"HTTPS_PROXY": cluster.HTTPProxy,
		"NO_PROXY":    "localhost,127.0.0.1,10.96.0.0/12,.svc,.local,.default,kubernetes,kotsadm-rqlite,kotsadm-api-node",
	})
	if _, _, err := RunCommandOnNode(t, tc, 0, line, withEnv); err != nil {
		t.Fatalf("fail to restore the installation: %v", err)
	}
	t.Logf("%s: checking installation state after restoring app", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", fmt.Sprintf("%s-previous-k0s", os.Getenv("SHORT_SHA"))}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}
	// remove the airgap bundle after upgrade
	line = []string{"rm", "/tmp/upgrade/release.airgap"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}

	if _, _, err := runPlaywrightTest(t, tc, "deploy-airgap-upgrade"); err != nil {
		t.Fatalf("fail to run playwright test deploy-airgap-upgrade: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
