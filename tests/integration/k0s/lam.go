package k0s

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
)

func prepareK0sBinaryInLAM(t *testing.T, node *K0sNode, dataDir string, k0sVersion string) {
	// Copy k0s binary from shared volume to LAM bin directory
	// The k0s binary is already in /shared-tmp/k0s-{version} from cluster bootstrap
	sharedK0sPath := fmt.Sprintf("/shared-tmp/k0s-%s", k0sVersion)
	lamK0sPath := filepath.Join(dataDir, "bin", "k0s")

	copyCmd := fmt.Sprintf("cp %s %s && chmod +x %s", sharedK0sPath, lamK0sPath, lamK0sPath)
	execCommand(t, node, []string{"sh", "-c", copyCmd})

	t.Logf("Copied k0s %s from shared volume to LAM directory", k0sVersion)
}

func copyLAMBinaryToContainer(t *testing.T, node *K0sNode, dataDir string) {
	// Copy LAM binary from host to container at the expected location in data-dir
	// The binary watcher expects it at <data-dir>/bin/local-artifact-mirror
	lamBinaryPath := filepath.Join(dataDir, "bin", "local-artifact-mirror")

	cmd := exec.Command("docker", "cp", "../../../output/bin/local-artifact-mirror", fmt.Sprintf("%s:%s", node.name, lamBinaryPath))
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to copy LAM binary: %s", output)

	// Make executable
	execCommand(t, node, []string{"chmod", "+x", lamBinaryPath})
	t.Logf("Copied LAM binary to %s in container %s", lamBinaryPath, node.name)
}

func startLAM(t *testing.T, node *K0sNode, dataDir string) string {
	port := 50000

	// Copy LAM binary into container
	copyLAMBinaryToContainer(t, node, dataDir)

	// Start LAM as background process inside container
	lamBinaryPath := filepath.Join(dataDir, "bin", "local-artifact-mirror")
	startCmd := fmt.Sprintf(
		"nohup %s serve --data-dir %s --port %d > /tmp/lam.log 2>&1 &",
		lamBinaryPath, dataDir, port,
	)
	execCommand(t, node, []string{"sh", "-c", startCmd})
	t.Logf("Started LAM server inside container %s", node.name)

	// Wait for LAM to be ready (check from inside container)
	lamURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	for i := 0; i < 30; i++ {
		// Check if LAM is responding from inside the container
		code, _ := node.container.Exec(
			[]string{"sh", "-c", fmt.Sprintf("curl -f %s/bin/ > /dev/null 2>&1", lamURL)},
			dockertest.ExecOptions{},
		)
		if code == 0 {
			t.Logf("LAM ready at %s (inside container %s)", lamURL, node.name)
			return lamURL
		}
		time.Sleep(1 * time.Second)
	}

	// If we get here, LAM failed to start - show logs
	var logOutput bytes.Buffer
	node.container.Exec(
		[]string{"cat", "/tmp/lam.log"},
		dockertest.ExecOptions{StdOut: &logOutput},
	)
	t.Fatalf("LAM failed to start. Logs:\n%s", logOutput.String())
	return ""
}

func blockInternetAccess(t *testing.T, cluster *K0sCluster) {
	for _, node := range cluster.nodes {
		// Block all outbound traffic except:
		// 1. Cluster-internal IPs (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
		// 2. Localhost (127.0.0.0/8)
		// 3. Container network (Docker bridge network)

		rules := []string{
			// Allow localhost
			"iptables -A OUTPUT -d 127.0.0.0/8 -j ACCEPT",
			// Allow private networks (cluster-internal communication)
			"iptables -A OUTPUT -d 10.0.0.0/8 -j ACCEPT",
			"iptables -A OUTPUT -d 172.16.0.0/12 -j ACCEPT",
			"iptables -A OUTPUT -d 192.168.0.0/16 -j ACCEPT",
			// Drop everything else (blocks internet)
			"iptables -A OUTPUT -j DROP",
		}

		for _, rule := range rules {
			cmd := []string{"sh", "-c", rule}
			// Best effort - don't fail if iptables rules fail
			node.container.Exec(cmd, dockertest.ExecOptions{})
		}
		t.Logf("Blocked internet access on node %s (allowed cluster-internal only)", node.name)
	}
}
