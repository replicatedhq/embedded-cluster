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
	ID        string `json:"id"`
	Name      string `json:"name"`
	NetworkID string `json:"network_id"`
	Status    string `json:"status"`

	privateIP       string `json:"-"`
	sshEndpoint     string `json:"-"`
	adminConsoleURL string `json:"-"`
}

type Network struct {
	ID string `json:"id"`
}

type NetworkReport struct {
	Events []EventWrapper `json:"events"`
}

type EventWrapper struct {
	EventData string `json:"event_data"`
}

type NetworkEvent struct {
	Timestamp     time.Time `json:"timestamp"`
	PID           int       `json:"pid"`
	SrcIP         string    `json:"srcIp"`
	SrcPort       int       `json:"srcPort"`
	DstIP         string    `json:"dstIp"`
	DstPort       int       `json:"dstPort"`
	DNSQueryName  string    `json:"dnsQueryName"`
	LikelyService string    `json:"likelyService"`
	Command       string    `json:"comm"`
}

func NewCluster(in *ClusterInput) *Cluster {
	c := &Cluster{t: in.T, supportBundleNodeIndex: in.SupportBundleNodeIndex}
	c.t.Cleanup(c.Destroy)

	nodes, err := NewNodes(in)
	if err != nil {
		in.T.Fatalf("failed to create nodes: %v", err)
	}
	in.T.Logf("cluster created with network ID %s", nodes[0].NetworkID)
	c.Nodes = nodes
	c.network = &Network{ID: nodes[0].NetworkID}

	return c
}

func NewNodes(in *ClusterInput) ([]Node, error) {
	in.T.Logf("%s: creating %s nodes", time.Now().Format(time.RFC3339), strconv.Itoa(in.Nodes))

	args := []string{
		"vm", "create",
		"--name", "ec-test-suite",
		"--count", strconv.Itoa(in.Nodes),
		"--wait", "10m",
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
	if key := os.Getenv("CMX_SSH_PUBLIC_KEY"); key != "" {
		args = append(args, "--ssh-public-key", key)
	}

	output, err := exec.Command("replicated", args...).Output() // stderr can break json parsing
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("create nodes: %w: stderr: %s: stdout: %s", err, string(exitErr.Stderr), string(output))
		}
		return nil, fmt.Errorf("create nodes: %w: stdout: %s", err, string(output))
	}

	var nodes []Node
	if err := json.Unmarshal(output, &nodes); err != nil {
		return nil, fmt.Errorf("unmarshal node: %v: %s", err, string(output))
	}

	// TODO (@salah): remove this once the bug is fixed in CMX
	// note: the vm gets marked as ready before the services are actually running
	time.Sleep(30 * time.Second)

	for i := range nodes {
		in.T.Logf("%s: getting ssh endpoint for node ID: %s", time.Now().Format(time.RFC3339), nodes[i].ID)

		sshEndpoint, err := getSSHEndpoint(nodes[i].ID)
		if err != nil {
			return nil, fmt.Errorf("get ssh endpoint for node %s: %v", nodes[i].ID, err)
		}
		nodes[i].sshEndpoint = sshEndpoint

		if err := waitForSSH(nodes[i], in.T); err != nil {
			return nil, fmt.Errorf("wait for ssh to be available on node %d: %v", i, err)
		}

		privateIP, err := discoverPrivateIP(nodes[i])
		if err != nil {
			return nil, fmt.Errorf("discover node private IP: %v", err)
		}
		nodes[i].privateIP = privateIP

		if err := ensureAssetsDir(nodes[i]); err != nil {
			return nil, fmt.Errorf("ensure assets dir on node %s: %v", nodes[i].ID, err)
		}

		if err := copyScriptsToNode(nodes[i]); err != nil {
			return nil, fmt.Errorf("copy scripts to node %s: %v", nodes[i].ID, err)
		}

		if i == 0 {
			in.T.Logf("exposing port 30003 on node %s", nodes[i].ID)
			hostname, err := exposePort(nodes[i], "30003")
			if err != nil {
				return nil, fmt.Errorf("expose port: %v", err)
			}
			nodes[i].adminConsoleURL = fmt.Sprintf("http://%s", hostname)
		}

		in.T.Logf("node %d created with ID %s and private IP %s", i, nodes[i].ID, nodes[i].privateIP)
	}

	return nodes, nil
}

func discoverPrivateIP(node Node) (string, error) {
	stdout, stderr, err := runCommandOnNode(node, []string{"hostname", "-I"})
	if err != nil {
		return "", fmt.Errorf("get node IP: %v: %s: %s", err, stdout, stderr)
	}

	for _, ip := range strings.Fields(stdout) {
		if strings.HasPrefix(ip, "10.") {
			return ip, nil
		}
	}

	return "", fmt.Errorf("failed to find private ip starting with 10 dot")
}

func ensureAssetsDir(node Node) error {
	stdout, stderr, err := runCommandOnNode(node, []string{"mkdir", "-p", "/assets"})
	if err != nil {
		return fmt.Errorf("create directory: %v: %s: %s", err, stdout, stderr)
	}
	return nil
}

func copyScriptsToNode(node Node) error {
	// Create a temporary directory for the archive
	tempDir, err := os.MkdirTemp("", "scripts-archive")
	if err != nil {
		return fmt.Errorf("create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create the archive
	archivePath := filepath.Join(tempDir, "scripts.tgz")
	output, err := exec.Command("tar", "-czf", archivePath, "-C", "scripts", ".").CombinedOutput()
	if err != nil {
		return fmt.Errorf("create scripts archive: %v: %s", err, string(output))
	}

	// Copy the archive to the node
	if err := copyFileToNode(node, archivePath, "/tmp/scripts.tgz"); err != nil {
		return fmt.Errorf("copy scripts archive to node: %v", err)
	}

	// Extract the archive in /usr/local/bin
	_, stderr, err := runCommandOnNode(node, []string{"tar", "-xzf", "/tmp/scripts.tgz", "-C", "/usr/local/bin"})
	if err != nil {
		return fmt.Errorf("extract scripts archive: %v: %s", err, stderr)
	}

	// Clean up the archive on the node
	_, stderr, err = runCommandOnNode(node, []string{"rm", "-f", "/tmp/scripts.tgz"})
	if err != nil {
		return fmt.Errorf("clean up scripts archive: %v: %s", err, stderr)
	}

	return nil
}

func getSSHEndpoint(nodeID string) (string, error) {
	args := []string{
		"vm",
		"ssh-endpoint",
		nodeID,
	}
	if user := os.Getenv("CMX_SSH_USERNAME"); user != "" {
		args = append(args, "--username", user)
	}
	output, err := exec.Command("replicated", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

func waitForSSH(node Node, t *testing.T) error {
	timeout := time.After(5 * time.Minute)
	tick := time.Tick(5 * time.Second)
	var lastErr error

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out after 5 minutes: last error: %w", lastErr)
		case <-tick:
			t.Logf("%s: checking SSH connectivity to node ID: %s", time.Now().Format(time.RFC3339), node.ID)
			stdout, stderr, err := runCommandOnNode(node, []string{"uptime"})
			t.Logf("%s: SSH attempt - stdout: %s, stderr: %s, err: %v", time.Now().Format(time.RFC3339), stdout, stderr, err)
			if err == nil {
				t.Logf("%s: SSH connection successful to node ID: %s", time.Now().Format(time.RFC3339), node.ID)
				return nil
			}
			lastErr = fmt.Errorf("%w: stdout: %s: stderr: %s", err, stdout, stderr)
		}
	}
}

func (c *Cluster) Airgap() error {
	// Update network policy to airgap
	output, err := exec.Command("replicated", "network", "update", c.network.ID, "--policy=airgap").CombinedOutput()
	if err != nil {
		return fmt.Errorf("update network policy: %v: %s", err, string(output))
	}

	// Wait until the nodes are airgapped
	for node := range c.Nodes {
		if err := c.waitUntilAirgapped(node); err != nil {
			return fmt.Errorf("wait until node %d is airgapped: %v", node, err)
		}
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

func (c *Cluster) WaitForReboot() {
	time.Sleep(30 * time.Second)
	for i := range c.Nodes {
		c.waitForSSH(i)
		c.waitForClockSync(i)
	}
}

func (c *Cluster) waitForSSH(node int) {
	if err := waitForSSH(c.Nodes[node], c.t); err != nil {
		c.t.Fatalf("failed to wait for ssh to be available on node %d: %v", node, err)
	}
}

func (c *Cluster) waitForClockSync(node int) {
	timeout := time.After(5 * time.Minute)
	tick := time.Tick(time.Second)
	for {
		select {
		case <-timeout:
			stdout, stderr, err := c.RunCommandOnNode(node, []string{"timedatectl show -p NTP -p NTPSynchronized"})
			c.t.Fatalf("timeout waiting for clock sync on node %d: %v: %s: %s", node, err, stdout, stderr)
		case <-tick:
			status, _, _ := c.RunCommandOnNode(node, []string{"timedatectl show -p NTP -p NTPSynchronized"})
			if strings.Contains(status, "NTP=yes") && strings.Contains(status, "NTPSynchronized=yes") {
				return
			}
		}
	}
}

func (c *Cluster) Cleanup(envs ...map[string]string) {
	c.generateSupportBundle(envs...)
	c.copyPlaywrightReport()
	c.Destroy()
}

func (c *Cluster) Destroy() {
	if os.Getenv("SKIP_CMX_CLEANUP") != "" {
		c.t.Logf("Skipping CMX cleanup")
		return
	}

	for _, node := range c.Nodes {
		c.removeNode(node)
	}
}

func (c *Cluster) removeNode(node Node) {
	output, err := exec.Command("replicated", "vm", "rm", node.ID).CombinedOutput()
	if err != nil {
		c.t.Logf("failed to destroy node %s: %v: %s", node.ID, err, string(output))
	}
}

func (c *Cluster) RunCommandOnNode(node int, line []string, envs ...map[string]string) (string, string, error) {
	return runCommandOnNode(c.Nodes[node], line, envs...)
}

func runCommandOnNode(node Node, line []string, envs ...map[string]string) (string, string, error) {
	for _, env := range envs {
		for k, v := range env {
			line = append([]string{fmt.Sprintf("%s=%s", k, v)}, line...)
		}
	}
	line = append([]string{"sudo", "PATH=$PATH:/usr/local/bin"}, line...)

	cmd := exec.Command("ssh", append(sshArgs(), node.sshEndpoint, strings.Join(line, " "))...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if exitErr, ok := err.(*exec.ExitError); ok && (exitErr.ExitCode() == 255 || exitErr.ExitCode() == 143) {
		// check if this is a reset-installation command that resulted in exit code 255 or 143
		// as this is expected behavior when the node reboots and the ssh connection is lost
		if strings.Contains(strings.Join(line, " "), "reset-installation") {
			return stdout.String(), stderr.String(), nil
		}
	}

	return stdout.String(), stderr.String(), err
}

func (c *Cluster) SetupPlaywrightAndRunTest(testName string, args ...string) (string, string, error) {
	if err := c.SetupPlaywright(); err != nil {
		return "", "", fmt.Errorf("setup playwright: %w", err)
	}
	return c.RunPlaywrightTest(testName, args...)
}

func (c *Cluster) SetupPlaywright(envs ...map[string]string) error {
	c.t.Logf("%s: bypassing kurl-proxy", time.Now().Format(time.RFC3339))
	_, stderr, err := c.RunCommandOnNode(0, []string{"/usr/local/bin/bypass-kurl-proxy.sh"}, envs...)
	if err != nil {
		return fmt.Errorf("bypass kurl-proxy: %v: %s", err, string(stderr))
	}
	c.t.Logf("%s: installing playwright", time.Now().Format(time.RFC3339))
	output, err := exec.Command("sh", "-c", "cd playwright && npm ci && npx playwright install --with-deps").CombinedOutput()
	if err != nil {
		return fmt.Errorf("install playwright: %v: %s", err, string(output))
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
		return stdout.String(), stderr.String(), err
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
			if stdout, stderr, err := c.RunCommandOnNode(i, []string{"/usr/local/bin/collect-support-bundle-host.sh"}, envs...); err != nil {
				c.t.Logf("stdout: %s", stdout)
				c.t.Logf("stderr: %s", stderr)
				c.t.Logf("fail to generate support bundle from node %d: %v", i, err)
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
	if stdout, stderr, err := c.RunCommandOnNode(c.supportBundleNodeIndex, []string{"/usr/local/bin/collect-support-bundle-cluster.sh"}, envs...); err != nil {
		c.t.Logf("stdout: %s", stdout)
		c.t.Logf("stderr: %s", stderr)
		c.t.Logf("fail to generate cluster support bundle from node %d: %v", c.supportBundleNodeIndex, err)
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
	output, err := exec.Command("tar", "-czf", "playwright-report.tar.gz", "-C", "./playwright/playwright-report", ".").CombinedOutput()
	if err != nil {
		c.t.Logf("fail to compress playwright report: %v: %s", err, string(output))
	}
}

func exposePort(node Node, port string) (string, error) {
	output, err := exec.Command("replicated", "vm", "port", "expose", node.ID, "--port", port).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("expose port: %v: %s", err, string(output))
	}

	output, err = exec.Command("replicated", "vm", "port", "ls", node.ID, "-ojson").Output() // stderr can break json parsing
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("get port info: %w: stderr: %s: stdout: %s", err, string(exitErr.Stderr), string(output))
		}
		return "", fmt.Errorf("get port info: %w: stdout: %s", err, string(output))
	}

	var ports []struct {
		Hostname string `json:"hostname"`
	}
	if err := json.Unmarshal(output, &ports); err != nil {
		return "", fmt.Errorf("unmarshal port info: %v", err)
	}

	if len(ports) == 0 {
		return "", fmt.Errorf("no ports found for node %s", node.ID)
	}
	return ports[0].Hostname, nil
}

func copyFileToNode(node Node, src, dst string) error {
	scpEndpoint := strings.Replace(node.sshEndpoint, "ssh://", "scp://", 1)

	output, err := exec.Command("scp", append(sshArgs(), src, fmt.Sprintf("%s/%s", scpEndpoint, dst))...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("copy file to node: %v: %s", err, string(output))
	}
	return nil
}

func copyFileFromNode(node Node, src, dst string) error {
	scpEndpoint := strings.Replace(node.sshEndpoint, "ssh://", "scp://", 1)

	output, err := exec.Command("scp", append(sshArgs(), fmt.Sprintf("%s/%s", scpEndpoint, src), dst)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("copy file from node: %v: %s", err, string(output))
	}
	return nil
}

func sshArgs() []string {
	return []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "BatchMode=yes",
	}
}

func (c *Cluster) SetNetworkReport(enabled bool) error {
	// Update network reporting
	output, err := exec.Command("replicated", "network", "update", c.network.ID, fmt.Sprintf("--collect-report=%v", enabled)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("update network collect-report: %v: %s", err, string(output))
	}

	// Wait until the nodes are all back in running
	for nodeNum, node := range c.Nodes {
		if err := c.waitUntilRunning(node, nodeNum, 30*time.Second); err != nil {
			return fmt.Errorf("wait until node %d is airgapped: %v", nodeNum, err)
		}
	}

	return nil
}

func (c *Cluster) waitUntilRunning(node Node, nodeNum int, timeoutDuration time.Duration) error {
	timeout := time.After(timeoutDuration)
	tick := time.Tick(2 * time.Second)
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out after waiting %s for node to be in running state", timeoutDuration)
		case <-tick:
			output, err := exec.Command("replicated", "vm", "ls", "-ojson").Output()
			if err != nil {
				return fmt.Errorf("check node status: %v", err)
			}

			nodes := []Node{}
			if err := json.Unmarshal(output, &nodes); err != nil {
				return fmt.Errorf("unmarshal node info: %v", err)
			}

			for _, checkNode := range nodes {
				if checkNode.ID != node.ID {
					continue
				}

				if checkNode.Status == "running" {
					return nil
				}

				c.t.Logf("node %v is not running yet, %v", nodeNum, checkNode.Status)
			}
		}
	}
}

func (c *Cluster) CollectNetworkReport() ([]NetworkEvent, error) {
	output, err := exec.Command("replicated", "network", "report", fmt.Sprintf("--id=%v", c.network.ID), "-ojson").Output()
	if err != nil {
		return nil, fmt.Errorf("collect network report: %v", err)
	}

	// TODO: investigate CLI changes to make event_data a json object instead of a string

	type networkReport struct {
		Events []eventWrapper `json:"events"`
	}

	events := []eventWrapper{}
	if err := json.Unmarshal(output, &events); err != nil {
		return nil, fmt.Errorf("unmarshal network events: %v", err)
	}

	networkEvents := make([]NetworkEvent, 0, len(events))
	for _, e := range events {
		ne := NetworkEvent{}
		if err := json.Unmarshal([]byte(e.EventData), &ne); err != nil {
			return nil, fmt.Errorf("unmarshal network event data: %v", err)
		}

		networkEvents = append(networkEvents, ne)
	}

	return networkEvents, nil
}
