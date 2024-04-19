package e2e

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

func TestSingleNodeInstallation(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "ubuntu/jammy",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Destroy()
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install.sh", "ui"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if err := setupTestim(t, tc); err != nil {
		t.Fatalf("fail to setup testim: %v", err)
	}
	if _, _, err := runTestimTest(t, tc, "deploy-kots-application"); err != nil {
		t.Fatalf("fail to run testim test deploy-kots-application: %v", err)
	}

	t.Logf("%s: running kots upstream upgrade", time.Now().Format(time.RFC3339))
	line = []string{"kots-upstream-upgrade.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to run kots upstream upgrade: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstallationRockyLinux8(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "rockylinux/8",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Destroy()

	t.Logf("%s: installing tar", time.Now().Format(time.RFC3339))
	line := []string{"yum-install-tar.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-install.sh", "cli"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running kots upstream upgrade", time.Now().Format(time.RFC3339))
	line = []string{"kots-upstream-upgrade.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to run kots upstream upgrade: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstallationDebian12(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "debian/12",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Destroy()

	t.Logf("%s: installing test dependencies on node 0", time.Now().Format(time.RFC3339))
	commands := [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "ca-certificates", "curl", "-y"},
		{"update-ca-certificates"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail to install ssh on node 0: %v", err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install.sh", "cli"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running kots upstream upgrade", time.Now().Format(time.RFC3339))
	line = []string{"kots-upstream-upgrade.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to run kots upstream upgrade: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeInstallationCentos8Stream(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "centos/8-Stream",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Destroy()

	t.Logf("%s: installing tar", time.Now().Format(time.RFC3339))
	line := []string{"yum-install-tar.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-install.sh", "cli"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running kots upstream upgrade", time.Now().Format(time.RFC3339))
	line = []string{"kots-upstream-upgrade.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to run kots upstream upgrade: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestHostPreflight(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                                 t,
		Nodes:                             1,
		Image:                             "centos/8-Stream",
		LicensePath:                       "license.yaml",
		EmbeddedClusterPath:               "../output/bin/embedded-cluster",
		EmbeddedClusterReleaseBuilderPath: "../output/bin/embedded-cluster-release-builder",
	})
	defer tc.Destroy()

	t.Logf("%s: installing test dependencies on node 0", time.Now().Format(time.RFC3339))
	commands := [][]string{
		{"dnf", "install", "-y", "openssh-server", "binutils", "tar"},
		{"systemctl", "enable", "sshd"},
		{"systemctl", "start", "sshd"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail to install dependencies on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: running embedded-cluster preflights on node 0", time.Now().Format(time.RFC3339))
	line := []string{"embedded-preflight.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// This test creates 4 nodes, installs on the first one and then generate 2 join tokens
// for controllers and one join token for worker nodes. Joins the nodes and then waits
// for them to report ready.
func TestMultiNodeInstallation(t *testing.T) {
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               4,
		Image:               "ubuntu/jammy",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Destroy()

	// bootstrap the first node and makes sure it is healthy. also executes the kots
	// ssl certificate configuration (kurl-proxy).
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 0, []string{"single-node-install.sh", "ui"}); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if err := setupTestim(t, tc); err != nil {
		t.Fatalf("fail to setup testim: %v", err)
	}
	if _, _, err := runTestimTest(t, tc, "deploy-kots-application"); err != nil {
		t.Fatalf("fail to run testim test deploy-kots-application: %v", err)
	}

	// generate all node join commands (2 for controllers and 1 for worker).
	t.Logf("%s: generating two new controller token commands", time.Now().Format(time.RFC3339))
	controllerCommands := []string{}
	for i := 0; i < 2; i++ {
		stdout, stderr, err := runTestimTest(t, tc, "get-join-controller-command")
		if err != nil {
			t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
		}
		command, err := findJoinCommandInOutput(stdout)
		if err != nil {
			t.Fatalf("fail to find the join command in the output: %v", err)
		}
		controllerCommands = append(controllerCommands, command)
		t.Log("controller join token command:", command)
	}
	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := runTestimTest(t, tc, "get-join-worker-command")
	if err != nil {
		t.Fatalf("fail to generate worker join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("worker join token command:", command)

	// join the nodes.
	for i, cmd := range controllerCommands {
		node := i + 1
		t.Logf("%s: joining node %d to the cluster (controller)", time.Now().Format(time.RFC3339), node)
		if _, _, err := RunCommandOnNode(t, tc, node, strings.Split(cmd, " ")); err != nil {
			t.Fatalf("fail to join node %d as a controller: %v", node, err)
		}
		// XXX If we are too aggressive joining nodes we can see the following error being
		// thrown by kotsadm on its log (and we get a 500 back):
		// "
		// failed to get controller role name: failed to get cluster config: failed to get
		// current installation: failed to list installations: etcdserver: leader changed
		// "
		t.Logf("node %d joined, sleeping...", node)
		time.Sleep(30 * time.Second)
	}
	t.Logf("%s: joining node 3 to the cluster as a worker", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 3, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 3 to the cluster as a worker: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = RunCommandOnNode(t, tc, 0, []string{"wait-for-ready-nodes.sh", "4"})
	if err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	t.Log(stdout)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestInstallWithoutEmbed(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "rockylinux/8",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster-original",
	})
	defer tc.Destroy()
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"default-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestInstallFromReplicatedApp(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:     t,
		Nodes: 1,
		Image: "ubuntu/jammy",
	})
	defer tc.Destroy()
	t.Logf("%s: downloading embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"vandoor-prepare.sh", os.Getenv("SHORT_SHA"), os.Getenv("LICENSE_ID"), "false"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to download embedded-cluster on node 0 %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-install.sh", "cli"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running kots upstream upgrade", time.Now().Format(time.RFC3339))
	line = []string{"kots-upstream-upgrade.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to run kots upstream upgrade: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestResetAndReinstall(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "ubuntu/jammy",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer tc.Destroy()
	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install.sh", "cli"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: resetting the installation", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to reset the installation: %v", err)
	}

	t.Logf("%s: installing embedded-cluster on node 0 after reset", time.Now().Format(time.RFC3339))
	line = []string{"single-node-install.sh", "cli"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: checking installation state after reinstall", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestResetAndReinstallAirgap(t *testing.T) {
	t.Parallel()

	t.Logf("%s: downloading airgap file", time.Now().Format(time.RFC3339))
	// download airgap bundle
	airgapURL := fmt.Sprintf("https://staging.replicated.app/embedded/embedded-cluster-smoke-test-staging-app/ci-airgap/appver-%s?airgap=true", os.Getenv("SHORT_SHA"))

	req, err := http.NewRequest("GET", airgapURL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", os.Getenv("AIRGAP_LICENSE_ID"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to download airgap bundle: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to download airgap bundle: %s", resp.Status)
	}

	// pipe response to a temporary file
	airgapBundlePath := "/tmp/airgap-bundle.tar.gz"
	f, err := os.Create(airgapBundlePath)
	if err != nil {
		t.Fatalf("failed to create temporary file: %v", err)
	}
	defer f.Close()
	size, err := f.ReadFrom(resp.Body)
	if err != nil {
		t.Fatalf("failed to write response to temporary file: %v", err)
	}
	t.Logf("downloaded airgap bundle to %s (%d bytes)", airgapBundlePath, size)

	t.Logf("%s: creating airgap node", time.Now().Format(time.RFC3339))

	tc := cluster.NewTestCluster(&cluster.Input{
		T:                t,
		Nodes:            1,
		Image:            "ubuntu/jammy",
		WithProxy:        true,
		AirgapBundlePath: airgapBundlePath,
	})
	defer tc.Destroy()

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}

	if _, _, err = RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh"}
	if _, _, err = RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: resetting the installation", time.Now().Format(time.RFC3339))
	line = []string{"reset-installation.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to reset the installation: %v", err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh"}
	if _, _, err = RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestOldVersionUpgrade(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:     t,
		Nodes: 1,
		Image: "ubuntu/jammy",
	})
	defer tc.Destroy()
	t.Logf("%s: downloading embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"vandoor-prepare.sh", fmt.Sprintf("%s-pre-minio-removal", os.Getenv("SHORT_SHA")), os.Getenv("LICENSE_ID"), "false"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to download embedded-cluster on node 0 %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"pre-minio-removal-install.sh", "cli"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: checking installation state", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", fmt.Sprintf("%s-pre-minio-removal", os.Getenv("SHORT_SHA"))}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running kots upstream upgrade", time.Now().Format(time.RFC3339))
	line = []string{"kots-upstream-upgrade.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to run kots upstream upgrade: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestSingleNodeAirgapInstallationUbuntuJammy(t *testing.T) {
	t.Parallel()

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		downloadAirgapBundle(t, fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")), airgapInstallBundlePath)
		wg.Done()
	}()
	go func() {
		downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath)
		wg.Done()
	}()
	wg.Wait()

	tc := cluster.NewTestCluster(&cluster.Input{
		T:                       t,
		Nodes:                   1,
		Image:                   "ubuntu/jammy",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapInstallBundlePath,
		AirgapUpgradeBundlePath: airgapUpgradeBundlePath,
	})
	defer tc.Destroy()

	// delete airgap bundles once they've been copied to the nodes
	if err := os.Remove(airgapInstallBundlePath); err != nil {
		t.Logf("failed to remove airgap install bundle: %v", err)
	}
	if err := os.Remove(airgapUpgradeBundlePath); err != nil {
		t.Logf("failed to remove airgap upgrade bundle: %v", err)
	}

	t.Logf("%s: preparing embedded cluster airgap files", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}

	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}

	if err := setupTestim(t, tc); err != nil {
		t.Fatalf("fail to setup testim: %v", err)
	}
	if _, _, err := runTestimTest(t, tc, "deploy-kots-application"); err != nil {
		t.Fatalf("fail to run testim test deploy-kots-application: %v", err)
	}

	t.Logf("%s: installing kots cli on node 0", time.Now().Format(time.RFC3339))
	line = []string{"install-kots-cli.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install kots cli on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-installation-state.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running kots upstream upgrade", time.Now().Format(time.RFC3339))
	line = []string{"kots-upstream-upgrade.sh", ""}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to run kots upstream upgrade: %v", err)
	}

	if stdout, stderr, err := runTestimTest(t, tc, "deploy-airgap-upgrade"); err != nil {
		t.Fatalf("fail to run testim test deploy-airgap-upgrade: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh"}
	if stdout, stderr, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func downloadAirgapBundle(t *testing.T, versionLabel string, destPath string) string {
	// download airgap bundle
	airgapURL := fmt.Sprintf("https://staging.replicated.app/embedded/embedded-cluster-smoke-test-staging-app/ci-airgap/%s?airgap=true", versionLabel)

	req, err := http.NewRequest("GET", airgapURL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", os.Getenv("AIRGAP_LICENSE_ID"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to download airgap bundle: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to download airgap bundle: %s", resp.Status)
	}

	// pipe response to a temporary file
	airgapBundlePath := destPath
	f, err := os.Create(airgapBundlePath)
	if err != nil {
		t.Fatalf("failed to create temporary file: %v", err)
	}
	defer f.Close()
	size, err := f.ReadFrom(resp.Body)
	if err != nil {
		t.Fatalf("failed to write response to temporary file: %v", err)
	}
	t.Logf("downloaded airgap bundle to %s (%d bytes)", airgapBundlePath, size)

	return airgapBundlePath
}

func setupTestim(t *testing.T, tc *cluster.Output) error {
	t.Logf("%s: bypassing kurl-proxy on node 0", time.Now().Format(time.RFC3339))
	line := []string{"bypass-kurl-proxy.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		return fmt.Errorf("fail to bypass kurl-proxy on node %s: %v", tc.Nodes[0], err)
	}

	line = []string{"install-testim.sh"}
	if tc.Proxy != "" {
		t.Logf("%s: installing testim on proxy node", time.Now().Format(time.RFC3339))
		if _, _, err := RunCommandOnProxyNode(t, tc, line); err != nil {
			return fmt.Errorf("fail to install testim on node %s: %v", tc.Proxy, err)
		}
	} else {
		t.Logf("%s: installing testim on node 0", time.Now().Format(time.RFC3339))
		if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
			return fmt.Errorf("fail to install testim on node %s: %v", tc.Nodes[0], err)
		}
	}

	return nil
}

func runTestimTest(t *testing.T, tc *cluster.Output, testName string) (stdout, stderr string, err error) {
	line := []string{"testim.sh", os.Getenv("TESTIM_ACCESS_TOKEN"), os.Getenv("TESTIM_BRANCH"), testName}
	if tc.Proxy != "" {
		t.Logf("%s: running testim test %s on proxy node", time.Now().Format(time.RFC3339), testName)
		stdout, stderr, err = RunCommandOnProxyNode(t, tc, line)
		if err != nil {
			return stdout, stderr, fmt.Errorf("fail to run testim test %s on node %s: %v", testName, tc.Proxy, err)
		}
	} else {
		t.Logf("%s: running testim test %s on node 0", time.Now().Format(time.RFC3339), testName)
		stdout, stderr, err = RunCommandOnNode(t, tc, 0, line)
		if err != nil {
			return stdout, stderr, fmt.Errorf("fail to run testim test %s on node %s: %v", testName, tc.Nodes[0], err)
		}
	}

	return stdout, stderr, nil
}
