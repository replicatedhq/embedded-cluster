package e2e

import (
	"os"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/e2e/cluster/docker"
	"github.com/stretchr/testify/assert"
)

func TestCollectSupportBundle(t *testing.T) {
	t.Parallel()

	RequireEnvVars(t, []string{
		"APP_INSTALL_VERSION",
		"EC_BINARY_PATH",
	})

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "licenses/license.yaml",
		ECBinaryPath: os.Getenv("EC_BINARY_PATH"),
	})
	defer tc.Cleanup()

	t.Logf("%s: installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"single-node-install.sh", "cli", os.Getenv("APP_INSTALL_VERSION")}
	stdout, stderr, err := tc.RunCommandOnNode(0, line)
	assert.NoErrorf(t, err, "fail to install embedded-cluster: %v: %s: %s", err, stdout, stderr)

	line = []string{"collect-support-bundle-host.sh"}
	stdout, stderr, err = tc.RunCommandOnNode(0, line)
	assert.NoErrorf(t, err, "fail to collect host support bundle: %v: %s: %s", err, stdout, stderr)

	line = []string{"collect-support-bundle-cluster.sh"}
	stdout, stderr, err = tc.RunCommandOnNode(0, line)
	assert.NoErrorf(t, err, "fail to collect cluster support bundle: %v: %s: %s", err, stdout, stderr)

	t.Logf("%s: collecting support bundle with the embedded-cluster binary", time.Now().Format(time.RFC3339))
	line = []string{"embedded-cluster", "support-bundle"}
	stdout, stderr, err = tc.RunCommandOnNode(0, line)
	assert.NoErrorf(t, err, "fail to collect support bundle using embedded-cluster binary: %v: %s: %s", err, stdout, stderr)

	line = []string{"validate-support-bundle.sh"}
	stdout, stderr, err = tc.RunCommandOnNode(0, line)
	assert.NoErrorf(t, err, "fail to validate support bundle: %v: %s: %s", err, stdout, stderr)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
