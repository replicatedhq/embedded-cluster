package lxd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type buffer struct {
	*bytes.Buffer
}

func (b *buffer) Close() error {
	return nil
}

type RunCommandOption func(cmd *Command)

func WithECShelEnv() RunCommandOption {
	return func(cmd *Command) {
		cmd.Env = map[string]string{
			"EMBEDDED_CLUSTER_METRICS_BASEURL": "https://staging.replicated.app",
			"KUBECONFIG":                       "/var/lib/k0s/pki/admin.conf",
			"PATH":                             "/var/lib/embedded-cluster/bin",
		}
	}
}

func WithEnv(env map[string]string) RunCommandOption {
	return func(cmd *Command) {
		cmd.Env = env
	}
}

// RunCommandsOnNode runs a series of commands on a node.
func (c *Cluster) RunCommandsOnNode(t *testing.T, node int, cmds [][]string, opts ...RunCommandOption) error {
	for _, cmd := range cmds {
		cmdstr := strings.Join(cmd, " ")
		t.Logf("running `%s` node %d", cmdstr, node)
		_, _, err := c.RunCommandOnNode(t, node, cmd, opts...)
		if err != nil {
			return err
		}
	}
	return nil
}

// RunRegularUserCommandOnNode runs a command on a node as a regular user (not root) with a timeout.
func (c *Cluster) RunRegularUserCommandOnNode(t *testing.T, node int, line []string, opts ...RunCommandOption) (string, string, error) {
	stdout := &buffer{bytes.NewBuffer(nil)}
	stderr := &buffer{bytes.NewBuffer(nil)}
	cmd := &Command{
		Node:        c.Nodes[node],
		Line:        line,
		Stdout:      stdout,
		Stderr:      stderr,
		RegularUser: true,
	}
	for _, fn := range opts {
		fn(cmd)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := Run(ctx, t, *cmd); err != nil {
		t.Logf("stdout:\n%s\nstderr:%s\n", stdout.String(), stderr.String())
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

// RunCommandOnNode runs a command on a node with a timeout.
func (c *Cluster) RunCommandOnNode(t *testing.T, node int, line []string, opts ...RunCommandOption) (string, string, error) {
	stdout := &buffer{bytes.NewBuffer(nil)}
	stderr := &buffer{bytes.NewBuffer(nil)}
	cmd := &Command{
		Node:   c.Nodes[node],
		Line:   line,
		Stdout: stdout,
		Stderr: stderr,
	}
	for _, fn := range opts {
		fn(cmd)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := Run(ctx, t, *cmd); err != nil {
		t.Logf("stdout:\n%s", stdout.String())
		t.Logf("stderr:\n%s", stderr.String())
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

// RunCommandOnProxyNode runs a command on the proxy node with a timeout.
func (c *Cluster) RunCommandOnProxyNode(t *testing.T, line []string, opts ...RunCommandOption) (string, string, error) {
	if c.Proxy == "" {
		return "", "", fmt.Errorf("no proxy node found")
	}

	stdout := &buffer{bytes.NewBuffer(nil)}
	stderr := &buffer{bytes.NewBuffer(nil)}
	cmd := &Command{
		Node:   c.Proxy,
		Line:   line,
		Stdout: stdout,
		Stderr: stderr,
	}
	for _, fn := range opts {
		fn(cmd)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	if err := Run(ctx, t, *cmd); err != nil {
		t.Logf("stdout:\n%s", stdout.String())
		t.Logf("stderr:\n%s", stderr.String())
		return stdout.String(), stderr.String(), err
	}
	return stdout.String(), stderr.String(), nil
}

func (c *Cluster) InstallTestDependenciesDebian(t *testing.T, node int, withProxy bool) {
	t.Helper()
	t.Logf("%s: installing test dependencies on node %s", time.Now().Format(time.RFC3339), c.Nodes[node])
	commands := [][]string{
		{"apt-get", "update", "-y"},
		{"apt-get", "install", "curl", "expect", "-y"},
	}
	var opts []RunCommandOption
	if withProxy {
		opts = append(opts, WithEnv(map[string]string{
			"http_proxy":  HTTPProxy,
			"https_proxy": HTTPProxy,
		}))
	}
	if err := c.RunCommandsOnNode(t, node, commands, opts...); err != nil {
		t.Fatalf("fail to install test dependencies on node %s: %v", c.Nodes[node], err)
	}
}

func WithMITMProxyEnv(nodeIPs []string) RunCommandOption {
	return WithEnv(map[string]string{
		"HTTP_PROXY":  HTTPMITMProxy,
		"HTTPS_PROXY": HTTPMITMProxy,
		"NO_PROXY":    strings.Join(nodeIPs, ","),
	})
}

func WithProxyEnv(nodeIPs []string) RunCommandOption {
	return WithEnv(map[string]string{
		"HTTP_PROXY":  HTTPProxy,
		"HTTPS_PROXY": HTTPProxy,
		"NO_PROXY":    strings.Join(nodeIPs, ","),
	})
}

func (c *Cluster) Cleanup(t *testing.T) {
	if t.Failed() {
		c.generateSupportBundle(t)
		c.copyPlaywrightReport(t)
	}
}

func (c *Cluster) SetupPlaywrightAndRunTest(t *testing.T, testName string, args ...string) (stdout, stderr string, err error) {
	if err := c.SetupPlaywright(t); err != nil {
		return "", "", fmt.Errorf("failed to setup playwright: %w", err)
	}
	return c.RunPlaywrightTest(t, testName, args...)
}

func (c *Cluster) SetupPlaywright(t *testing.T) error {
	t.Logf("%s: bypassing kurl-proxy on node 0", time.Now().Format(time.RFC3339))
	line := []string{"bypass-kurl-proxy.sh"}
	if _, stderr, err := c.RunCommandOnNode(t, 0, line); err != nil {
		return fmt.Errorf("fail to bypass kurl-proxy on node %s: %v: %s", c.Nodes[0], err, string(stderr))
	}
	line = []string{"install-playwright.sh"}
	t.Logf("%s: installing playwright on proxy node", time.Now().Format(time.RFC3339))
	if _, stderr, err := c.RunCommandOnProxyNode(t, line); err != nil {
		return fmt.Errorf("fail to install playwright on node %s: %v: %s", c.Proxy, err, string(stderr))
	}
	return nil
}

func (c *Cluster) RunPlaywrightTest(t *testing.T, testName string, args ...string) (stdout, stderr string, err error) {
	t.Logf("%s: running playwright test %s on proxy node", time.Now().Format(time.RFC3339), testName)
	line := []string{"playwright.sh", testName}
	line = append(line, args...)
	stdout, stderr, err = c.RunCommandOnProxyNode(t, line)
	if err != nil {
		return stdout, stderr, fmt.Errorf("fail to run playwright test %s on node %s: %v", testName, c.Proxy, err)
	}
	return stdout, stderr, nil
}

func (c *Cluster) generateSupportBundle(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(len(c.Nodes))

	for i := range c.Nodes {
		go func(i int, wg *sync.WaitGroup) {
			defer wg.Done()
			t.Logf("%s: generating host support bundle from node %s", time.Now().Format(time.RFC3339), c.Nodes[i])
			line := []string{"collect-support-bundle-host.sh"}
			if stdout, stderr, err := c.RunCommandOnNode(t, i, line); err != nil {
				t.Logf("stdout: %s", stdout)
				t.Logf("stderr: %s", stderr)
				t.Logf("fail to generate support bundle from node %s: %v", c.Nodes[i], err)
				return
			}

			t.Logf("%s: copying host support bundle from node %s to local machine", time.Now().Format(time.RFC3339), c.Nodes[i])
			if err := CopyFileFromNode(c.Nodes[i], "/root/host.tar.gz", fmt.Sprintf("support-bundle-host-%s.tar.gz", c.Nodes[i])); err != nil {
				t.Logf("fail to copy host support bundle from node %s to local machine: %v", c.Nodes[i], err)
			}
		}(i, &wg)
	}

	node := c.Nodes[0]
	t.Logf("%s: generating cluster support bundle from node %s", time.Now().Format(time.RFC3339), node)
	line := []string{"collect-support-bundle-cluster.sh"}
	if stdout, stderr, err := c.RunCommandOnNode(t, 0, line); err != nil {
		t.Logf("stdout: %s", stdout)
		t.Logf("stderr: %s", stderr)
		t.Logf("fail to generate cluster support from node %s bundle: %v", node, err)
	} else {
		t.Logf("%s: copying cluster support bundle from node %s to local machine", time.Now().Format(time.RFC3339), node)
		if err := CopyFileFromNode(node, "/root/cluster.tar.gz", "support-bundle-cluster.tar.gz"); err != nil {
			t.Logf("fail to copy cluster support bundle from node %s to local machine: %v", node, err)
		}
	}

	wg.Wait()
}

func (c *Cluster) copyPlaywrightReport(t *testing.T) {
	line := []string{"tar", "-czf", "playwright-report.tar.gz", "-C", "/automation/playwright/playwright-report", "."}
	t.Logf("%s: compressing playwright report on proxy node", time.Now().Format(time.RFC3339))
	if _, _, err := c.RunCommandOnProxyNode(t, line); err != nil {
		t.Logf("fail to compress playwright report on node %s: %v", c.Proxy, err)
		return
	}
	t.Logf("%s: copying playwright report to local machine", time.Now().Format(time.RFC3339))
	if err := CopyFileFromNode(c.Proxy, "/root/playwright-report.tar.gz", "playwright-report.tar.gz"); err != nil {
		t.Logf("fail to copy playwright report to local machine: %v", err)
	}
}
