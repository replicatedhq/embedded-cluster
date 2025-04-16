package cmx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

type ClusterInput struct {
	T                      *testing.T
	Nodes                  int
	Distribution           string
	Version                string
	InstanceType           string
	DiskSize               int
	SupportBundleNodeIndex int
}

type Cluster struct {
	Nodes []Node

	t                      *testing.T
	network                *Network `json:"-"`
	supportBundleNodeIndex int
}

type Node struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	sshEndpoint     string `json:"-"`
	adminConsoleURL string `json:"-"`
}

type Network struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func NewCluster(in *ClusterInput) *Cluster {
	c := &Cluster{t: in.T, supportBundleNodeIndex: in.SupportBundleNodeIndex}
	c.Nodes = make([]Node, in.Nodes)

	network, err := NewNetwork(in)
	if err != nil {
		in.T.Fatalf("failed to create network: %v", err)
	}
	c.network = network

	g := new(errgroup.Group)
	var mu sync.Mutex

	for i := range c.Nodes {
		func(i int) {
			g.Go(func() error {
				node, err := NewNode(in, i, network.ID)
				if err != nil {
					return fmt.Errorf("failed to create node %d: %w", i, err)
				}
				mu.Lock()
				c.Nodes[i] = *node
				mu.Unlock()
				return nil
			})
		}(i)
	}

	if err := g.Wait(); err != nil {
		in.T.Fatalf("failed to create nodes: %v", err)
	}

	return c
}

func NewNetwork(in *ClusterInput) (*Network, error) {
	name := fmt.Sprintf("ec-e2e-%s", uuid.New().String())
	in.T.Logf("creating network %s", name)

	output, err := exec.Command("replicated", "network", "create", "--name", name, "--wait", "5m", "-ojson").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to create network %s: %v: %s", name, err, string(output))
	}

	var networks []Network
	if err := json.Unmarshal(output, &networks); err != nil {
		return nil, fmt.Errorf("failed to parse networks output: %v", err)
	}
	if len(networks) != 1 {
		return nil, fmt.Errorf("expected 1 network, got %d", len(networks))
	}
	return &networks[0], nil
}

func NewNode(in *ClusterInput, index int, networkID string) (*Node, error) {
	nodeName := fmt.Sprintf("node%d", index)
	in.T.Logf("creating node %s", nodeName)

	args := []string{
		"vm", "create",
		"--name", nodeName,
		"--network", networkID,
		"--wait", "5m",
		"-ojson",
	}
	if in.Distribution != "" {
		args = append(args, "--distribution", in.Distribution)
	}
	if in.Version != "" {
		args = append(args, "--version", in.Version)
	}
	if in.InstanceType != "" {
		args = append(args, "--instance-type", in.InstanceType)
	}
	if in.DiskSize != 0 {
		args = append(args, "--disk", strconv.Itoa(in.DiskSize))
	}

	output, err := exec.Command("replicated", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to create node %s: %v: %s", nodeName, err, string(output))
	}

	var nodes []Node
	if err := json.Unmarshal(output, &nodes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node: %v: %s", err, string(output))
	}
	if len(nodes) != 1 {
		return nil, fmt.Errorf("expected 1 node, got %d", len(nodes))
	}
	node := nodes[0]

	sshEndpoint, err := getSSHEndpoint(node.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get ssh endpoint for node %s: %v", nodeName, err)
	}
	node.sshEndpoint = sshEndpoint

	in.T.Logf("ensuring assets dir on node %s", node.Name)
	if err := ensureAssetsDir(node); err != nil {
		return nil, fmt.Errorf("failed to ensure assets dir on node %s: %v", node.Name, err)
	}

	in.T.Logf("copying scripts to node %s", node.Name)
	if err := copyScriptsToNode(node); err != nil {
		return nil, fmt.Errorf("failed to copy scripts to node %s: %v", node.Name, err)
	}

	if index == 0 {
		in.T.Logf("exposing port 30003 on node %s", node.Name)
		hostname, err := exposePort(node, "30003")
		if err != nil {
			return nil, fmt.Errorf("failed to expose port: %v", err)
		}
		node.adminConsoleURL = fmt.Sprintf("http://%s", hostname)
	}

	return &node, nil
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
	_, stderr, err = runCommandOnNode(node, []string{"rm", "-f", "/tmp/scripts.tgz"})
	if err != nil {
		return fmt.Errorf("failed to clean up scripts archive: %v: %s", err, stderr)
	}

	return nil
}

func getSSHEndpoint(nodeID string) (string, error) {
	cmd := exec.Command("replicated", "vm", "ssh-endpoint", nodeID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get SSH endpoint for node %s: %v: %s", nodeID, err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

func (c *Cluster) Airgap() error {
	// Update network policy to airgap
	cmd := exec.Command("replicated", "network", "update", "policy",
		"--id", c.network.ID,
		"--policy=airgap")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update network policy: %v: %s", err, string(output))
	}

	// Wait until the nodes are airgapped
	for node := range c.Nodes {
		if err := c.waitUntilAirgapped(node); err != nil {
			return fmt.Errorf("failed to wait until node %d is airgapped: %v", node, err)
		}
	}

	return nil
}

func (c *Cluster) RefreshSSHEndpoints() error {
	for i := range c.Nodes {
		sshEndpoint, err := getSSHEndpoint(c.Nodes[i].ID)
		if err != nil {
			return fmt.Errorf("failed to get SSH endpoint for node %d: %v", i, err)
		}
		c.Nodes[i].sshEndpoint = sshEndpoint
	}
	return nil
}

func (c *Cluster) waitUntilAirgapped(node int) error {
	timeout := time.After(2 * time.Minute)
	tick := time.Tick(5 * time.Second)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out waiting for node to be airgapped after 1 minute")
		case <-tick:
			_, _, err := c.RunCommandOnNode(node, []string{"curl", "--connect-timeout", "5", "www.google.com"})
			if err != nil {
				c.t.Logf("node %d is airgapped successfully", node)
				return nil
			}
			c.t.Logf("node %d is not airgapped yet", node)
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
			c.t.Logf("failed to destroy node %s: %v: %s", node.Name, err, string(output))
		}
	}
}

func (c *Cluster) RunCommandOnNode(node int, line []string, envs ...map[string]string) (string, string, error) {
	return runCommandOnNode(c.Nodes[node], line, envs...)
}

func runCommandOnNode(node Node, line []string, envs ...map[string]string) (string, string, error) {
	line = append([]string{"sudo"}, line...)
	cmd := exec.Command("ssh", append(sshArgs(), node.sshEndpoint, strings.Join(line, " "))...)

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
	c.t.Logf("%s: bypassing kurl-proxy", time.Now().Format(time.RFC3339))
	_, stderr, err := c.RunCommandOnNode(0, []string{"bypass-kurl-proxy.sh"}, envs...)
	if err != nil {
		return fmt.Errorf("fail to bypass kurl-proxy: %v: %s", err, string(stderr))
	}
	c.t.Logf("%s: installing playwright", time.Now().Format(time.RFC3339))
	cmd := exec.Command("sh", "-c", "cd playwright && npm ci && npx playwright install --with-deps")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fail to install playwright: %v: %s", err, string(out))
	}
	return nil
}

func (c *Cluster) RunPlaywrightTest(testName string, args ...string) (string, string, error) {
	c.t.Logf("%s: running playwright test %s", time.Now().Format(time.RFC3339), testName)
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
			c.t.Logf("%s: generating host support bundle from node %d", time.Now().Format(time.RFC3339), i)
			if stdout, stderr, err := c.RunCommandOnNode(i, []string{"collect-support-bundle-host.sh"}, envs...); err != nil {
				c.t.Logf("stdout: %s", stdout)
				c.t.Logf("stderr: %s", stderr)
				c.t.Logf("fail to generate support from node %d bundle: %v", i, err)
				return
			}

			c.t.Logf("%s: copying host support bundle from node %d to local machine", time.Now().Format(time.RFC3339), i)
			src := "host.tar.gz"
			dst := fmt.Sprintf("support-bundle-host-%d.tar.gz", i)
			if err := copyFileFromNode(c.Nodes[i], src, dst); err != nil {
				c.t.Logf("fail to copy support bundle from node %d: %v", i, err)
				return
			}
		}(i, &wg)
	}

	c.t.Logf("%s: generating cluster support bundle from node %d", time.Now().Format(time.RFC3339), c.supportBundleNodeIndex)
	if stdout, stderr, err := c.RunCommandOnNode(c.supportBundleNodeIndex, []string{"collect-support-bundle-cluster.sh"}, envs...); err != nil {
		c.t.Logf("stdout: %s", stdout)
		c.t.Logf("stderr: %s", stderr)
		c.t.Logf("fail to generate cluster support from node %d bundle: %v", c.supportBundleNodeIndex, err)
	} else {
		c.t.Logf("%s: copying cluster support bundle from node %d to local machine", time.Now().Format(time.RFC3339), c.supportBundleNodeIndex)
		src := "cluster.tar.gz"
		dst := "support-bundle-cluster.tar.gz"
		if err := copyFileFromNode(c.Nodes[c.supportBundleNodeIndex], src, dst); err != nil {
			c.t.Logf("fail to copy cluster support bundle from node %d: %v", c.supportBundleNodeIndex, err)
		}
	}

	wg.Wait()
}

func (c *Cluster) copyPlaywrightReport() {
	c.t.Logf("%s: compressing playwright report", time.Now().Format(time.RFC3339))
	cmd := exec.Command("tar", "-czf", "playwright-report.tar.gz", "-C", "./playwright/playwright-report", ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.t.Logf("fail to compress playwright report: %v: %s", err, string(out))
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

	cmd := exec.Command("scp", append(sshArgs(), src, fmt.Sprintf("%s/%s", scpEndpoint, dst))...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy file to node: %v: %s", err, string(output))
	}
	return nil
}

func copyFileFromNode(node Node, src, dst string) error {
	scpEndpoint := strings.Replace(node.sshEndpoint, "ssh://", "scp://", 1)

	cmd := exec.Command("scp", append(sshArgs(), fmt.Sprintf("%s/%s", scpEndpoint, src), dst)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to copy file from node: %v: %s", err, string(output))
	}
	return nil
}

func sshArgs() []string {
	return []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=10",
	}
}
