package e2e

import (
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
)

func TestUnsupportedOverrides(t *testing.T) {
	t.Parallel()

	tc := docker.NewCluster(&docker.ClusterInput{
		T:                    t,
		Nodes:                1,
		Distro:               "debian-bookworm",
		LicensePath:          "license.yaml",
		ECBinaryPath:         "../output/bin/embedded-cluster",
		ECReleaseBuilderPath: "../output/bin/embedded-cluster-release-builder",
	})
	defer tc.Cleanup()

	t.Logf("%s: installing embedded-cluster with unsupported overrides on node 0", time.Now().Format(time.RFC3339))
	line := []string{"unsupported-overrides.sh"}
	if stdout, stderr, err := tc.RunCommandOnNode(0, line); err != nil {
		t.Fatalf("fail to install embedded-cluster: %v: %s: %s", err, stdout, stderr)
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
