package cluster

import (
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/lxd"
)

var (
	_ Cluster = (*lxd.Cluster)(nil)
	_ Cluster = (*docker.Cluster)(nil)
)

type Cluster interface {
	Cleanup()

	RunCommandOnNode(node int, line []string, envs ...map[string]string) (string, string, error)

	SetupPlaywrightAndRunTest(testName string, args ...string) (stdout, stderr string, err error)
	SetupPlaywright() error
	RunPlaywrightTest(testName string, args ...string) (stdout, stderr string, err error)
}
