package e2e

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
)

type installOptions struct {
	viaCLI                  bool
	version                 string
	adminConsolePort        string
	localArtifactMirrorPort string
	podCidr                 string
	serviceCidr             string
	httpProxy               string
	httpsProxy              string
	noProxy                 string
	privateCA               string
	configValuesFile        string
	withEnv                 map[string]string
}

type installationStateOptions struct {
	version    string
	k8sVersion string
	withEnv    map[string]string
}

type joinOptions struct {
	isAirgap   bool
	isHA       bool
	isRestore  bool
	keepAssets bool
	withEnv    map[string]string
}

func installSingleNode(t *testing.T, tc cluster.Cluster) {
	installSingleNodeWithOptions(t, tc, installOptions{})
}

func installSingleNodeWithOptions(t *testing.T, tc cluster.Cluster, opts installOptions) {
	line := []string{"single-node-install.sh"}

	if opts.viaCLI {
		line = append(line, "cli")
	} else {
		line = append(line, "ui")
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
	if opts.privateCA != "" {
		line = append(line, "--private-ca", opts.privateCA)
	}
	if opts.configValuesFile != "" {
		line = append(line, "--config-values", opts.configValuesFile)
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
	line := []string{"check-installation-state.sh"}
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
	stdout, stderr, err := tc.RunPlaywrightTest("get-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v: %s: %s", err, stdout, stderr)
	}
	t.Log("controller join token command:", command)

	if opts.isAirgap {
		t.Logf("%s: preparing embedded cluster airgap files on node %d", time.Now().Format(time.RFC3339), node)
		if _, _, err := tc.RunCommandOnNode(node, []string{"airgap-prepare.sh"}, opts.withEnv); err != nil {
			t.Fatalf("fail to prepare airgap files on node %d: %v", node, err)
		}
	}

	t.Logf("%s: joining node %d to the cluster as a controller%s", time.Now().Format(time.RFC3339), node,
		map[bool]string{true: " in ha mode", false: ""}[opts.isHA])

	var joinCommand []string
	if opts.isHA {
		if _, ok := tc.(*docker.Cluster); ok {
			joinCommand = []string{"join-ha.exp", fmt.Sprintf("'%s'", command)}
		} else {
			joinCommand = []string{"join-ha.exp", command}
		}
	} else if opts.isRestore {
		joinCommand = strings.Split(command, " ") // do not pass --no-ha as there should not be a prompt during a restore
	} else {
		command = strings.Replace(command, "join", "join --no-ha", 1) // bypass prompt
		joinCommand = strings.Split(command, " ")
	}

	if stdout, stderr, err := tc.RunCommandOnNode(node, joinCommand, opts.withEnv); err != nil {
		t.Fatalf("fail to join node %d as a controller%s: %v: %s: %s",
			node, map[bool]string{true: " in ha mode", false: ""}[opts.isHA], err, stdout, stderr)
	}

	if opts.isAirgap && !opts.keepAssets {
		// remove the airgap bundle and binary after joining
		line := []string{"rm", "/assets/release.airgap"}
		if _, _, err := tc.RunCommandOnNode(node, line, opts.withEnv); err != nil {
			t.Fatalf("fail to remove airgap bundle on node %d: %v", node, err)
		}
		line = []string{"rm", "/usr/local/bin/embedded-cluster"}
		if _, _, err := tc.RunCommandOnNode(node, line, opts.withEnv); err != nil {
			t.Fatalf("fail to remove embedded-cluster binary on node %d: %v", node, err)
		}
	}
}

func joinWorkerNode(t *testing.T, tc cluster.Cluster, node int) {
	joinWorkerNodeWithOptions(t, tc, node, joinOptions{})
}

func joinWorkerNodeWithOptions(t *testing.T, tc cluster.Cluster, node int, opts joinOptions) {
	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunPlaywrightTest("get-join-worker-command")
	if err != nil {
		t.Fatalf("fail to generate worker join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v: %s: %s", err, stdout, stderr)
	}
	t.Log("worker join token command:", command)

	if opts.isAirgap {
		t.Logf("%s: preparing embedded cluster airgap files on node %d", time.Now().Format(time.RFC3339), node)
		if _, _, err := tc.RunCommandOnNode(node, []string{"airgap-prepare.sh"}); err != nil {
			t.Fatalf("fail to prepare airgap files on node %d: %v", node, err)
		}
	}

	t.Logf("%s: joining node %d to the cluster as a worker", time.Now().Format(time.RFC3339), node)
	if stdout, stderr, err := tc.RunCommandOnNode(node, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node %d to the cluster as a worker: %v: %s: %s", node, err, stdout, stderr)
	}

	if opts.isAirgap && !opts.keepAssets {
		// remove the airgap bundle and binary after joining
		line := []string{"rm", "/assets/release.airgap"}
		if _, _, err := tc.RunCommandOnNode(node, line); err != nil {
			t.Fatalf("fail to remove airgap bundle on node %d: %v", node, err)
		}
		line = []string{"rm", "/usr/local/bin/embedded-cluster"}
		if _, _, err := tc.RunCommandOnNode(node, line); err != nil {
			t.Fatalf("fail to remove embedded-cluster binary on node %d: %v", node, err)
		}
	}
}

func waitForNodes(t *testing.T, tc cluster.Cluster, nodes int, envs map[string]string, args ...string) {
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunCommandOnNode(0, append([]string{"wait-for-ready-nodes.sh", fmt.Sprintf("%d", nodes)}, args...), envs)
	if err != nil {
		t.Fatalf("fail to wait for ready nodes: %v: %s: %s", err, stdout, stderr)
	}
}

func checkWorkerProfile(t *testing.T, tc cluster.Cluster, node int) {
	t.Logf("checking worker profile on node %d", node)
	line := []string{"check-worker-profile.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(node, line); err != nil {
		t.Fatalf("fail to check worker profile on node %d: %v: %s: %s", node, err, stdout, stderr)
	}
}
