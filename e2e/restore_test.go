package e2e

import (
	"fmt"
	"os"
	"strings"
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
	if _, _, err := runPlaywrightTest(t, tc, "deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
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

func TestSingleNodeDisasterRecoveryWithProxy(t *testing.T) {
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
		WithProxy:           true,
		LicensePath:         "snapshot-license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer cleanupCluster(t, tc)

	t.Logf("%s: installing test dependencies on node 0", time.Now().Format(time.RFC3339))
	commands := [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "expect", "-y"},
	}
	withEnv := WithEnv(map[string]string{
		"http_proxy":  cluster.HTTPProxy,
		"https_proxy": cluster.HTTPProxy,
	})
	if err := RunCommandsOnNode(t, tc, 0, commands, withEnv); err != nil {
		t.Fatalf("fail to install test dependencies on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-proxy-install.sh"}
	line = append(line, "--http-proxy", cluster.HTTPProxy)
	line = append(line, "--https-proxy", cluster.HTTPProxy)
	line = append(line, "--no-proxy", cluster.NOProxy)
	withEnv = WithEnv(map[string]string{
		"HTTP_PROXY":  cluster.HTTPProxy,
		"HTTPS_PROXY": cluster.HTTPProxy,
		"NO_PROXY":    cluster.NOProxy,
	})
	if _, _, err := RunCommandOnNode(t, tc, 0, line, withEnv); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if err := setupPlaywright(t, tc); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := runPlaywrightTest(t, tc, "deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
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
	line = append(line, "--http-proxy", cluster.HTTPProxy)
	line = append(line, "--https-proxy", cluster.HTTPProxy)
	line = append(line, "--no-proxy", cluster.NOProxy)
	withEnv = WithEnv(map[string]string{
		"HTTP_PROXY":  cluster.HTTPProxy,
		"HTTPS_PROXY": cluster.HTTPProxy,
		"NO_PROXY":    cluster.NOProxy,
	})
	if _, _, err := RunCommandOnNode(t, tc, 0, line, withEnv); err != nil {
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
	if _, _, err := runPlaywrightTest(t, tc, "deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
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
	line = append(line, "--pod-cidr", "10.128.0.0/20")
	line = append(line, "--service-cidr", "10.129.0.0/20")
	withEnv = WithEnv(map[string]string{
		"HTTP_PROXY":  cluster.HTTPProxy,
		"HTTPS_PROXY": cluster.HTTPProxy,
		"NO_PROXY":    cluster.NOProxy,
	})
	if _, _, err := RunCommandOnNode(t, tc, 0, line, withEnv); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	if err := setupPlaywright(t, tc); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := runPlaywrightTest(t, tc, "deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}
	if _, _, err := runPlaywrightTest(t, tc, "create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v", err)
	}
	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", fmt.Sprintf("%s-previous-k0s", os.Getenv("SHORT_SHA"))}
	stdout, _, err := RunCommandOnNode(t, tc, 0, line)
	if err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}
	t.Log(stdout)
	// ensure that the cluster is using the right IP ranges.
	t.Logf("%s: checking service and pod IP addresses", time.Now().Format(time.RFC3339))
	stdout, _, err = RunCommandOnNode(t, tc, 0, []string{"check-cidr-ranges.sh", "^10.128.[0-9]*.[0-9]", "^10.129.[0-9]*.[0-9]"})
	if err != nil {
		t.Fatalf("fail to check addresses on node %s: %v", tc.Nodes[0], err)
	}
	t.Log(stdout)
	t.Logf("%s: resetting the installation", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to reset the installation: %v", err)
	}
	t.Logf("%s: installing test dependencies on node 0", time.Now().Format(time.RFC3339))
	commands = [][]string{
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
	testArgs = append(testArgs, "--pod-cidr", "10.128.0.0/20", "--service-cidr", "10.129.0.0/20")
	line = append([]string{"restore-installation-airgap.exp"}, testArgs...)
	withEnv = WithEnv(map[string]string{
		"HTTP_PROXY":  cluster.HTTPProxy,
		"HTTPS_PROXY": cluster.HTTPProxy,
		"NO_PROXY":    "localhost,127.0.0.1,.svc,.local,.default,kubernetes,kotsadm-rqlite,kotsadm-api-node",
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
	line = []string{"rm", "/assets/upgrade/release.airgap"}
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
		Nodes:               3,
		Image:               "debian/12",
		LicensePath:         "snapshot-license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer cleanupCluster(t, tc)

	// install "expect" dependency on node 0 as that's where the restore process will be initiated.
	// install "expect" dependency on node 2 as that's where the HA join command will run.
	t.Logf("%s: installing test dependencies on node 2", time.Now().Format(time.RFC3339))
	commands := [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "expect", "-y"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail to install test dependencies on node %s: %v", tc.Nodes[0], err)
	}
	if err := RunCommandsOnNode(t, tc, 2, commands); err != nil {
		t.Fatalf("fail to install test dependencies on node %s: %v", tc.Nodes[2], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 0, []string{"single-node-install.sh", "ui"}); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if err := setupPlaywright(t, tc); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := runPlaywrightTest(t, tc, "deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	// join a controller
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := runPlaywrightTest(t, tc, "get-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("controller join token command:", command)
	t.Logf("%s: joining node 1 to the cluster (controller)", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 1, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 1 as a controller: %v", err)
	}

	// join another controller in HA mode
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err = runPlaywrightTest(t, tc, "get-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err = findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("controller join token command:", command)
	t.Logf("%s: joining node 2 to the cluster (controller) in ha mode", time.Now().Format(time.RFC3339))
	line := append([]string{"join-ha.exp"}, []string{command}...)
	if _, _, err := RunCommandOnNode(t, tc, 2, line); err != nil {
		t.Fatalf("fail to join node 2 as a controller in ha mode: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = RunCommandOnNode(t, tc, 0, []string{"wait-for-ready-nodes.sh", "3"})
	if err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	t.Log(stdout)

	t.Logf("%s: checking installation state after enabling high availability", time.Now().Format(time.RFC3339))
	line = []string{"check-post-ha-state.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check post ha state: %v", err)
	}

	if _, _, err := runPlaywrightTest(t, tc, "create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v", err)
	}

	// reset the cluster
	line = []string{"reset-installation.sh", "--force", "--reboot"}
	t.Logf("%s: resetting the installation on node 2", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 2, line); err != nil {
		t.Fatalf("fail to reset the installation: %v", err)
	}
	t.Logf("%s: resetting the installation on node 1", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 1, line); err != nil {
		t.Fatalf("fail to reset the installation: %v", err)
	}
	t.Logf("%s: resetting the installation on node 0", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to reset the installation: %v", err)
	}

	// wait for reboot
	t.Logf("%s: waiting for nodes to reboot", time.Now().Format(time.RFC3339))
	time.Sleep(60 * time.Second)

	// begin restoring the cluster
	t.Logf("%s: restoring the installation: phase 1", time.Now().Format(time.RFC3339))
	line = append([]string{"restore-multi-node-phase1.exp"}, testArgs...)
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to restore phase 1 of the installation: %v", err)
	}

	// restore phase 1 completes when the prompt for adding nodes is reached.
	// add the expected nodes to the cluster, then continue to phase 2.

	// join a controller
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err = runPlaywrightTest(t, tc, "get-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err = findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("controller join token command:", command)
	t.Logf("%s: joining node 1 to the cluster (controller)", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 1, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 1 as a controller: %v", err)
	}

	// join another controller in non-HA mode
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err = runPlaywrightTest(t, tc, "get-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err = findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("controller join token command:", command)
	t.Logf("%s: joining node 2 to the cluster (controller)", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 2, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 2 as a controller: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = RunCommandOnNode(t, tc, 0, []string{"wait-for-ready-nodes.sh", "3", "true"})
	if err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	t.Log(stdout)

	t.Logf("%s: restoring the installation: phase 2", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 0, []string{"restore-multi-node-phase2.exp"}); err != nil {
		t.Fatalf("fail to restore phase 2 of the installation: %v", err)
	}

	t.Logf("%s: checking installation state after restoring the high availability backup", time.Now().Format(time.RFC3339))
	line = []string{"check-post-ha-state.sh", os.Getenv("SHORT_SHA"), "true"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check post ha state: %v", err)
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
	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			t.Fatalf("missing required environment variable: %s", envVar)
		}
	}

	testArgs := []string{}
	for _, envVar := range requiredEnvVars {
		testArgs = append(testArgs, os.Getenv(envVar))
	}

	t.Logf("%s: downloading airgap file", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	downloadAirgapBundle(t, fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")), airgapInstallBundlePath, os.Getenv("AIRGAP_SNAPSHOT_LICENSE_ID"))

	tc := cluster.NewTestCluster(&cluster.Input{
		T:                       t,
		Nodes:                   3,
		Image:                   "debian/12",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapInstallBundlePath,
	})
	defer cleanupCluster(t, tc)

	// install "expect" dependency on node 0 as that's where the restore process will be initiated.
	// install "expect" dependency on node 2 as that's where the HA join command will run.
	t.Logf("%s: installing test dependencies", time.Now().Format(time.RFC3339))
	commands := [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "expect", "curl", "-y"},
	}
	withEnv := WithEnv(map[string]string{
		"http_proxy":  cluster.HTTPProxy,
		"https_proxy": cluster.HTTPProxy,
	})
	if err := RunCommandsOnNode(t, tc, 0, commands, withEnv); err != nil {
		t.Fatalf("fail to install test dependencies on node %s: %v", tc.Nodes[0], err)
	}
	if err := RunCommandsOnNode(t, tc, 2, commands, withEnv); err != nil {
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
		"NO_PROXY":    cluster.NOProxy,
	})
	if _, _, err := RunCommandOnNode(t, tc, 0, line, withEnv); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if err := setupPlaywright(t, tc); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := runPlaywrightTest(t, tc, "deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	// join a controller
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := runPlaywrightTest(t, tc, "get-join-controller-command")
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
	if _, _, err := RunCommandOnNode(t, tc, 1, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node 1: %v", err)
	}
	t.Logf("%s: joining node 1 to the cluster (controller)", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 1, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 1 as a controller: %v", err)
	}

	// join another controller in HA mode
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err = runPlaywrightTest(t, tc, "get-join-controller-command")
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
	if _, _, err := RunCommandOnNode(t, tc, 2, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node 2: %v", err)
	}
	t.Logf("%s: joining node 2 to the cluster (controller) in ha mode", time.Now().Format(time.RFC3339))
	line = append([]string{"join-ha.exp"}, []string{command}...)
	if _, _, err := RunCommandOnNode(t, tc, 2, line); err != nil {
		t.Fatalf("fail to join node 2 as a controller in ha mode: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = RunCommandOnNode(t, tc, 0, []string{"wait-for-ready-nodes.sh", "3"})
	if err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	t.Log(stdout)

	t.Logf("%s: checking installation state after enabling high availability", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-post-ha-state.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check post ha state: %v", err)
	}

	if _, _, err := runPlaywrightTest(t, tc, "create-backup", testArgs...); err != nil {
		t.Fatalf("fail to run playwright test create-backup: %v", err)
	}

	// reset the cluster
	line = []string{"reset-installation.sh", "--force", "--reboot"}
	t.Logf("%s: resetting the installation on node 2", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 2, line); err != nil {
		t.Fatalf("fail to reset the installation: %v", err)
	}
	t.Logf("%s: resetting the installation on node 1", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 1, line); err != nil {
		t.Fatalf("fail to reset the installation: %v", err)
	}
	t.Logf("%s: resetting the installation on node 0", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to reset the installation: %v", err)
	}

	// wait for reboot
	t.Logf("%s: waiting for nodes to reboot", time.Now().Format(time.RFC3339))
	time.Sleep(60 * time.Second)

	// begin restoring the cluster
	t.Logf("%s: restoring the installation: phase 1", time.Now().Format(time.RFC3339))
	line = append([]string{"restore-multi-node-airgap-phase1.exp"}, testArgs...)
	withEnv = WithEnv(map[string]string{
		"HTTP_PROXY":  cluster.HTTPProxy,
		"HTTPS_PROXY": cluster.HTTPProxy,
		"NO_PROXY":    cluster.NOProxy,
	})
	if _, _, err := RunCommandOnNode(t, tc, 0, line, withEnv); err != nil {
		t.Fatalf("fail to restore phase 1 of the installation: %v", err)
	}

	// restore phase 1 completes when the prompt for adding nodes is reached.
	// add the expected nodes to the cluster, then continue to phase 2.

	// join a controller
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err = runPlaywrightTest(t, tc, "get-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err = findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("controller join token command:", command)
	t.Logf("%s: joining node 1 to the cluster (controller)", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 1, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 1 as a controller: %v", err)
	}

	// join another controller in non-HA mode
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err = runPlaywrightTest(t, tc, "get-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err = findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("controller join token command:", command)
	t.Logf("%s: joining node 2 to the cluster (controller)", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 2, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 2 as a controller: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = RunCommandOnNode(t, tc, 0, []string{"wait-for-ready-nodes.sh", "3", "true"})
	if err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	t.Log(stdout)

	t.Logf("%s: restoring the installation: phase 2", time.Now().Format(time.RFC3339))
	line = []string{"restore-multi-node-airgap-phase2.exp"}
	withEnv = WithEnv(map[string]string{
		"HTTP_PROXY":  cluster.HTTPProxy,
		"HTTPS_PROXY": cluster.HTTPProxy,
		"NO_PROXY":    cluster.NOProxy,
	})
	if _, _, err := RunCommandOnNode(t, tc, 0, line, withEnv); err != nil {
		t.Fatalf("fail to restore phase 2 of the installation: %v", err)
	}

	t.Logf("%s: checking installation state after restoring the high availability backup", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-post-ha-state.sh", os.Getenv("SHORT_SHA"), "true"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check post ha state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
