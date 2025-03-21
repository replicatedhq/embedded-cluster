package cmx

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
)

const (
	DefaultDistribution = "ubuntu"
	DefaultVersion      = "22.04"
	DefaultTTL          = "2h"
	DefaultInstanceType = "r1.medium"
	DefaultDiskSize     = 50
)

// Cluster implements the cluster.Cluster interface for Replicated VM
type Cluster struct {
	t *testing.T

	gid       string
	networkID string
	nodes     []*node
	proxyNode *node
	sshUser   string
}

type ClusterInput struct {
	T                       *testing.T
	Nodes                   int
	Distribution            string
	Version                 string
	WithProxy               bool
	LicensePath             string
	EmbeddedClusterPath     string
	AirgapInstallBundlePath string
	AirgapUpgradeBundlePath string
}

// NewCluster creates a new CMX cluster using the provided configuration
func NewCluster(ctx context.Context, input ClusterInput) *Cluster {
	if input.T == nil {
		panic("testing.T is required")
	}

	sshUser := os.Getenv("REPLICATEDVM_SSH_USER")
	if sshUser == "" {
		input.T.Fatalf("REPLICATEDVM_SSH_USER is not set")
	}

	c := &Cluster{
		t:       input.T,
		sshUser: sshUser,
		gid:     uuid.New().String(),
	}
	c.t.Cleanup(c.destroy)

	c.t.Logf("Creating network")
	network, err := createNetwork(ctx, c.gid, DefaultTTL)
	if err != nil {
		c.t.Fatalf("Failed to create network: %v", err)
	}
	c.networkID = network.ID

	c.t.Logf("Creating %d nodes", input.Nodes)
	nodes, err := createNodes(ctx, c.gid, c.networkID, DefaultTTL, clusterInputToCreateNodeOpts(input))
	if err != nil {
		c.t.Fatalf("Failed to create nodes: %v", err)
	}
	c.nodes = nodes

	// If proxy is requested, create an additional node
	if input.WithProxy {
		c.t.Logf("Creating proxy node")
		proxyNodes, err := createNodes(ctx, c.gid, c.networkID, DefaultTTL, createNodeOpts{
			Distribution: DefaultDistribution,
			Version:      DefaultVersion,
			Count:        1,
			InstanceType: "r1.small",
			DiskSize:     10,
		})
		if err != nil {
			c.t.Fatalf("Failed to create proxy node: %v", err)
		}
		c.proxyNode = proxyNodes[0]
	}

	for _, node := range c.nodes {
		c.t.Logf("Copying files to node %s", node.ID)
		err := c.copyFilesToNode(ctx, node, input)
		if err != nil {
			c.t.Fatalf("Failed to copy files to node %s: %v", node.ID, err)
		}
		c.t.Logf("Copying dirs to node %s", node.ID)
		err = c.copyDirsToNode(ctx, node)
		if err != nil {
			c.t.Fatalf("Failed to copy dirs to node %s: %v", node.ID, err)
		}
	}

	return c
}

func clusterInputToCreateNodeOpts(input ClusterInput) createNodeOpts {
	opts := createNodeOpts{
		Distribution: input.Distribution,
		Version:      input.Version,
		Count:        input.Nodes,
	}
	if opts.Distribution == "" {
		opts.Distribution = DefaultDistribution
	}
	if opts.Version == "" {
		opts.Version = DefaultVersion
	}
	if opts.Count <= 0 {
		opts.Count = 1
	}
	if opts.InstanceType == "" {
		opts.InstanceType = DefaultInstanceType
	}
	if opts.DiskSize <= 0 {
		opts.DiskSize = DefaultDiskSize
	}
	return opts
}

// Cleanup removes the VM instance
func (c *Cluster) Cleanup(envs ...map[string]string) {
	// TODO: generate support bundle and copy playwright report
	c.destroy()
}

func (c *Cluster) destroy() {
	if c.gid != "" {
		// Best effort cleanup
		c.t.Logf("Cleaning up nodes")
		err := deleteNodesByGroupID(context.Background(), c.gid)
		if err != nil {
			c.t.Logf("Failed to cleanup cluster: %v", err)
		}
	}

	if c.networkID != "" {
		c.t.Logf("Cleaning up network %s", c.networkID)
		err := deleteNetwork(context.Background(), c.networkID)
		if err != nil {
			c.t.Logf("Failed to cleanup network: %v", err)
		}
	}
}

// RunCommandOnNode executes a command on the specified node using replicated vm ssh
func (c *Cluster) RunCommandOnNode(ctx context.Context, node int, command []string, envs ...map[string]string) (string, string, error) {
	c.t.Logf("Running command on node %s: %s", c.nodes[node].ID, strings.Join(command, " "))
	return c.runCommandOnNode(ctx, c.nodes[node], command, envs...)
}

// RunCommandOnProxyNode executes a command on the proxy node
func (c *Cluster) RunCommandOnProxyNode(ctx context.Context, command []string, envs ...map[string]string) (string, string, error) {
	c.t.Logf("Running command on proxy node: %s", strings.Join(command, " "))
	return c.runCommandOnNode(ctx, c.proxyNode, command, envs...)
}

func (c *Cluster) Node(node int) *node {
	return c.nodes[node]
}

func (c *Cluster) runCommandOnNode(ctx context.Context, node *node, command []string, envs ...map[string]string) (string, string, error) {
	args := []string{}
	args = append(args, sshConnectionArgs(node)...)
	args = append(args, "sh", "-c", strings.Join(command, " "))
	cmd := exec.CommandContext(ctx, "ssh", args...)

	env := os.Environ()
	for _, e := range envs {
		for k, v := range e {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	cmd.Env = env

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()

	stdout := outBuf.String()
	stderr := errBuf.String()

	return stdout, stderr, err
}

func (c *Cluster) copyFilesToNode(ctx context.Context, node *node, in ClusterInput) error {
	files := map[string]string{
		in.LicensePath:             "/assets/license.yaml",            //0644
		in.EmbeddedClusterPath:     "/usr/local/bin/embedded-cluster", //0755
		in.AirgapInstallBundlePath: "/assets/ec-release.tgz",          //0755
		in.AirgapUpgradeBundlePath: "/assets/ec-release-upgrade.tgz",  //0755
	}
	for src, dest := range files {
		if src != "" {
			err := c.CopyFileToNode(ctx, node, src, dest)
			if err != nil {
				return fmt.Errorf("copy file %s to node %s at %s: %v", src, node.ID, dest, err)
			}
		}
	}
	return nil
}

func (c *Cluster) copyDirsToNode(ctx context.Context, node *node) error {
	dirs := map[string]string{
		"../../../scripts": "/usr/local/bin",
		"playwright":       "/automation/playwright",
		"../operator/charts/embedded-cluster-operator/troubleshoot": "/automation/troubleshoot",
	}
	for src, dest := range dirs {
		err := c.CopyDirToNode(ctx, node, src, dest)
		if err != nil {
			return fmt.Errorf("copy dir %s to node %s at %s: %v", src, node.ID, dest, err)
		}
	}
	return nil
}

func (c *Cluster) CopyFileToNode(ctx context.Context, node *node, src, dest string) error {
	c.t.Logf("Copying file %s to node %s at %s", src, node.ID, dest)

	_, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %v", src, err)
	}

	err = c.mkdirOnNode(ctx, node, filepath.Dir(dest))
	if err != nil {
		return fmt.Errorf("mkdir %s on node %s: %v", filepath.Dir(dest), node.ID, err)
	}

	args := []string{src}
	args = append(args, sshConnectionArgs(node)...)
	args[0] = fmt.Sprintf("%s:%s", args[0], dest)
	scpCmd := exec.CommandContext(ctx, "scp", args...)
	output, err := scpCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("err: %v, output: %s", err, string(output))
	}
	return nil
}

func (c *Cluster) CopyDirToNode(ctx context.Context, node *node, src, dest string) error {
	c.t.Logf("Copying dir %s to node %s at %s", src, node.ID, dest)

	_, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %v", src, err)
	}

	err = c.mkdirOnNode(ctx, node, filepath.Dir(dest))
	if err != nil {
		return fmt.Errorf("mkdir %s on node %s: %v", filepath.Dir(dest), node.ID, err)
	}

	args := []string{src}
	args = append(args, sshConnectionArgs(node)...)
	args[0] = fmt.Sprintf("%s:%s", args[0], dest)
	scpCmd := exec.CommandContext(ctx, "scp", args...)
	output, err := scpCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("err: %v, output: %s", err, string(output))
	}
	return nil
}

func (c *Cluster) mkdirOnNode(ctx context.Context, node *node, dir string) error {
	_, stderr, err := c.runCommandOnNode(ctx, node, []string{"mkdir", "-p", dir}, nil)
	if err != nil {
		return fmt.Errorf("err: %v, stderr: %s", err, stderr)
	}
	return nil
}

func sshConnectionArgs(node *node) []string {
	if sshUser := os.Getenv("REPLICATEDVM_SSH_USER"); sshUser != "" {
		// If ssh user is provided, we can make a direct ssh connection
		return []string{fmt.Sprintf("%s@%s", sshUser, node.DirectSSHEndpoint), "-p", strconv.Itoa(node.DirectSSHPort), "-o", "StrictHostKeyChecking=no"}
	}

	sshDomain := os.Getenv("REPLICATEDVM_SSH_DOMAIN")
	if sshDomain == "" {
		sshDomain = "replicatedcluster.com"
	}
	return []string{fmt.Sprintf("%s@%s", node.ID, sshDomain), "-o", "StrictHostKeyChecking=no"}
}

// SetupPlaywright installs necessary dependencies for Playwright testing
func (c *Cluster) SetupPlaywright(ctx context.Context, envs ...map[string]string) error {
	c.t.Logf("Setting up Playwright")

	// Install Node.js and other dependencies
	setupCommands := [][]string{
		{"curl", "-fsSL", "https://deb.nodesource.com/setup_16.x", "|", "sudo", "-E", "bash", "-"},
		{"sudo", "apt-get", "install", "-y", "nodejs"},
		{"npm", "install", "-g", "playwright"},
		{"npx", "playwright", "install"},
	}

	for _, cmd := range setupCommands {
		_, stderr, err := c.RunCommandOnNode(ctx, 0, cmd, envs...)
		if err != nil {
			return fmt.Errorf("run command %q on node %s: %v, stderr: %s", strings.Join(cmd, " "), c.nodes[0].ID, err, stderr)
		}
	}

	return nil
}

// SetupPlaywrightAndRunTest combines setup and test execution
func (c *Cluster) SetupPlaywrightAndRunTest(ctx context.Context, testName string, args ...string) (string, string, error) {
	if err := c.SetupPlaywright(ctx); err != nil {
		return "", "", err
	}
	return c.RunPlaywrightTest(ctx, testName, args...)
}

// RunPlaywrightTest executes a Playwright test
func (c *Cluster) RunPlaywrightTest(ctx context.Context, testName string, args ...string) (string, string, error) {
	c.t.Logf("Running Playwright test %s", testName)

	// Construct the test command
	testCmd := append([]string{"npx", "playwright", "test", testName}, args...)
	return c.RunCommandOnNode(ctx, 0, testCmd)
}
