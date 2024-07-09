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
	defer cleanupCluster(t, tc)

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

func TestSingleNodeInstallationAlmaLinux8(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "almalinux/8",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer cleanupCluster(t, tc)

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
	defer cleanupCluster(t, tc)

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

func TestSingleNodeInstallationDebian11(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "debian/11",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer cleanupCluster(t, tc)

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

func TestSingleNodeInstallationCentos9Stream(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               1,
		Image:               "centos/9-Stream",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer cleanupCluster(t, tc)

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
		Image:                             "centos/9-Stream",
		LicensePath:                       "license.yaml",
		EmbeddedClusterPath:               "../output/bin/embedded-cluster",
		EmbeddedClusterReleaseBuilderPath: "../output/bin/embedded-cluster-release-builder",
	})
	defer tc.Destroy()

	t.Logf("%s: installing test dependencies on node 0", time.Now().Format(time.RFC3339))
	commands := [][]string{
		{"dnf", "install", "-y", "openssh-server", "binutils", "tar", "fio"},
		{"systemctl", "enable", "sshd"},
		{"systemctl", "start", "sshd"},
	}
	if err := RunCommandsOnNode(t, tc, 0, commands); err != nil {
		t.Fatalf("fail to install dependencies on node %s: %v", tc.Nodes[0], err)
	}

	// listPaths := [][]string{
	// 	{"/usr/local/bin/k0s", "sysinfo"},
	// 	{"ls", "-al", "/proc/config.gz"},
	// 	{"ls", "-al", "/boot/config-*"},
	// 	{"ls", "-al", "/usr/src/linux-*/.config"},
	// 	{"ls", "-al", "/usr/src/linux/.config"},
	// 	{"ls", "-al", "/usr/lib/modules/*/config"},
	// 	{"ls", "-al", "/usr/lib/ostree-boot/config-*"},
	// 	{"ls", "-al", "/usr/lib/kernel/config-*"},
	// 	{"ls", "-al", "/usr/src/linux-headers-*/.config"},
	// 	{"ls", "-al", "/lib/modules/*/build/.config"},
	// }
	// for _, cmd := range listPaths {
	// 	stdout, stderr, err := RunCommandOnNode(t, tc, 0, cmd)
	// 	if err != nil {
	// 		t.Errorf("failed to list paths: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	// 	} else {
	// 		t.Logf("list paths stdout: %s", stdout)
	// 		t.Logf("list paths stderr: %s", stderr)
	// 	}
	// }

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
		Image:               "debian/12",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer cleanupCluster(t, tc)

	// bootstrap the first node and makes sure it is healthy. also executes the kots
	// ssl certificate configuration (kurl-proxy).
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

	// generate all node join commands (2 for controllers and 1 for worker).
	t.Logf("%s: generating two new controller token commands", time.Now().Format(time.RFC3339))
	controllerCommands := []string{}
	for i := 0; i < 2; i++ {
		stdout, stderr, err := runPlaywrightTest(t, tc, "get-join-controller-command")
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
	stdout, stderr, err := runPlaywrightTest(t, tc, "get-join-worker-command")
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
		Image:               "almalinux/8",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster-original",
	})
	defer cleanupCluster(t, tc)

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
		Image: "debian/12",
	})
	defer cleanupCluster(t, tc)

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
		Image:               "debian/12",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer cleanupCluster(t, tc)

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
		T:                       t,
		Nodes:                   1,
		Image:                   "debian/12",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapBundlePath,
	})
	defer cleanupCluster(t, tc)

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
		Image: "debian/12",
	})
	defer cleanupCluster(t, tc)

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
	line = []string{"check-pre-minio-removal-installation-state.sh", fmt.Sprintf("%s-pre-minio-removal", os.Getenv("SHORT_SHA"))}
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

func TestSingleNodeAirgapUpgrade(t *testing.T) {
	t.Parallel()

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		downloadAirgapBundle(t, fmt.Sprintf("appver-%s-previous-k0s", os.Getenv("SHORT_SHA")), airgapInstallBundlePath, os.Getenv("AIRGAP_LICENSE_ID"))
		wg.Done()
	}()
	go func() {
		downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath, os.Getenv("AIRGAP_LICENSE_ID"))
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

	// delete airgap bundles once they've been copied to the nodes
	if err := os.Remove(airgapInstallBundlePath); err != nil {
		t.Logf("failed to remove airgap install bundle: %v", err)
	}
	if err := os.Remove(airgapUpgradeBundlePath); err != nil {
		t.Logf("failed to remove airgap upgrade bundle: %v", err)
	}

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
	// remove the airgap bundle after installation
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}

	if err := setupPlaywright(t, tc); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := runPlaywrightTest(t, tc, "deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
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

func TestMultiNodeAirgapUpgradeSameK0s(t *testing.T) {
	t.Parallel()

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		downloadAirgapBundle(t, fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")), airgapInstallBundlePath, os.Getenv("AIRGAP_LICENSE_ID"))
		wg.Done()
	}()
	go func() {
		downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath, os.Getenv("AIRGAP_LICENSE_ID"))
		wg.Done()
	}()
	wg.Wait()

	tc := cluster.NewTestCluster(&cluster.Input{
		T:                       t,
		Nodes:                   2,
		Image:                   "debian/12",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapInstallBundlePath,
		AirgapUpgradeBundlePath: airgapUpgradeBundlePath,
	})
	defer cleanupCluster(t, tc)

	// delete airgap bundles once they've been copied to the nodes
	if err := os.Remove(airgapInstallBundlePath); err != nil {
		t.Logf("failed to remove airgap install bundle: %v", err)
	}
	if err := os.Remove(airgapUpgradeBundlePath); err != nil {
		t.Logf("failed to remove airgap upgrade bundle: %v", err)
	}

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

	// upgrade airgap bundle is only needed on the first node
	line := []string{"rm", "/assets/ec-release-upgrade.tgz"}
	if _, _, err := RunCommandOnNode(t, tc, 1, line); err != nil {
		t.Fatalf("fail to remove upgrade airgap bundle on node %s: %v", tc.Nodes[1], err)
	}

	t.Logf("%s: preparing embedded cluster airgap files on node 0", time.Now().Format(time.RFC3339))
	line = []string{"airgap-prepare.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	// remove artifacts after installation to save space
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on node %s: %v", tc.Nodes[0], err)
	}
	line = []string{"rm", "/var/lib/embedded-cluster/bin/embedded-cluster"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on node %s: %v", tc.Nodes[0], err)
	}

	if err := setupPlaywright(t, tc); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := runPlaywrightTest(t, tc, "deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	// generate worker node join command.
	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := runPlaywrightTest(t, tc, "get-join-worker-command")
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
	if _, _, err := RunCommandOnNode(t, tc, 1, line); err != nil {
		t.Fatalf("fail to prepare airgap files on worker node: %v", err)
	}
	t.Logf("%s: joining worker node to the cluster", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 1, strings.Split(workerCommand, " ")); err != nil {
		t.Fatalf("fail to join worker node to the cluster: %v", err)
	}
	// remove artifacts after joining to save space
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := RunCommandOnNode(t, tc, 1, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on worker node: %v", err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster"}
	if _, _, err := RunCommandOnNode(t, tc, 1, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on worker node: %v", err)
	}
	line = []string{"rm", "/var/lib/embedded-cluster/bin/embedded-cluster"}
	if _, _, err := RunCommandOnNode(t, tc, 1, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on node %s: %v", tc.Nodes[0], err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = RunCommandOnNode(t, tc, 0, []string{"wait-for-ready-nodes.sh", "2"})
	if err != nil {
		t.Fatalf("fail to wait for ready nodes: %v", err)
	}
	t.Log(stdout)

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}
	// remove the airgap bundle and binary after upgrade
	line = []string{"rm", "/assets/upgrade/release.airgap"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster-upgrade"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster-upgrade binary on node %s: %v", tc.Nodes[0], err)
	}

	if _, _, err := runPlaywrightTest(t, tc, "deploy-airgap-upgrade", "true"); err != nil {
		t.Fatalf("fail to run playwright test deploy-airgap-upgrade: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestMultiNodeAirgapUpgrade(t *testing.T) {
	t.Parallel()

	t.Logf("%s: downloading airgap files", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	airgapUpgradeBundlePath := "/tmp/airgap-upgrade-bundle.tar.gz"
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		downloadAirgapBundle(t, fmt.Sprintf("appver-%s-previous-k0s", os.Getenv("SHORT_SHA")), airgapInstallBundlePath, os.Getenv("AIRGAP_LICENSE_ID"))
		wg.Done()
	}()
	go func() {
		downloadAirgapBundle(t, fmt.Sprintf("appver-%s-upgrade", os.Getenv("SHORT_SHA")), airgapUpgradeBundlePath, os.Getenv("AIRGAP_LICENSE_ID"))
		wg.Done()
	}()
	wg.Wait()

	tc := cluster.NewTestCluster(&cluster.Input{
		T:                       t,
		Nodes:                   2,
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
	if err := os.Remove(airgapUpgradeBundlePath); err != nil {
		t.Logf("failed to remove airgap upgrade bundle: %v", err)
	}

	// upgrade airgap bundle is only needed on the first node
	line := []string{"rm", "/assets/ec-release-upgrade.tgz"}
	if _, _, err := RunCommandOnNode(t, tc, 1, line); err != nil {
		t.Fatalf("fail to remove upgrade airgap bundle on node %s: %v", tc.Nodes[1], err)
	}

	t.Logf("%s: preparing embedded cluster airgap files on node 0", time.Now().Format(time.RFC3339))
	line = []string{"airgap-prepare.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	// remove the airgap bundle and binary after installation
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on node %s: %v", tc.Nodes[0], err)
	}

	if err := setupPlaywright(t, tc); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := runPlaywrightTest(t, tc, "deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	// generate worker node join command.
	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := runPlaywrightTest(t, tc, "get-join-worker-command")
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
	if _, _, err := RunCommandOnNode(t, tc, 1, line); err != nil {
		t.Fatalf("fail to prepare airgap files on worker node: %v", err)
	}
	t.Logf("%s: joining worker node to the cluster", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 1, strings.Split(workerCommand, " ")); err != nil {
		t.Fatalf("fail to join worker node to the cluster: %v", err)
	}
	// remove the airgap bundle and binary after joining
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := RunCommandOnNode(t, tc, 1, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on worker node: %v", err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster"}
	if _, _, err := RunCommandOnNode(t, tc, 1, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on worker node: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = RunCommandOnNode(t, tc, 0, []string{"wait-for-ready-nodes.sh", "2"})
	if err != nil {
		t.Fatalf("fail to wait for ready nodes: %v", err)
	}
	t.Log(stdout)

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", fmt.Sprintf("%s-previous-k0s", os.Getenv("SHORT_SHA"))}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	t.Logf("%s: running airgap update", time.Now().Format(time.RFC3339))
	line = []string{"airgap-update.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to run airgap update: %v", err)
	}
	// remove the airgap bundle and binary after upgrade
	line = []string{"rm", "/assets/upgrade/release.airgap"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster-upgrade"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster-upgrade binary on node %s: %v", tc.Nodes[0], err)
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

// This test creates 4 nodes, installs on the first one and then generate 2 join tokens
// for controllers and one join token for worker nodes. Joins the nodes as HA and then waits
// for them to report ready. Runs additional high availability validations afterwards.
func TestMultiNodeHAInstallation(t *testing.T) {
	tc := cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               4,
		Image:               "debian/12",
		LicensePath:         "license.yaml",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	defer cleanupCluster(t, tc)

	// install "expect" dependency on node 3 as that's where the HA join command will run.
	t.Logf("%s: installing test dependencies on node 3", time.Now().Format(time.RFC3339))
	commands := [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "expect", "-y"},
	}
	if err := RunCommandsOnNode(t, tc, 3, commands); err != nil {
		t.Fatalf("fail to install test dependencies on node %s: %v", tc.Nodes[3], err)
	}

	// bootstrap the first node and makes sure it is healthy. also executes the kots
	// ssl certificate configuration (kurl-proxy).
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

	// join a worker
	t.Logf("%s: generating a new worker token command", time.Now().Format(time.RFC3339))
	stdout, stderr, err := runPlaywrightTest(t, tc, "get-join-worker-command")
	if err != nil {
		t.Fatalf("fail to generate worker join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err := findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("worker join token command:", command)
	t.Logf("%s: joining node 1 to the cluster as a worker", time.Now().Format(time.RFC3339))
	if _, _, err := RunCommandOnNode(t, tc, 1, strings.Split(command, " ")); err != nil {
		t.Fatalf("fail to join node 1 to the cluster as a worker: %v", err)
	}

	// join a controller
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

	// join another controller in HA mode
	stdout, stderr, err = runPlaywrightTest(t, tc, "get-join-controller-command")
	if err != nil {
		t.Fatalf("fail to generate controller join token:\nstdout: %s\nstderr: %s", stdout, stderr)
	}
	command, err = findJoinCommandInOutput(stdout)
	if err != nil {
		t.Fatalf("fail to find the join command in the output: %v", err)
	}
	t.Log("controller join token command:", command)
	t.Logf("%s: joining node 3 to the cluster (controller) in ha mode", time.Now().Format(time.RFC3339))
	line := append([]string{"join-ha.exp"}, []string{command}...)
	if _, _, err := RunCommandOnNode(t, tc, 3, line); err != nil {
		t.Fatalf("fail to join node 3 as a controller in ha mode: %v", err)
	}

	// wait for the nodes to report as ready.
	t.Logf("%s: all nodes joined, waiting for them to be ready", time.Now().Format(time.RFC3339))
	stdout, _, err = RunCommandOnNode(t, tc, 0, []string{"wait-for-ready-nodes.sh", "4"})
	if err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	t.Log(stdout)

	t.Logf("%s: checking installation state after enabling high availability", time.Now().Format(time.RFC3339))
	line = []string{"check-post-ha-state.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check post ha state: %v", err)
	}

	bin := strings.Split(command, " ")[0]
	t.Logf("%s: resetting controller node", time.Now().Format(time.RFC3339))
	stdout, stderr, err = RunCommandOnNode(t, tc, 2, []string{bin, "reset", "--no-prompt"})
	if err != nil {
		t.Fatalf("fail to remove controller node %s:", err)
	}
	if !strings.Contains(stderr, "High-availability clusters must maintain at least three controller nodes") {
		t.Errorf("reset output does not contain the ha warning")
		t.Logf("stdout: %s\nstderr: %s", stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// This test creates 4 airgap nodes, installs on the first one and then generate 2 join tokens
// for controllers and one join token for worker nodes. Joins the nodes as airgap HA and then waits
// for them to report ready. Runs additional high availability validations afterwards.
func TestMultiNodeAirgapHAInstallation(t *testing.T) {
	t.Parallel()

	t.Logf("%s: downloading airgap file", time.Now().Format(time.RFC3339))
	airgapInstallBundlePath := "/tmp/airgap-install-bundle.tar.gz"
	downloadAirgapBundle(t, fmt.Sprintf("appver-%s", os.Getenv("SHORT_SHA")), airgapInstallBundlePath, os.Getenv("AIRGAP_LICENSE_ID"))

	tc := cluster.NewTestCluster(&cluster.Input{
		T:                       t,
		Nodes:                   3,
		Image:                   "debian/12",
		WithProxy:               true,
		AirgapInstallBundlePath: airgapInstallBundlePath,
	})
	defer cleanupCluster(t, tc)

	// delete airgap bundles once they've been copied to the nodes
	if err := os.Remove(airgapInstallBundlePath); err != nil {
		t.Logf("failed to remove airgap install bundle: %v", err)
	}

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

	// install "expect" dependency on node 2 as that's where the HA join command will run.
	t.Logf("%s: installing test dependencies on node 2", time.Now().Format(time.RFC3339))
	commands = [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "expect", "-y"},
	}
	withEnv = WithEnv(map[string]string{
		"http_proxy":  cluster.HTTPProxy,
		"https_proxy": cluster.HTTPProxy,
	})
	if err := RunCommandsOnNode(t, tc, 2, commands, withEnv); err != nil {
		t.Fatalf("fail to install test dependencies on node %s: %v", tc.Nodes[2], err)
	}

	t.Logf("%s: preparing embedded cluster airgap files on node 0", time.Now().Format(time.RFC3339))
	line := []string{"airgap-prepare.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to prepare airgap files on node %s: %v", tc.Nodes[0], err)
	}

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line = []string{"single-node-airgap-install.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster on node %s: %v", tc.Nodes[0], err)
	}
	// remove artifacts after installation to save space
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node %s: %v", tc.Nodes[0], err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on node %s: %v", tc.Nodes[0], err)
	}

	if err := setupPlaywright(t, tc); err != nil {
		t.Fatalf("fail to setup playwright: %v", err)
	}
	if _, _, err := runPlaywrightTest(t, tc, "deploy-app"); err != nil {
		t.Fatalf("fail to run playwright test deploy-app: %v", err)
	}

	t.Logf("%s: checking installation state after app deployment", time.Now().Format(time.RFC3339))
	line = []string{"check-airgap-installation-state.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check installation state: %v", err)
	}

	// join a controller
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
	// remove the airgap bundle and binary after joining
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := RunCommandOnNode(t, tc, 1, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node 1: %v", err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster"}
	if _, _, err := RunCommandOnNode(t, tc, 1, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on node 1: %v", err)
	}

	// join another controller in HA mode
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
	// remove the airgap bundle and binary after joining
	line = []string{"rm", "/assets/release.airgap"}
	if _, _, err := RunCommandOnNode(t, tc, 2, line); err != nil {
		t.Fatalf("fail to remove airgap bundle on node 2: %v", err)
	}
	line = []string{"rm", "/usr/local/bin/embedded-cluster"}
	if _, _, err := RunCommandOnNode(t, tc, 2, line); err != nil {
		t.Fatalf("fail to remove embedded-cluster binary on node 2: %v", err)
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

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestInstallSnapshotFromReplicatedApp(t *testing.T) {
	t.Parallel()
	tc := cluster.NewTestCluster(&cluster.Input{
		T:     t,
		Nodes: 1,
		Image: "debian/12",
	})
	defer cleanupCluster(t, tc)

	t.Logf("%s: downloading embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"vandoor-prepare.sh", os.Getenv("SHORT_SHA"), os.Getenv("SNAPSHOT_LICENSE_ID"), "false"}
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

	t.Logf("%s: ensuring velero is installed", time.Now().Format(time.RFC3339))
	line = []string{"check-velero-state.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check velero state: %v", err)
	}

	t.Logf("%s: running kots upstream upgrade", time.Now().Format(time.RFC3339))
	line = []string{"kots-upstream-upgrade.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to run kots upstream upgrade: %v", err)
	}

	t.Logf("%s: checking installation state after upgrade", time.Now().Format(time.RFC3339))
	line = []string{"check-postupgrade-state.sh", os.Getenv("SHORT_SHA")}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Fatalf("fail to check postupgrade state: %v", err)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func downloadAirgapBundle(t *testing.T, versionLabel string, destPath string, licenseID string) string {
	// download airgap bundle
	airgapURL := fmt.Sprintf("https://staging.replicated.app/embedded/embedded-cluster-smoke-test-staging-app/ci-airgap/%s?airgap=true", versionLabel)

	req, err := http.NewRequest("GET", airgapURL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", licenseID)

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

func setupPlaywright(t *testing.T, tc *cluster.Output) error {
	t.Logf("%s: bypassing kurl-proxy on node 0", time.Now().Format(time.RFC3339))
	line := []string{"bypass-kurl-proxy.sh"}
	if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		return fmt.Errorf("fail to bypass kurl-proxy on node %s: %v", tc.Nodes[0], err)
	}

	line = []string{"install-playwright.sh"}
	if tc.Proxy != "" {
		t.Logf("%s: installing playwright on proxy node", time.Now().Format(time.RFC3339))
		if _, _, err := RunCommandOnProxyNode(t, tc, line); err != nil {
			return fmt.Errorf("fail to install playwright on node %s: %v", tc.Proxy, err)
		}
	} else {
		t.Logf("%s: installing playwright on node 0", time.Now().Format(time.RFC3339))
		if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
			return fmt.Errorf("fail to install playwright on node %s: %v", tc.Nodes[0], err)
		}
	}
	return nil
}

func runPlaywrightTest(t *testing.T, tc *cluster.Output, testName string, args ...string) (stdout, stderr string, err error) {
	line := []string{"playwright.sh", testName}
	line = append(line, args...)
	if tc.Proxy != "" {
		t.Logf("%s: running playwright test %s on proxy node", time.Now().Format(time.RFC3339), testName)
		stdout, stderr, err = RunCommandOnProxyNode(t, tc, line)
		if err != nil {
			return stdout, stderr, fmt.Errorf("fail to run playwright test %s on node %s: %v", testName, tc.Proxy, err)
		}
	} else {
		t.Logf("%s: running playwright test %s on node 0", time.Now().Format(time.RFC3339), testName)
		stdout, stderr, err = RunCommandOnNode(t, tc, 0, line)
		if err != nil {
			return stdout, stderr, fmt.Errorf("fail to run playwright test %s on node %s: %v", testName, tc.Nodes[0], err)
		}
	}
	return stdout, stderr, nil
}

func generateAndCopySupportBundle(t *testing.T, tc *cluster.Output) {
	t.Logf("%s: generating support bundle", time.Now().Format(time.RFC3339))
	line := []string{"collect-support-bundle.sh"}
	if stdout, stderr, err := RunCommandOnNode(t, tc, 0, line); err != nil {
		t.Logf("stdout: %s", stdout)
		t.Logf("stderr: %s", stderr)
		t.Errorf("fail to generate support bundle: %v", err)
	}

	t.Logf("%s: copying host support bundle to local machine", time.Now().Format(time.RFC3339))
	if err := cluster.CopyFileFromNode(tc.Nodes[0], "/root/host.tar.gz", "support-bundle-host.tar.gz"); err != nil {
		t.Errorf("fail to copy host support bundle to local machine: %v", err)
	}
	t.Logf("%s: copying cluster support bundle to local machine", time.Now().Format(time.RFC3339))
	if err := cluster.CopyFileFromNode(tc.Nodes[0], "/root/cluster.tar.gz", "support-bundle-cluster.tar.gz"); err != nil {
		t.Errorf("fail to copy cluster support bundle to local machine: %v", err)
	}
}

func copyPlaywrightReport(t *testing.T, tc *cluster.Output) {
	line := []string{"tar", "-czf", "playwright-report.tar.gz", "-C", "/automation/playwright/playwright-report", "."}
	if tc.Proxy != "" {
		t.Logf("%s: compressing playwright report on proxy node", time.Now().Format(time.RFC3339))
		if _, _, err := RunCommandOnProxyNode(t, tc, line); err != nil {
			t.Errorf("fail to compress playwright report on node %s: %v", tc.Proxy, err)
			return
		}
		t.Logf("%s: copying playwright report to local machine", time.Now().Format(time.RFC3339))
		if err := cluster.CopyFileFromNode(tc.Proxy, "/root/playwright-report.tar.gz", "playwright-report.tar.gz"); err != nil {
			t.Errorf("fail to copy playwright report to local machine: %v", err)
		}
	} else {
		t.Logf("%s: compressing playwright report on node 0", time.Now().Format(time.RFC3339))
		if _, _, err := RunCommandOnNode(t, tc, 0, line); err != nil {
			t.Errorf("fail to compress playwright report on node %s: %v", tc.Nodes[0], err)
			return
		}
		t.Logf("%s: copying playwright report to local machine", time.Now().Format(time.RFC3339))
		if err := cluster.CopyFileFromNode(tc.Nodes[0], "/root/playwright-report.tar.gz", "playwright-report.tar.gz"); err != nil {
			t.Errorf("fail to copy playwright report to local machine: %v", err)
		}
	}
}

func cleanupCluster(t *testing.T, tc *cluster.Output) {
	if t.Failed() {
		generateAndCopySupportBundle(t, tc)
		copyPlaywrightReport(t, tc)
	}
	tc.Destroy()
}
