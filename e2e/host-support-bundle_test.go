package e2e

import (
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
)

func TestHostCollectSupportBundleInCluster(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{"SHORT_SHA"})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	installSingleNodeWithOptions(t, tc, installOptions{
		viaCLI: true,
	})

	line := []string{"collect-support-bundle-host-in-cluster.sh"}
	stdout, stderr, err := tc.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to collect host support bundle: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
