package k0s

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
)

type K0sCluster struct {
	nodes        []*K0sNode
	kubeconfig   string
	pool         *dockertest.Pool
	sharedTmpVol string // Shared volume for k0s binaries
	dataDir      string
}

type K0sNode struct {
	name      string
	container *dockertest.Resource
	volume    string
}

func bootstrapK0sCluster(t *testing.T, nodeNames []string, k0sVersion string, dataDir string) *K0sCluster {
	pool, err := dockertest.NewPool("")
	require.NoError(t, err)

	// Create shared volume for k0s binaries
	sharedTmpVolName := fmt.Sprintf("k0s-shared-tmp-%d", time.Now().Unix())

	cluster := &K0sCluster{
		nodes:        make([]*K0sNode, len(nodeNames)),
		pool:         pool,
		sharedTmpVol: sharedTmpVolName,
		dataDir:      dataDir,
	}

	// Create VMs for all nodes
	for i, name := range nodeNames {
		t.Logf("Creating VM for node %s", name)
		node := createK0sVM(t, pool, name, sharedTmpVolName, dataDir)
		cluster.nodes[i] = node
	}

	// Download k0s binary to shared volume (once for all nodes)
	t.Logf("Downloading k0s %s to shared volume", k0sVersion)
	downloadK0sToSharedVolume(t, cluster.nodes[0], k0sVersion)

	// Bootstrap first node as controller
	t.Logf("Installing k0s %s on %s", k0sVersion, nodeNames[0])
	installK0sController(t, cluster.nodes[0], k0sVersion)

	// Get kubeconfig from first node
	cluster.kubeconfig = getKubeconfig(t, cluster.nodes[0])

	// Generate join token
	joinToken := generateJoinToken(t, cluster.nodes[0])

	// Join remaining nodes concurrently
	t.Logf("Joining %d nodes to cluster concurrently", len(cluster.nodes)-1)
	var wg sync.WaitGroup
	for i := 1; i < len(cluster.nodes); i++ {
		wg.Add(1)
		go func(idx int, nodeName string) {
			defer wg.Done()
			t.Logf("Joining node %s to cluster", nodeName)
			joinK0sController(t, cluster.nodes[idx], k0sVersion, joinToken)
		}(i, nodeNames[i])
	}
	wg.Wait()

	// Wait for all nodes to be ready
	waitForNodesReady(t, cluster, len(nodeNames))

	return cluster
}

func createK0sVM(t *testing.T, pool *dockertest.Pool, name string, sharedTmpVol string, dataDir string) *K0sNode {
	// Get distro image from env or use default
	distro := os.Getenv("EC_TEST_DISTRO")
	if distro == "" {
		distro = "debian-bookworm"
	}

	// Run container with systemd
	container, err := pool.RunWithOptions(&dockertest.RunOptions{
		Name:       name,
		Repository: "replicated/ec-distro",
		Tag:        distro,
		Mounts: []string{
			fmt.Sprintf("%s:%s", dataDir, dataDir),
			fmt.Sprintf("%s:/shared-tmp", sharedTmpVol), // Mount shared volume
		},
		Privileged: true,
	}, func(config *docker.HostConfig) {
		config.RestartPolicy = docker.RestartPolicy{
			Name: "unless-stopped",
		}
	})
	require.NoError(t, err)

	// Wait for systemd to be ready
	err = pool.Retry(func() error {
		code, err := container.Exec([]string{"systemctl", "status"}, dockertest.ExecOptions{})
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("systemd not ready")
		}
		return nil
	})
	require.NoError(t, err)

	return &K0sNode{
		name:      name,
		container: container,
		// volume:    volumeName,
	}
}

func downloadK0sToSharedVolume(t *testing.T, node *K0sNode, version string) {
	// Download k0s binary to shared volume (format: k0s-{version}-{arch})
	downloadCmd := fmt.Sprintf(
		"curl -sSLf https://github.com/k0sproject/k0s/releases/download/%s/k0s-%s-amd64 -o /shared-tmp/k0s-%s && chmod +x /shared-tmp/k0s-%s",
		version, version, version, version,
	)
	execCommand(t, node, []string{"sh", "-c", downloadCmd})
	t.Logf("Downloaded k0s %s to shared volume", version)
}

func installK0sController(t *testing.T, node *K0sNode, version string) {
	// Copy k0s binary from shared volume
	copyCmd := fmt.Sprintf("cp /shared-tmp/k0s-%s /usr/local/bin/k0s && chmod +x /usr/local/bin/k0s", version)
	execCommand(t, node, []string{"sh", "-c", copyCmd})

	// Install k0s as controller
	execCommand(t, node, []string{"k0s", "install", "controller", "--enable-worker"})

	// Start k0s
	execCommand(t, node, []string{"systemctl", "start", "k0scontroller"})

	// Wait for k0s to be ready
	waitForK0sReady(t, node)
}

func generateJoinToken(t *testing.T, node *K0sNode) string {
	output := execCommandWithOutput(t, node, []string{"k0s", "token", "create", "--role=controller"})
	return strings.TrimSpace(output)
}

func joinK0sController(t *testing.T, node *K0sNode, version string, token string) {
	// Copy k0s binary from shared volume
	copyCmd := fmt.Sprintf("cp /shared-tmp/k0s-%s /usr/local/bin/k0s && chmod +x /usr/local/bin/k0s", version)
	execCommand(t, node, []string{"sh", "-c", copyCmd})

	// Write token to file
	tokenPath := "/tmp/k0s-token"
	execCommand(t, node, []string{"sh", "-c", fmt.Sprintf("echo '%s' > %s", token, tokenPath)})

	// Install k0s as controller with token
	execCommand(t, node, []string{"k0s", "install", "controller", "--enable-worker", "--token-file", tokenPath})

	// Start k0s
	execCommand(t, node, []string{"systemctl", "start", "k0scontroller"})

	// Wait for k0s to be ready
	waitForK0sReady(t, node)
}

func execCommand(t *testing.T, node *K0sNode, cmd []string) {
	code, err := node.container.Exec(cmd, dockertest.ExecOptions{})
	require.NoError(t, err)
	require.Equal(t, 0, code, "Command failed: %v", cmd)
}

func execCommandWithOutput(t *testing.T, node *K0sNode, cmd []string) string {
	var stdout bytes.Buffer
	code, err := node.container.Exec(cmd, dockertest.ExecOptions{
		StdOut: &stdout,
	})
	require.NoError(t, err)
	require.Equal(t, 0, code, "Command failed: %v", cmd)
	return stdout.String()
}

func getKubeconfig(t *testing.T, node *K0sNode) string {
	output := execCommandWithOutput(t, node, []string{"k0s", "kubeconfig", "admin"})

	// Write kubeconfig to temp file
	kubeconfigPath := filepath.Join(t.TempDir(), "kubeconfig")
	err := os.WriteFile(kubeconfigPath, []byte(output), 0600)
	require.NoError(t, err)

	return kubeconfigPath
}

func waitForK0sReady(t *testing.T, node *K0sNode) {
	for i := 0; i < 60; i++ {
		code, _ := node.container.Exec([]string{"k0s", "kubectl", "get", "nodes"}, dockertest.ExecOptions{})
		if code == 0 {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatal("k0s failed to become ready")
}

func waitForNodesReady(t *testing.T, cluster *K0sCluster, expectedNodes int) {
	for i := 0; i < 60; i++ {
		code, err := cluster.nodes[0].container.Exec(
			[]string{"k0s", "kubectl", "get", "nodes", "--no-headers"},
			dockertest.ExecOptions{},
		)
		if err == nil && code == 0 {
			// Count ready nodes
			var stdout bytes.Buffer
			cluster.nodes[0].container.Exec(
				[]string{"k0s", "kubectl", "get", "nodes", "--no-headers"},
				dockertest.ExecOptions{StdOut: &stdout},
			)
			lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
			if len(lines) == expectedNodes {
				return
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("Expected %d nodes to be ready", expectedNodes)
}

func (c *K0sCluster) Cleanup() {
	for _, node := range c.nodes {
		c.pool.Purge(node.container)
		if node.volume != "" {
			// Remove node volume
			c.pool.Client.RemoveVolume(node.volume)
		}
	}
	// Remove shared volume
	if c.sharedTmpVol != "" {
		c.pool.Client.RemoveVolume(c.sharedTmpVol)
	}
}

func (c *K0sCluster) GetKubeconfig() string {
	return c.kubeconfig
}
