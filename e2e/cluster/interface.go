package cluster

import (
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/cmx"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
	"github.com/replicatedhq/embedded-cluster/e2e/cluster/lxd"
)

var (
	_ Cluster = (*lxd.Cluster)(nil)
	_ Cluster = (*docker.Cluster)(nil)
	_ Cluster = (*cmx.Cluster)(nil)
)

type Cluster interface {
	Cleanup(envs ...map[string]string)

	RunCommandOnNode(node int, line []string, envs ...map[string]string) (string, string, error)

	SetupPlaywrightAndRunTest(testName string, args ...string) (string, string, error)
	SetupPlaywright(envs ...map[string]string) error
	RunPlaywrightTest(testName string, args ...string) (string, string, error)
}
