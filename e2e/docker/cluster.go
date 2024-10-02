package docker

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

type Cluster struct {
	Nodes []*Container
}

func NewTestCluster(t *testing.T, distro string, nodes int) *Cluster {
	c := &Cluster{}
	for i := 0; i < nodes; i++ {
		c.Nodes = append(c.Nodes, NewNode(t, distro))
	}
	c.WaitForSystemd()
	return c
}

func NewNode(t *testing.T, distro string) *Container {
	c := NewContainer(t).
		WithImage(fmt.Sprintf("replicated/ec-distro:%s", distro)).
		WithVolume("/var/lib/k0s").
		WithPort("30003:30003").
		WithScripts().
		WithECBinary()
	if licensePath := os.Getenv("LICENSE_PATH"); licensePath != "" {
		t.Logf("using license %s", licensePath)
		c = c.WithLicense(licensePath)
	}
	c.Start()
	return c
}

func (c *Cluster) WaitForSystemd() {
	for _, node := range c.Nodes {
		timeout := time.After(15 * time.Second)
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
