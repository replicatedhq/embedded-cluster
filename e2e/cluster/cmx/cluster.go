package cmx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type Node struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	sshEndpoint     string `json:"-"`
	adminConsoleURL string `json:"-"`
}

type Cluster struct {
	Nodes []Node

	T                      *testing.T
	SupportBundleNodeIndex int
}

type ClusterInput struct {
	T                      *testing.T
	Nodes                  int
	Distro                 string
	SupportBundleNodeIndex int
}

func NewCluster(in *ClusterInput) *Cluster {
	c := &Cluster{T: in.T, SupportBundleNodeIndex: in.SupportBundleNodeIndex}
	c.Nodes = make([]Node, in.Nodes)

	var wg sync.WaitGroup
	wg.Add(in.Nodes)
	var mu sync.Mutex

	for i := range c.Nodes {
		go func(i int) {
			defer wg.Done()
			node := NewNode(in, i)
			mu.Lock()
			c.Nodes[i] = node
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	return c
}

func NewNode(in *ClusterInput, index int) Node {
	in.T.Logf("creating node%d", index)
	nodeName := fmt.Sprintf("node%d", index)

	cmd := exec.Command("replicated", "vm", "create",
		"--name", nodeName,
		"--distribution", strings.Split(in.Distro, "/")[0],
		"--version", strings.Split(in.Distro, "/")[1],
		"--wait", "2m")

	output, err := cmd.CombinedOutput()
	if err != nil {
		in.T.Fatalf("failed to create node %s: %v: %s", nodeName, err, string(output))
	}

	nodeID := getNodeIDByName(in.T, nodeName)
	if nodeID == "" {
		in.T.Fatalf("failed to get node ID for %s", nodeName)
	}

	sshEndpoint, err := getSSHEndpoint(nodeID)
	if err != nil {
		in.T.Fatalf("failed to get ssh endpoint for node %s: %v", nodeName, err)
	}

	node := Node{
		ID:          nodeID,
		Name:        nodeName,
		sshEndpoint: sshEndpoint,
	}

	in.T.Logf("ensuring assets dir on node %s", node.Name)
	if err := ensureAssetsDir(node); err != nil {
		in.T.Fatalf("failed to ensure assets dir on node %s: %v", node.Name, err)
	}

	in.T.Logf("copying scripts to node %s", node.Name)
	if err := copyScriptsToNode(node); err != nil {
		in.T.Fatalf("failed to copy scripts to node %s: %v", node.Name, err)
	}

	if index == 0 {
		in.T.Logf("exposing port 30003 on node %s", node.Name)
		hostname, err := exposePort(node, "30003")
		if err != nil {
			in.T.Fatalf("failed to expose port: %v", err)
		}
		node.adminConsoleURL = fmt.Sprintf("http://%s", hostname)
	}

	return node
}

func ensureAssetsDir(node Node) error {
	stdout, stderr, err := runCommandOnNode(node, []string{"mkdir", "-p", "/assets"})
	if err != nil {
		return fmt.Errorf("failed to create directory: %v: %s: %s", err, stdout, stderr)
	}
	return nil
}

func copyScriptsToNode(node Node) error {
	// Create a temporary directory for the archive
	tempDir, err := os.MkdirTemp("", "scripts-archive")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create the archive
	archivePath := filepath.Join(tempDir, "scripts.tgz")
	output, err := exec.Command("tar", "-czf", archivePath, "-C", "scripts", ".").CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create scripts archive: %v: %s", err, string(output))
	}

	// Copy the archive to the node
	if err := copyFileToNode(node, archivePath, "/tmp/scripts.tgz"); err != nil {
		return fmt.Errorf("failed to copy scripts archive to node: %v", err)
	}

	// Extract the archive in /usr/local/bin
	_, stderr, err := runCommandOnNode(node, []string{"tar", "-xzf", "/tmp/scripts.tgz", "-C", "/usr/local/bin"})
	if err != nil {
		return fmt.Errorf("failed to extract scripts archive: %v: %s", err, stderr)
	}

	// Clean up the archive on the node
	_, stderr, err = runCommandOnNode(node, []string{"rm", "/tmp/scripts.tgz"})
	if err != nil {
		return fmt.Errorf("failed to clean up scripts archive: %v: %s", err, stderr)
	}

	return nil
}

func getNodeIDByName(t *testing.T, name string) string {
	cmd := exec.Command("replicated", "vm", "ls", "--output", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to list nodes: %v: %s", err, string(output))
	}

	var nodes []Node
	if err := json.Unmarshal(output, &nodes); err != nil {
		t.Fatalf("failed to unmarshal nodes: %v: %s", err, string(output))
	}

	for _, node := range nodes {
		if node.Name == name {
			return node.ID
		}
	}

	t.Fatalf("node %s not found", name)
	return ""
}

func getSSHEndpoint(nodeID string) (string, error) {
	cmd := exec.Command("replicated", "vm", "ssh-endpoint", nodeID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get SSH endpoint for node %s: %v: %s", nodeID, err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

func (c *Cluster) AirgapNode(node int) error {
	// Get networks
	cmd := exec.Command("replicated", "network", "ls", "-ojson")
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.T.Fatalf("failed to list networks: %v: %s", err, string(output))
	}

	// Parse JSON output
	var networks []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output, &networks); err != nil {
		c.T.Fatalf("failed to parse networks output: %v", err)
	}

	// Find network ID for the node
	var networkID string
	for _, network := range networks {
		if network.Name == c.Nodes[node].Name {
			networkID = network.ID
			break
		}
	}
	if networkID == "" {
		c.T.Fatalf("could not find network ID for node %s", c.Nodes[node].Name)
	}

	// Update network policy to airgap
	cmd = exec.Command("replicated", "network", "update", "policy",
		"--id", networkID,
		"--policy=airgap")
	output, err = cmd.CombinedOutput()
	if err != nil {
		c.T.Fatalf("failed to update network policy: %v: %s", err, string(output))
	}

	// Wait until the node is airgapped
	if err := c.waitUntilAirgapped(node); err != nil {
		c.T.Fatalf("failed to wait until node is airgapped: %v", err)
	}

	return nil
}

func (c *Cluster) waitUntilAirgapped(node int) error {
	timeout := time.After(1 * time.Minute)
	tick := time.Tick(1 * time.Second)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out waiting for node to be airgapped after 1 minute")
		case <-tick:
			_, _, err := c.RunCommandOnNode(node, []string{"curl", "--connect-timeout", "5", "www.google.com"})
			if err != nil {
				c.T.Logf("node is airgapped successfully")
				return nil
			}
			c.T.Logf("node is not airgapped yet")
		}
	}
}

func (c *Cluster) Cleanup(envs ...map[string]string) {
	c.generateSupportBundle(envs...)
	c.copyPlaywrightReport()
	c.Destroy()
}

func (c *Cluster) Destroy() {
	for _, node := range c.Nodes {
		cmd := exec.Command("replicated", "vm", "rm", node.ID)
		output, err := cmd.CombinedOutput()
		if err != nil {
			c.T.Logf("failed to destroy node %s: %v: %s", node.Name, err, string(output))
		}
	}
}

func (c *Cluster) RunCommandOnNode(node int, line []string, envs ...map[string]string) (string, string, error) {
	return runCommandOnNode(c.Nodes[node], line, envs...)
}

func runCommandOnNode(node Node, line []string, envs ...map[string]string) (string, string, error) {
	line = append([]string{"sudo"}, line...)
	cmd := exec.Command("ssh", "-o", "StrictHostKeyChecking=no", node.sshEndpoint, strings.Join(line, " "))

	for _, env := range envs {
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 255 {
		// check if this is a reset-installation command that resulted in exit code 255
		// as this is expected behavior when the node reboots and the ssh connection is lost
		if strings.Contains(strings.Join(line, " "), "reset-installation") {
			return stdout.String(), stderr.String(), nil
		}
	}

	return stdout.String(), stderr.String(), err
}

func (c *Cluster) SetupPlaywrightAndRunTest(testName string, args ...string) (string, string, error) {
	if err := c.SetupPlaywright(); err != nil {
		return "", "", fmt.Errorf("failed to setup playwright: %w", err)
	}
	return c.RunPlaywrightTest(testName, args...)
}

func (c *Cluster) SetupPlaywright(envs ...map[string]string) error {
	c.T.Logf("%s: bypassing kurl-proxy", time.Now().Format(time.RFC3339))
	_, stderr, err := c.RunCommandOnNode(0, []string{"bypass-kurl-proxy.sh"}, envs...)
	if err != nil {
		return fmt.Errorf("fail to bypass kurl-proxy: %v: %s", err, string(stderr))
	}
	c.T.Logf("%s: installing playwright", time.Now().Format(time.RFC3339))
	cmd := exec.Command("sh", "-c", "cd playwright && npm ci && npx playwright install --with-deps")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fail to install playwright: %v: %s", err, string(out))
	}
	return nil
}

func (c *Cluster) RunPlaywrightTest(testName string, args ...string) (string, string, error) {
	c.T.Logf("%s: running playwright test %s", time.Now().Format(time.RFC3339), testName)
	cmdArgs := []string{testName}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("scripts/playwright.sh", cmdArgs...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("BASE_URL=%s", c.Nodes[0].adminConsoleURL))
	cmd.Env = append(cmd.Env, "PLAYWRIGHT_DIR=./playwright")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stdout.String(), stderr.String(), fmt.Errorf("fail to run playwright test %s: %v", testName, err)
	}
	return stdout.String(), stderr.String(), nil
}

func (c *Cluster) generateSupportBundle(envs ...map[string]string) {
	wg := sync.WaitGroup{}
	wg.Add(len(c.Nodes))

	for i := range c.Nodes {
		go func(i int, wg *sync.WaitGroup) {
			defer wg.Done()
			c.T.Logf("%s: generating host support bundle from node %d", time.Now().Format(time.RFC3339), i)
			if stdout, stderr, err := c.RunCommandOnNode(i, []string{"collect-support-bundle-host.sh"}, envs...); err != nil {
				c.T.Logf("stdout: %s", stdout)
				c.T.Logf("stderr: %s", stderr)
				c.T.Logf("fail to generate support from node %d bundle: %v", i, err)
				return
			}

			c.T.Logf("%s: copying host support bundle from node %d to local machine", time.Now().Format(time.RFC3339), i)
			src := "host.tar.gz"
			dst := fmt.Sprintf("support-bundle-host-%d.tar.gz", i)
			if err := copyFileFromNode(c.Nodes[i], src, dst); err != nil {
				c.T.Logf("fail to copy support bundle from node %d: %v", i, err)
				return
			}
		}(i, &wg)
	}

	c.T.Logf("%s: generating cluster support bundle from node %d", time.Now().Format(time.RFC3339), c.SupportBundleNodeIndex)
	if stdout, stderr, err := c.RunCommandOnNode(c.SupportBundleNodeIndex, []string{"collect-support-bundle-cluster.sh"}, envs...); err != nil {
		c.T.Logf("stdout: %s", stdout)
		c.T.Logf("stderr: %s", stderr)
		c.T.Logf("fail to generate cluster support from node %d bundle: %v", c.SupportBundleNodeIndex, err)
	} else {
		c.T.Logf("%s: copying cluster support bundle from node %d to local machine", time.Now().Format(time.RFC3339), c.SupportBundleNodeIndex)
		src := "cluster.tar.gz"
		dst := "support-bundle-cluster.tar.gz"
		if err := copyFileFromNode(c.Nodes[c.SupportBundleNodeIndex], src, dst); err != nil {
			c.T.Logf("fail to copy cluster support bundle from node %d: %v", c.SupportBundleNodeIndex, err)
		}
	}

	wg.Wait()
}

func (c *Cluster) copyPlaywrightReport() {
	c.T.Logf("%s: compressing playwright report", time.Now().Format(time.RFC3339))
	cmd := exec.Command("tar", "-czf", "playwright-report.tar.gz", "-C", "./playwright/playwright-report", ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.T.Logf("fail to compress playwright report: %v: %s", err, string(out))
	}
}

func exposePort(node Node, port string) (string, error) {
	cmd := exec.Command("replicated", "vm", "port", "expose", node.ID, "--port", port)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to expose port: %v: %s", err, string(output))
	}

	cmd = exec.Command("replicated", "vm", "port", "ls", node.ID, "-ojson")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get port info: %v: %s", err, string(output))
	}

	var ports []struct {
		Hostname string `json:"hostname"`
	}
	if err := json.Unmarshal(output, &ports); err != nil {
		return "", fmt.Errorf("failed to unmarshal port info: %v", err)
	}

	if len(ports) == 0 {
		return "", fmt.Errorf("no ports found for node %s", node.ID)
	}
	return ports[0].Hostname, nil
}

func copyFileToNode(node Node, src, dst string) error {
	scpEndpoint := strings.Replace(node.sshEndpoint, "ssh://", "scp://", 1)

	cmd := exec.Command("scp", "-o", "StrictHostKeyChecking=no", src, fmt.Sprintf("%s/%s", scpEndpoint, dst))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy file to node: %v: %s", err, string(output))
	}
	return nil
}

func copyFileFromNode(node Node, src, dst string) error {
	scpEndpoint := strings.Replace(node.sshEndpoint, "ssh://", "scp://", 1)

	cmd := exec.Command("scp", "-o", "StrictHostKeyChecking=no", fmt.Sprintf("%s/%s", scpEndpoint, src), dst)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy file from node: %v: %s", err, string(output))
	}
	return nil
}
