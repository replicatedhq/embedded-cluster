package cmx

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
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

	if val := os.Getenv("REPLICATEDVM_SSH_USER"); val == "" {
		input.T.Fatalf("REPLICATEDVM_SSH_USER is not set")
	}
	if val := os.Getenv("CMX_REPLICATED_API_TOKEN"); val == "" {
		input.T.Fatalf("CMX_REPLICATED_API_TOKEN is not set")
	}

	c := &Cluster{
		t:   input.T,
		gid: uuid.New().String(),
	}
	c.t.Cleanup(c.destroy)

	c.t.Logf("Creating network")
	network, err := createNetwork(c.t.Context(), c.gid, DefaultTTL)
	if err != nil {
		c.t.Fatalf("Failed to create network: %v", err)
	}
	c.networkID = network.ID

	eg := errgroup.Group{}

	eg.Go(func() error {
		c.t.Logf("Creating %d node(s)", input.Nodes)
		start := time.Now()
		nodes, err := createNodes(c.t.Context(), c.gid, c.networkID, DefaultTTL, clusterInputToCreateNodeOpts(input))
		if err != nil {
			return fmt.Errorf("create nodes: %v", err)
		}
		c.nodes = nodes
		c.t.Logf("-> Created %d nodes in %s", len(nodes), time.Since(start))
		return nil
	})

	eg.Go(func() error {
		c.t.Logf("Creating proxy node")
		start := time.Now()
		proxyNodes, err := createNodes(c.t.Context(), c.gid, c.networkID, DefaultTTL, createNodeOpts{
			Distribution: DefaultDistribution,
			Version:      DefaultVersion,
			Count:        1,
			InstanceType: "r1.small",
			DiskSize:     10,
		})
		if err != nil {
			return fmt.Errorf("create proxy node: %v", err)
		}
		c.proxyNode = proxyNodes[0]

		c.t.Logf("Enabling SSH access on proxy node")
		err = c.enableSSHAccessOnNode(c.proxyNode)
		if err != nil {
			return fmt.Errorf("enable ssh access on proxy node: %v", err)
		}

		c.t.Logf("Copying dirs to proxy node")
		err = c.copyDirsToNode(c.proxyNode)
		if err != nil {
			return fmt.Errorf("copy dirs to proxy node: %v", err)
		}

		c.t.Logf("-> Created proxy node in %s", time.Since(start))
		return nil
	})

	err = eg.Wait()
	if err != nil {
		c.t.Fatalf("Failed to create nodes: %v", err)
	}

	if input.WithProxy {
		c.t.Logf("Configuring proxy")
		// TODO: ConfigureProxy
	}

	eg = errgroup.Group{}

	for _, node := range c.nodes {
		eg.Go(func() error {
			start := time.Now()

			c.t.Logf("Enabling SSH access on node %s", node.ID)
			err := c.enableSSHAccessOnNode(node)
			if err != nil {
				return fmt.Errorf("enable ssh access: %v", err)
			}

			c.t.Logf("Copying files to node %s", node.ID)
			err = c.copyFilesToNode(node, input)
			if err != nil {
				return fmt.Errorf("copy files to node %s: %v", node.ID, err)
			}

			c.t.Logf("Copying dirs to node %s", node.ID)
			err = c.copyDirsToNode(node)
			if err != nil {
				return fmt.Errorf("copy dirs to node %s: %v", node.ID, err)
			}

			c.t.Logf("Installing dependencies on node %s", node.ID)
			_, stderr, err := c.runCommandOnNode(node, "root", []string{"/automation/scripts/install-deps.sh"})
			if err != nil {
				return fmt.Errorf("install dependencies on node %s: %v, stderr: %s", node.ID, err, stderr)
			}

			c.t.Logf("-> Initialized node %s in %s", node.ID, time.Since(start))
			return nil
		})
	}

	err = eg.Wait()
	if err != nil {
		c.t.Fatalf("Failed to copy files and dirs to nodes: %v", err)
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

// RunCommandOnNode executes a command on the specified node as the root user
func (c *Cluster) RunCommandOnNode(node int, command []string, envs ...map[string]string) (string, string, error) {
	start := time.Now()
	c.t.Logf("Running command on node %s: %s", c.nodes[node].ID, strings.Join(command, " "))
	stdout, stderr, err := c.runCommandOnNode(c.nodes[node], "root", command, envs...)
	if err != nil {
		return stdout, stderr, err
	}
	c.t.Logf("  -> Command on node %s completed in %s", c.nodes[node].ID, time.Since(start))
	return "", "", nil
}

// RunCommandOnProxyNode executes a command on the proxy node as the root user
func (c *Cluster) RunCommandOnProxyNode(command []string, envs ...map[string]string) (string, string, error) {
	start := time.Now()
	c.t.Logf("Running command on proxy node: %s", strings.Join(command, " "))
	stdout, stderr, err := c.runCommandOnNode(c.proxyNode, "root", command, envs...)
	if err != nil {
		return stdout, stderr, err
	}
	c.t.Logf("  -> Command on proxy node completed in %s", time.Since(start))
	return stdout, stderr, nil
}

// RunRegularUserCommandOnNode executes a command on the specified node as a non-root user
func (c *Cluster) RunRegularUserCommandOnNode(node int, command []string, envs ...map[string]string) (string, string, error) {
	start := time.Now()
	c.t.Logf("Running command on node %s as user %s: %s", c.nodes[node].ID, os.Getenv("REPLICATEDVM_SSH_USER"), strings.Join(command, " "))
	stdout, stderr, err := c.runCommandOnNode(c.nodes[node], os.Getenv("REPLICATEDVM_SSH_USER"), command, envs...)
	if err != nil {
		return stdout, stderr, err
	}
	c.t.Logf("  -> Command on node %s completed in %s", c.nodes[node].ID, time.Since(start))
	return stdout, stderr, nil
}

// Cleanup removes the VM instance
func (c *Cluster) Cleanup(envs ...map[string]string) {
	// TODO: generate support bundle and copy playwright report
	c.destroy()
}

// CopyFileToNode copies a file to a node
func (c *Cluster) CopyFileToNode(node int, src, dest string) error {
	return c.copyFileToNode(c.nodes[node], src, dest)
}

// CopyDirToNode copies a directory to a node
func (c *Cluster) CopyDirToNode(node int, src, dest string) error {
	return c.copyDirToNode(c.nodes[node], src, dest)
}

// SetupPlaywright installs necessary dependencies for Playwright testing
func (c *Cluster) SetupPlaywright(envs ...map[string]string) error {
	c.t.Logf("Setting up Playwright")

	line := []string{"/automation/scripts/bypass-kurl-proxy.sh"}
	if _, stderr, err := c.RunCommandOnNode(0, line, envs...); err != nil {
		return fmt.Errorf("bypass kurl-proxy on proxy node: %v: %s", err, string(stderr))
	}
	line = []string{"/automation/scripts/install-playwright.sh"}
	if _, stderr, err := c.RunCommandOnProxyNode(line); err != nil {
		return fmt.Errorf("install playwright on proxy node: %v: %s", err, string(stderr))
	}
	return nil
}

// SetupPlaywrightAndRunTest combines setup and test execution
func (c *Cluster) SetupPlaywrightAndRunTest(testName string, args ...string) (string, string, error) {
	if err := c.SetupPlaywright(); err != nil {
		return "", "", err
	}
	return c.RunPlaywrightTest(testName, args...)
}

// RunPlaywrightTest executes a Playwright test
func (c *Cluster) RunPlaywrightTest(testName string, args ...string) (string, string, error) {
	c.t.Logf("Running Playwright test %s", testName)

	line := []string{"/automation/scripts/playwright.sh", testName}
	line = append(line, args...)
	env := map[string]string{
		"BASE_URL": fmt.Sprintf("http://%s", net.JoinHostPort("TODO", "30003")), // TODO: get ip and expose port
	}
	stdout, stderr, err := c.RunCommandOnProxyNode(line, env)
	if err != nil {
		return stdout, stderr, fmt.Errorf("run playwright test %s on proxy node: %v", testName, err)
	}
	return stdout, stderr, nil
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

func (c *Cluster) runCommandOnNode(node *node, sshUser string, command []string, envs ...map[string]string) (string, string, error) {
	args := []string{}
	args = append(args, sshConnectionArgs(node, sshUser, false)...)
	args = append(args, fmt.Sprintf("sh -c '%s'", strings.Join(command, " ")))
	c.t.Logf("  -> Running ssh command on node %s: %q", node.ID, args)
	cmd := exec.CommandContext(c.t.Context(), "ssh", args...)

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

func (c *Cluster) enableSSHAccessOnNode(node *node) error {
	c.t.Logf("Enabling SSH access with root user on node %s", node.ID)
	command := []string{
		"sudo", "mkdir", "-p", "/root/.ssh",
		"&&", "sudo", "cp", "-f", "$HOME/.ssh/authorized_keys", "/root/.ssh/authorized_keys",
	}
	_, stderr, err := c.runCommandOnNode(node, os.Getenv("REPLICATEDVM_SSH_USER"), command)
	if err != nil {
		return fmt.Errorf("enable SSH access with root user: %v, stderr: %s", err, stderr)
	}
	return nil
}

func (c *Cluster) copyFilesToNode(node *node, in ClusterInput) error {
	files := map[string]string{
		in.LicensePath:             "/assets/license.yaml",            //0644
		in.EmbeddedClusterPath:     "/usr/local/bin/embedded-cluster", //0755
		in.AirgapInstallBundlePath: "/assets/ec-release.tgz",          //0755
		in.AirgapUpgradeBundlePath: "/assets/ec-release-upgrade.tgz",  //0755
	}
	for src, dest := range files {
		if src != "" {
			err := c.copyFileToNode(node, src, dest)
			if err != nil {
				return fmt.Errorf("copy file %s to node %s at %s: %v", src, node.ID, dest, err)
			}
		}
	}
	return nil
}

func (c *Cluster) copyDirsToNode(node *node) error {
	dirs := map[string]string{
		"scripts":    "/automation/scripts",
		"playwright": "/automation/playwright",
		"../operator/charts/embedded-cluster-operator/troubleshoot": "/automation/troubleshoot",
	}
	for src, dest := range dirs {
		err := c.copyDirToNode(node, src, dest)
		if err != nil {
			return fmt.Errorf("copy dir %s to node %s at %s: %v", src, node.ID, dest, err)
		}
	}
	return nil
}

func (c *Cluster) copyFileToNode(node *node, src, dest string) error {
	start := time.Now()
	c.t.Logf("Copying file %s to node %s at %s", src, node.ID, dest)

	_, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %v", src, err)
	}

	err = c.mkdirOnNode(node, filepath.Dir(dest))
	if err != nil {
		return fmt.Errorf("mkdir %s on node %s: %v", filepath.Dir(dest), node.ID, err)
	}

	args := []string{}
	args = append(args, sshConnectionArgs(node, "root", true)...)
	args[len(args)-1] = fmt.Sprintf("%s:%s", args[len(args)-1], dest)
	args = append(args[0:len(args)-1], "-p", src, args[len(args)-1])

	c.t.Logf("  -> Running scp command on node %s: %q", node.ID, args)
	scpCmd := exec.CommandContext(c.t.Context(), "scp", args...)
	output, err := scpCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("err: %v, output: %s", err, string(output))
	}

	c.t.Logf("  -> Copied file %s to node %s in %s", src, node.ID, time.Since(start))
	return nil
}

func (c *Cluster) copyDirToNode(node *node, src, dest string) error {
	start := time.Now()
	c.t.Logf("Copying dir %s to node %s at %s", src, node.ID, dest)

	_, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %v", src, err)
	}

	srcTar, err := tmpFileName("*.tar.gz")
	if err != nil {
		return fmt.Errorf("create temp file: %v", err)
	}

	err = tgzDir(c.t.Context(), src, srcTar)
	if err != nil {
		return fmt.Errorf("tgz dir %s: %v", src, err)
	}
	defer os.Remove(srcTar)

	archiveDst := filepath.Join(filepath.Dir(dest), srcTar)
	err = c.copyFileToNode(node, srcTar, archiveDst)
	if err != nil {
		return fmt.Errorf("copy file %s to node %s at %s: %v", srcTar, node.ID, archiveDst, err)
	}

	envs := map[string]string{
		"COPYFILE_DISABLE": "true", // disable metadata files on macOS
	}
	_, stderr, err := c.runCommandOnNode(node, "root", []string{"tar", "-xzf", archiveDst, "-C", filepath.Dir(dest)}, envs)
	if err != nil {
		return fmt.Errorf("run command: %v, stderr: %s", err, stderr)
	}

	c.t.Logf("  -> Copied dir %s to node %s in %s", src, node.ID, time.Since(start))
	return nil
}

func (c *Cluster) mkdirOnNode(node *node, dir string) error {
	_, stderr, err := c.runCommandOnNode(node, "root", []string{"mkdir", "-p", dir}, nil)
	if err != nil {
		return fmt.Errorf("err: %v, stderr: %s", err, stderr)
	}
	return nil
}

func sshConnectionArgs(node *node, sshUser string, isSCP bool) []string {
	args := []string{"-o", "StrictHostKeyChecking=no"}

	// If ssh user is provided, we can make a direct ssh connection
	if sshKey := os.Getenv("REPLICATEDVM_SSH_KEY"); sshKey != "" {
		args = append(args, "-i", sshKey)
	}
	if isSCP {
		args = append(args, "-P", strconv.Itoa(node.DirectSSHPort))
	} else {
		args = append(args, "-p", strconv.Itoa(node.DirectSSHPort))
	}
	args = append(args, fmt.Sprintf("%s@%s", sshUser, node.DirectSSHEndpoint))
	return args
}
