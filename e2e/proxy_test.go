package e2e

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster"
)

// TestCreateProxiedEnvironment doesn't do much at this stage, it only creates an environment
// in which the embedded cluster is running behind a proxy. This is going to become useful
// once we start to work in the proxy support but for now it is useful only for manual
// interactions (i.e. easier than setting up such an infrastructure manually).
func TestCreateProxiedEnvironment(t *testing.T) {
	t.Parallel()
	cluster.NewTestCluster(&cluster.Input{
		T:                   t,
		Nodes:               3,
		WithProxy:           true,
		Image:               "ubuntu/jammy",
		EmbeddedClusterPath: "../output/bin/embedded-cluster",
	})
	t.Log("Proxied infrastructure created")
}
