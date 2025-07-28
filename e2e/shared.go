package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/lxd"
)

const (
	// License IDs used for e2e tests
	LicenseID                  = "2cQCFfBxG7gXDmq1yAgPSM4OViF"
	AirgapLicenseID            = "2eAqdricgviUeki42j02nIn1ayl"
	SnapshotLicenseID          = "2fSe1CXtMOX9jNgHTe00mvqO502"
	AirgapSnapshotLicenseID    = "2gEzHseTJQ4z2Axwj7KK9RYt4oT"
	MultiNodeDisabledLicenseID = "2vYEhmeVXsoDDoNB51uzBzCpang"
)

type installOptions struct {
	isAirgap                bool
	viaCLI                  bool
	version                 string
	adminConsolePort        string
	localArtifactMirrorPort string
	cidr                    string
	podCidr                 string
	serviceCidr             string
	httpProxy               string
	httpsProxy              string
	noProxy                 string
	configValuesFile        string
	networkInterface        string
	dataDir                 string
	withEnv                 map[string]string
}

type installationStateOptions struct {
	version    string
	k8sVersion string
	withEnv    map[string]string
}

type joinOptions struct {
	isHA      bool
	isRestore bool
	withEnv   map[string]string
}

type downloadECReleaseOptions struct {
	version   string
	licenseID string
	withEnv   map[string]string
}

type resetInstallationOptions struct {
	force   bool
	withEnv map[string]string
}

type postUpgradeStateOptions struct {
	node           int
	k8sVersion     string
	upgradeVersion string
	withEnv        map[string]string
}

func installSingleNode(t *testing.T, tc cluster.Cluster) {
	installSingleNodeWithOptions(t, tc, installOptions{})
}

func installSingleNodeWithOptions(t *testing.T, tc cluster.Cluster, opts installOptions) {
	line := []string{}

	if opts.isAirgap {
		line = append(line, "/usr/local/bin/single-node-airgap-install.sh")
	} else {
		line = append(line, "/usr/local/bin/single-node-install.sh")
		// the cli/ui option is currently only applicable for online installs
		if opts.viaCLI {
			line = append(line, "cli")
		} else {
			line = append(line, "ui")
		}
	}
	if opts.version != "" {
		line = append(line, opts.version)
	} else {
		line = append(line, os.Getenv("SHORT_SHA"))
	}
	if opts.adminConsolePort != "" {
		line = append(line, "--admin-console-port", opts.adminConsolePort)
	}
	if opts.localArtifactMirrorPort != "" {
		line = append(line, "--local-artifact-mirror-port", opts.localArtifactMirrorPort)
	}
	if opts.cidr != "" {
		line = append(line, "--cidr", opts.cidr)
	}
	if opts.podCidr != "" {
		line = append(line, "--pod-cidr", opts.podCidr)
	}
	if opts.serviceCidr != "" {
		line = append(line, "--service-cidr", opts.serviceCidr)
	}
	if opts.httpProxy != "" {
		line = append(line, "--http-proxy", opts.httpProxy)
	}
	if opts.httpsProxy != "" {
		line = append(line, "--https-proxy", opts.httpsProxy)
	}
	if opts.noProxy != "" {
		line = append(line, "--no-proxy", opts.noProxy)
	}
	if opts.configValuesFile != "" {
		line = append(line, "--config-values", opts.configValuesFile)
	}
	if opts.networkInterface != "" {
		line = append(line, "--network-interface", opts.networkInterface)
	}
	if opts.dataDir != "" {
		line = append(line, "--data-dir", opts.dataDir)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, opts.withEnv); err != nil {
		t.Fatalf("fail to install embedded-cluster on node 0: %v: %s: %s", err, stdout, stderr)
	}
}

func checkInstallationState(t *testing.T, tc cluster.Cluster) {
	checkInstallationStateWithOptions(t, tc, installationStateOptions{})
}

func checkInstallationStateWithOptions(t *testing.T, tc cluster.Cluster, opts installationStateOptions) {
	line := []string{"/usr/local/bin/check-installation-state.sh"}
	if opts.version != "" {
		line = append(line, opts.version)
	} else {
		line = append(line, os.Getenv("SHORT_SHA"))
	}
	if opts.k8sVersion != "" {
		line = append(line, opts.k8sVersion)
	} else {
		line = append(line, k8sVersion())
	}
	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, opts.withEnv); err != nil {
		t.Fatalf("fail to check installation state: %v: %s: %s", err, stdout, stderr)
	}
}

func joinControllerNode(t *testing.T, tc cluster.Cluster, node int) {
	joinControllerNodeWithOptions(t, tc, node, joinOptions{})
}

func joinControllerNodeWithOptions(t *testing.T, tc cluster.Cluster, node int, opts joinOptions) {
	t.Logf("%s: generating a new controller token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunPlaywrightTest("get-join-controller-commands")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	commands, err := findJoinCommandsInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v: %s: %s", err, stdout, stderr)
	}
	t.Log("controller join token commands:", commands)

	if len(commands) == 0 {
		t.Fatalf("no join commands found")
	}

	t.Logf("%s: joining node %d to the cluster as a controller%s", time.Now().Format(time.RFC3339), node,
		map[bool]string{true: " in ha mode", false: ""}[opts.isHA])

	lines := [][]string{}
	for i, command := range commands {
		if i < len(commands)-1 {
			lines = append(lines, strings.Fields(command))
			continue
		}
		// this is the join command
		var joinCommand []string
		if opts.isHA {
			if _, ok := tc.(*lxd.Cluster); ok {
				joinCommand = []string{"join-ha.exp", command}
			} else {
				joinCommand = []string{"join-ha.exp", fmt.Sprintf("'%s'", command)}
			}
		} else if opts.isRestore {
			joinCommand = strings.Fields(command) // do not pass --no-ha as there should not be a prompt during a restore
		} else {
			command = strings.Replace(command, "join", "join --no-ha", 1) // bypass prompt
			joinCommand = strings.Fields(command)
		}
		lines = append(lines, joinCommand)
	}

	for _, line := range lines {
		if stdout, stderr, err := tc.RunCommandOnNode(node, line, opts.withEnv); err != nil {
			t.Fatalf("fail to join node %d as a controller%s: %v: %s: %s",
				node, map[bool]string{true: " in ha mode", false: ""}[opts.isHA], err, stdout, stderr)
		}
	}
}

func joinWorkerNode(t *testing.T, tc cluster.Cluster, node int) {
	joinWorkerNodeWithOptions(t, tc, node, joinOptions{})
}

func joinWorkerNodeWithOptions(t *testing.T, tc cluster.Cluster, node int, opts joinOptions) {
	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunPlaywrightTest("get-join-worker-commands")
	if err != nil {
		t.Fatalf("fail to generate worker join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	commands, err := findJoinCommandsInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v: %s: %s", err, stdout, stderr)
	}
	t.Log("worker join commands:", commands)

	t.Logf("%s: joining node %d to the cluster as a worker", time.Now().Format(time.RFC3339), node)
	for _, command := range commands {
		if stdout, stderr, err := tc.RunCommandOnNode(node, strings.Fields(command), opts.withEnv); err != nil {
			t.Fatalf("fail to join node %d to the cluster as a worker: %v: %s: %s", node, err, stdout, stderr)
		}
	}
}

func waitForNodes(t *testing.T, tc cluster.Cluster, nodes int, envs map[string]string, args ...string) {
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunCommandOnNode(0, append([]string{"/usr/local/bin/wait-for-ready-nodes.sh", fmt.Sprintf("%d", nodes)}, args...), envs)
	if err != nil {
		t.Fatalf("fail to wait for ready nodes: %v: %s: %s", err, stdout, stderr)
	}
}

func checkWorkerProfile(t *testing.T, tc cluster.Cluster, node int) {
	t.Logf("checking worker profile on node %d", node)
	line := []string{"/usr/local/bin/check-worker-profile.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(node, line); err != nil {
		t.Fatalf("fail to check worker profile on node %d: %v: %s: %s", node, err, stdout, stderr)
	}
}

func checkNodeJoinCommand(t *testing.T, tc cluster.Cluster, node int) {
	t.Logf("node join command generation on node %d", node)
	line := []string{"/usr/local/bin/check-node-join-command.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(node, line); err != nil {
		t.Fatalf("fail to check if node join command is generated successfully on node %d: %v: %s: %s", node, err, stdout, stderr)
	}
}

func downloadECRelease(t *testing.T, tc cluster.Cluster, node int) {
	downloadECReleaseWithOptions(t, tc, node, downloadECReleaseOptions{})
}

func downloadECReleaseWithOptions(t *testing.T, tc cluster.Cluster, node int, opts downloadECReleaseOptions) {
	t.Logf("%s: downloading embedded cluster release on node %d", time.Now().Format(time.RFC3339), node)
	line := []string{"/usr/local/bin/vandoor-prepare.sh"}

	if opts.version != "" {
		line = append(line, opts.version)
	} else {
		line = append(line, fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")))
	}
	if opts.licenseID != "" {
		line = append(line, opts.licenseID)
	} else {
		line = append(line, LicenseID)
	}

	if stdout, stderr, err := tc.RunCommandOnNode(node, line, opts.withEnv); err != nil {
		t.Fatalf("fail to download embedded cluster release on node %d: %v: %s: %s", node, err, stdout, stderr)
	}
}

func resetInstallation(t *testing.T, tc cluster.Cluster, node int) {
	resetInstallationWithOptions(t, tc, node, resetInstallationOptions{})
}

func resetInstallationWithOptions(t *testing.T, tc cluster.Cluster, node int, opts resetInstallationOptions) {
	stdout, stderr, err := resetInstallationWithError(t, tc, node, opts)
	if err != nil {
		t.Fatalf("fail to reset the installation on node %d: %v: %s: %s", node, err, stdout, stderr)
	}
}

func resetInstallationWithError(t *testing.T, tc cluster.Cluster, node int, opts resetInstallationOptions) (string, string, error) {
	t.Logf("%s: resetting the installation on node %d", time.Now().Format(time.RFC3339), node)
	line := []string{"/usr/local/bin/reset-installation.sh"}
	if opts.force {
		line = append(line, "--force")
	}
	return tc.RunCommandOnNode(node, line, opts.withEnv)
}

func checkPostUpgradeState(t *testing.T, tc cluster.Cluster) {
	checkPostUpgradeStateWithOptions(t, tc, postUpgradeStateOptions{})
}

func checkPostUpgradeStateWithOptions(t *testing.T, tc cluster.Cluster, opts postUpgradeStateOptions) {
	line := []string{"/usr/local/bin/check-postupgrade-state.sh"}

	if opts.k8sVersion != "" {
		line = append(line, opts.k8sVersion)
	} else {
		line = append(line, k8sVersion())
	}

	if opts.upgradeVersion != "" {
		line = append(line, opts.upgradeVersion)
	} else {
		line = append(line, ecUpgradeTargetVersion())
	}

	t.Logf("%s: checking installation state after upgrade on node %d", time.Now().Format(time.RFC3339), opts.node)
	if stdout, stderr, err := tc.RunCommandOnNode(opts.node, line, opts.withEnv); err != nil {
		t.Fatalf("fail to check postupgrade state on node %d: %v: %s: %s", opts.node, err, stdout, stderr)
	}
}
