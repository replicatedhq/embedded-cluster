package e2e

import (
	"fmt"
	"os"
	"strconv"
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

	env := map[string]string{"DISABLE_FILESYSTEM_PERFORMANCE_CHECK": "1"}
	for k, v := range opts.withEnv {
		env[k] = v
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(0, line, env); err != nil {
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

	env := map[string]string{"DISABLE_FILESYSTEM_PERFORMANCE_CHECK": "1"}
	for k, v := range opts.withEnv {
		env[k] = v
	}

	for _, line := range lines {
		if stdout, stderr, err := tc.RunCommandOnNode(node, line, env); err != nil {
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

	env := map[string]string{"DISABLE_FILESYSTEM_PERFORMANCE_CHECK": "1"}
	for k, v := range opts.withEnv {
		env[k] = v
	}

	t.Logf("%s: joining node %d to the cluster as a worker", time.Now().Format(time.RFC3339), node)
	for _, command := range commands {
		if stdout, stderr, err := tc.RunCommandOnNode(node, strings.Fields(command), env); err != nil {
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

// checkContainerdRegistryConfigAbsent asserts the containerd registry drop-in is
// absent: online installs don't use the in-cluster registry.
// TODO(k0s-1.37-oldest): drop this check along with the migration.
// checkSELinuxLabels asserts that embedded cluster labeled its own files during
// install. The test deliberately pre-labels nothing, so these labels can only
// come from hostutils.ConfigureSELinuxFcontext and RestoreSELinuxContext.
//
// The bin_t rules cover our own binaries; the container_* rules are the ones
// k0s documents, without which containerd is not treated as a container
// runtime and container images are not container content.
func checkSELinuxLabels(t *testing.T, tc cluster.Cluster, node int) {
	t.Logf("%s: verifying selinux labels on node %d", time.Now().Format(time.RFC3339), node)

	for _, tt := range []struct {
		path  string
		field string
		want  string
	}{
		{"/var/lib/embedded-cluster", "--user", "system_u"},
		{"/var/lib/embedded-cluster/bin", "--user", "system_u"},
		{"/var/lib/embedded-cluster/bin", "--type", "bin_t"},
		{"/var/lib/embedded-cluster/k0s/bin/containerd", "--type", "container_runtime_exec_t"},
		{"/var/lib/embedded-cluster/k0s/bin/runc", "--type", "container_runtime_exec_t"},
		{"/var/lib/embedded-cluster/k0s/containerd", "--type", "container_var_lib_t"},
	} {
		line := []string{"secon", tt.field, "--file", tt.path}
		stdout, stderr, err := tc.RunCommandOnNode(node, line)
		if err != nil {
			t.Fatalf("fail to read selinux %s label of %s on node %d: %v: %s: %s", tt.field, tt.path, node, err, stdout, stderr)
		}
		if got := strings.TrimSpace(stdout); got != tt.want {
			t.Fatalf("selinux %s label of %s on node %d is %q, want %q", tt.field, tt.path, node, got, tt.want)
		}
	}
}

// checkSELinuxConfinement asserts that our workloads actually run confined,
// which is the part the old test never checked. containerd only labels
// containers when enable_selinux is set, and neither containerd nor k0s
// defaults it on, so without our drop-in every container inherits k0s's domain
// instead of container_t and container-selinux confines nothing.
func checkSELinuxConfinement(t *testing.T, tc cluster.Cluster, node int) {
	t.Logf("%s: verifying selinux confinement on node %d", time.Now().Format(time.RFC3339), node)

	t.Logf("%s: verifying container-selinux is installed", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(node, []string{"rpm -q container-selinux"}); err != nil {
		t.Fatalf("container-selinux is not installed on node %d: %v: %s: %s", node, err, stdout, stderr)
	}

	t.Logf("%s: verifying containerd selinux drop-in", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := tc.RunCommandOnNode(node, []string{"grep -q 'enable_selinux = true' /etc/k0s/containerd.d/embedded-selinux.toml"}); err != nil {
		t.Fatalf("containerd selinux drop-in is missing or does not enable selinux on node %d: %v: %s: %s", node, err, stdout, stderr)
	}

	t.Logf("%s: verifying containerd runs as container_runtime_t", time.Now().Format(time.RFC3339))
	stdout, stderr, err := tc.RunCommandOnNode(node, []string{"ps -eZ | grep -F '/containerd' | head -1"})
	if err != nil {
		t.Fatalf("fail to read containerd process domain on node %d: %v: %s: %s", node, err, stdout, stderr)
	}
	if !strings.Contains(stdout, "container_runtime_t") {
		t.Fatalf("containerd on node %d is not running as container_runtime_t: %s", node, stdout)
	}

	// Every pod sandbox should be container_t. If enable_selinux is off,
	// containerd never sets a label and these inherit k0s's domain instead.
	t.Logf("%s: verifying pods run as container_t", time.Now().Format(time.RFC3339))
	stdout, stderr, err = tc.RunCommandOnNode(node, []string{"ps -eZ | grep -c container_t"})
	if err != nil {
		t.Fatalf("fail to count container_t processes on node %d: %v: %s: %s", node, err, stdout, stderr)
	}
	count, convErr := strconv.Atoi(strings.TrimSpace(stdout))
	if convErr != nil {
		t.Fatalf("fail to parse container_t process count %q on node %d: %v", stdout, node, convErr)
	}
	if count == 0 {
		t.Fatalf("no processes are running as container_t on node %d, so nothing is confined", node)
	}
	t.Logf("%s: %d processes running as container_t on node %d", time.Now().Format(time.RFC3339), count, node)
}

func checkContainerdRegistryConfigAbsent(t *testing.T, tc cluster.Cluster, node int) {
	t.Logf("%s: verifying containerd registry drop-in is absent on node %d", time.Now().Format(time.RFC3339), node)
	line := []string{"test", "!", "-f", "/etc/k0s/containerd.d/embedded-registry.toml"}
	if stdout, stderr, err := tc.RunCommandOnNode(node, line); err != nil {
		t.Fatalf("containerd registry drop-in should be absent on online installs on node %d: %v: %s: %s", node, err, stdout, stderr)
	}
}

// checkContainerdRegistryConfigV2 asserts the registry drop-in still uses the
// containerd 1.7 schema required by k0s 1.34 and 1.35.
func checkContainerdRegistryConfigV2(t *testing.T, tc cluster.Cluster, node int) {
	t.Logf("%s: verifying containerd registry drop-in uses the v2 schema on node %d", time.Now().Format(time.RFC3339), node)
	line := []string{"grep -q 'io.containerd.grpc.v1.cri' /etc/k0s/containerd.d/embedded-registry.toml && ! grep -q 'io.containerd.cri.v1.images' /etc/k0s/containerd.d/embedded-registry.toml"}
	if stdout, stderr, err := tc.RunCommandOnNode(node, line); err != nil {
		t.Fatalf("containerd registry drop-in does not use the v2 schema on node %d: %v: %s: %s", node, err, stdout, stderr)
	}
}

// checkContainerdRegistryConfigV3 asserts the registry drop-in was migrated to
// the containerd 2.x schema required by k0s 1.36 and later.
func checkContainerdRegistryConfigV3(t *testing.T, tc cluster.Cluster, node int) {
	t.Logf("%s: verifying containerd registry drop-in uses the v3 schema on node %d", time.Now().Format(time.RFC3339), node)
	line := []string{"grep -q 'io.containerd.cri.v1.images' /etc/k0s/containerd.d/embedded-registry.toml && ! grep -q 'io.containerd.grpc.v1.cri' /etc/k0s/containerd.d/embedded-registry.toml"}
	if stdout, stderr, err := tc.RunCommandOnNode(node, line); err != nil {
		t.Fatalf("containerd registry drop-in does not use the v3 schema on node %d: %v: %s: %s", node, err, stdout, stderr)
	}
}
