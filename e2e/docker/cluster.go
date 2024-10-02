package docker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

type Cluster struct {
	Nodes []*Container
}

type ClusterInput struct {
	Nodes  int
	Distro string
	T      *testing.T
}

func NewCluster(in *ClusterInput) *Cluster {
	c := &Cluster{}
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
		WithScripts().
		WithECBinary()
	if licensePath := os.Getenv("LICENSE_PATH"); licensePath != "" {
		in.T.Logf("using license %s", licensePath)
		c = c.WithLicense(licensePath)
	}
	c.Start()
	return c
}

func (c *Cluster) WaitForReady() {
	for _, node := range c.Nodes {
		timeout := time.After(30 * time.Second)
		tick := time.Tick(time.Second)
		for {
			select {
			case <-timeout:
				node.t.Fatalf("timeout waiting for systemd to start")
			case <-tick:
				status, stderr, err := node.Exec("systemctl is-system-running")
				node.t.Logf("systemd status: %s, err: %v, stderr: %s", status, err, stderr)
				if strings.TrimSpace(status) == "running" {
					goto NextNode
				}
			}
		}
	NextNode:
	}
}

func (c *Cluster) Cleanup(t *testing.T) {
	if t.Failed() {
		c.generateSupportBundle(t)
		c.copyPlaywrightReport(t)
	}
	for _, node := range c.Nodes {
		node.Destroy()
	}
}

func (c *Cluster) SetupPlaywrightAndRunTest(t *testing.T, testName string, args ...string) (stdout, stderr string, err error) {
	if err := c.SetupPlaywright(t); err != nil {
		return "", "", fmt.Errorf("failed to setup playwright: %w", err)
	}
	return c.RunPlaywrightTest(t, testName, args...)
}

func (c *Cluster) SetupPlaywright(t *testing.T) error {
	t.Logf("%s: bypassing kurl-proxy", time.Now().Format(time.RFC3339))
	_, stderr, err := c.Nodes[0].Exec("bypass-kurl-proxy.sh")
	if err != nil {
		return fmt.Errorf("fail to bypass kurl-proxy: %v: %s", err, string(stderr))
	}
	t.Logf("%s: installing playwright", time.Now().Format(time.RFC3339))
	cmd := exec.Command("sh", "-c", "cd playwright && npm ci && npx playwright install --with-deps")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fail to install playwright: %v: %s", err, string(out))
	}
	return nil
}

func (c *Cluster) RunPlaywrightTest(t *testing.T, testName string, args ...string) (string, string, error) {
	t.Logf("%s: running playwright test %s", time.Now().Format(time.RFC3339), testName)
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

func (c *Cluster) generateSupportBundle(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(len(c.Nodes))

	for i := range c.Nodes {
		go func(i int, wg *sync.WaitGroup) {
			defer wg.Done()
			t.Logf("%s: generating host support bundle from node %d", time.Now().Format(time.RFC3339), i)
			if stdout, stderr, err := c.Nodes[i].Exec("collect-support-bundle-host.sh"); err != nil {
				t.Logf("stdout: %s", stdout)
				t.Logf("stderr: %s", stderr)
				t.Logf("fail to generate support from node %d bundle: %v", i, err)
				return
			}

			t.Logf("%s: copying host support bundle from node %d to local machine", time.Now().Format(time.RFC3339), i)
			src := fmt.Sprintf("%s:host.tar.gz", c.Nodes[i].GetID())
			dst := fmt.Sprintf("support-bundle-host-%d.tar.gz", i)
			if stdout, stderr, err := c.Nodes[i].CopyFile(src, dst); err != nil {
				t.Logf("stdout: %s", stdout)
				t.Logf("stderr: %s", stderr)
				t.Logf("fail to generate support bundle from node %d: %v", i, err)
				return
			}
		}(i, &wg)
	}

	t.Logf("%s: generating cluster support bundle from node 0", time.Now().Format(time.RFC3339))
	if stdout, stderr, err := c.Nodes[0].Exec("collect-support-bundle-cluster.sh"); err != nil {
		t.Logf("stdout: %s", stdout)
		t.Logf("stderr: %s", stderr)
		t.Logf("fail to generate cluster support from node %d bundle: %v", 0, err)
	} else {
		t.Logf("%s: copying cluster support bundle from node 0 to local machine", time.Now().Format(time.RFC3339))
		src := fmt.Sprintf("%s:cluster.tar.gz", c.Nodes[0].GetID())
		dst := "support-bundle-cluster.tar.gz"
		if stdout, stderr, err := c.Nodes[0].CopyFile(src, dst); err != nil {
			t.Logf("stdout: %s", stdout)
			t.Logf("stderr: %s", stderr)
			t.Logf("fail to generate cluster support bundle from node 0: %v", err)
		}
	}

	wg.Wait()
}

func (c *Cluster) copyPlaywrightReport(t *testing.T) {
	t.Logf("%s: compressing playwright report", time.Now().Format(time.RFC3339))
	cmd := exec.Command("tar", "-czf", "playwright-report.tar.gz", "-C", "./playwright/playwright-report", ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("fail to compress playwright report: %v: %s", err, string(out))
	}
}
