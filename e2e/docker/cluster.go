package docker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

type Cluster struct {
	Nodes []*Container

	t *testing.T
}

type ClusterInput struct {
	T            *testing.T
	Nodes        int
	Distro       string
	LicensePath  string
	ECBinaryPath string
}

func NewCluster(in *ClusterInput) *Cluster {
	c := &Cluster{t: in.T}
	for i := 0; i < in.Nodes; i++ {
		c.Nodes = append(c.Nodes, NewNode(in))
	}
	c.WaitForReady()
	return c
}

func NewNode(in *ClusterInput) *Container {
	c := NewContainer(in.T).
		WithImage(fmt.Sprintf("replicated/ec-distro:%s", in.Distro)).
		WithVolume("/var/lib/k0s").
		WithPort("30003:30003").
		WithScripts()
	if in.ECBinaryPath != "" {
		in.T.Logf("using embedded cluster binary %s", in.ECBinaryPath)
		c = c.WithECBinary(in.ECBinaryPath)
	}
	if in.LicensePath != "" {
		in.T.Logf("using license %s", in.LicensePath)
		c = c.WithLicense(in.LicensePath)
	}
	c.Run()
	return c
}

func (c *Cluster) WaitForReady() {
	for i, node := range c.Nodes {
		c.t.Logf("waiting for node %d to be ready", i)
		node.WaitForSystemd()
		node.WaitForClockSync()
		c.t.Logf("node %d is ready", i)
	}
}

func (c *Cluster) Cleanup() {
	if c.t.Failed() {
		c.generateSupportBundle()
		c.copyPlaywrightReport()
	}
	for _, node := range c.Nodes {
		node.Destroy()
	}
}

func (c *Cluster) SetupPlaywrightAndRunTest(testName string, args ...string) (stdout, stderr string, err error) {
	if err := c.SetupPlaywright(); err != nil {
		return "", "", fmt.Errorf("failed to setup playwright: %w", err)
	}
	return c.RunPlaywrightTest(testName, args...)
}

func (c *Cluster) SetupPlaywright() error {
	c.t.Logf("%s: bypassing kurl-proxy", time.Now().Format(time.RFC3339))
	_, stderr, err := c.Nodes[0].Exec("bypass-kurl-proxy.sh")
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
	cmd.Env = append(cmd.Env, "BASE_URL=http://localhost:30003")
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

func (c *Cluster) generateSupportBundle() {
	wg := sync.WaitGroup{}
	wg.Add(len(c.Nodes))

	for i := range c.Nodes {
		go func(i int, wg *sync.WaitGroup) {
			defer wg.Done()
			c.t.Logf("%s: generating host support bundle from node %d", time.Now().Format(time.RFC3339), i)
			if stdout, stderr, err := c.Nodes[i].Exec("collect-support-bundle-host.sh"); err != nil {
				c.t.Logf("stdout: %s", stdout)
				c.t.Logf("stderr: %s", stderr)
				c.t.Logf("fail to generate support from node %d bundle: %v", i, err)
				return
			}

			c.t.Logf("%s: copying host support bundle from node %d to local machine", time.Now().Format(time.RFC3339), i)
			src := fmt.Sprintf("%s:host.tar.gz", c.Nodes[i].GetID())
			dst := fmt.Sprintf("support-bundle-host-%d.tar.gz", i)
			if stdout, stderr, err := c.Nodes[i].CopyFile(src, dst); err != nil {
				c.t.Logf("stdout: %s", stdout)
				c.t.Logf("stderr: %s", stderr)
				c.t.Logf("fail to generate support bundle from node %d: %v", i, err)
				return
			}
		}(i, &wg)
	}

	c.t.Logf("%s: generating cluster support bundle from node 0", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := c.Nodes[0].Exec("collect-support-bundle-cluster.sh"); err != nil {
		c.t.Logf("stdout: %s", stdout)
		c.t.Logf("stderr: %s", stderr)
		c.t.Logf("fail to generate cluster support from node %d bundle: %v", 0, err)
	} else {
		c.t.Logf("%s: copying cluster support bundle from node 0 to local machine", time.Now().Format(time.RFC3339))
		src := fmt.Sprintf("%s:cluster.tar.gz", c.Nodes[0].GetID())
		dst := "support-bundle-cluster.tar.gz"
		if stdout, stderr, err := c.Nodes[0].CopyFile(src, dst); err != nil {
			c.t.Logf("stdout: %s", stdout)
			c.t.Logf("stderr: %s", stderr)
			c.t.Logf("fail to generate cluster support bundle from node 0: %v", err)
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
