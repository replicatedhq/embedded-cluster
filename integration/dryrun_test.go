package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/cluster/docker"
	dryruntypes "github.com/replicatedhq/embedded-cluster/pkg/dryrun/types"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	metricstypes "github.com/replicatedhq/embedded-cluster/pkg/metrics/types"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

func TestDryRunInstallation(t *testing.T) {
	t.Parallel()

	tc := docker.NewCluster(&docker.ClusterInput{
		T:            t,
		Nodes:        1,
		Distro:       "debian-bookworm",
		LicensePath:  "license.yaml",
		ECBinaryPath: "../output/bin/embedded-cluster",
	})
	defer tc.Cleanup()

	t.Logf("%s: dryrun installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"dryrun-install.sh", "--local-artifact-mirror-port", "50001", "--admin-console-port", "30002"}
	stdout, stderr, err := tc.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to install embedded-cluster: %v: %s: %s", err, stdout, stderr)
	}

	line = []string{"cat", "ec-dryrun.yaml"}
	stdout, stderr, err = tc.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to collect host support bundle: %v: %s: %s", err, stdout, stderr)
	}

	fmt.Println(stdout)

	dr := dryruntypes.DryRun{}
	if err := yaml.Unmarshal([]byte(stdout), &dr); err != nil {
		t.Fatalf("fail to unmarshal dryrun output: %v", err)
	}

	// --- validate os env --- //
	expectedOSEnv := map[string]string{
		"TMPDIR":     "/var/lib/embedded-cluster/tmp",
		"KUBECONFIG": "/var/lib/embedded-cluster/k0s/pki/admin.conf",
	}
	for expectedKey, expectedValue := range expectedOSEnv {
		assert.Equal(t, expectedValue, dr.OSEnv[expectedKey])
	}

	// --- validate host preflight spec --- //
	foundFSPCollector := false
	foundLAMPortCollector := false
	foundKotsadmPortCollector := false

	for _, hc := range dr.HostPreflightSpec.Collectors {
		if hc.FilesystemPerformance != nil {
			foundFSPCollector = true
			assert.Equal(t, "/var/lib/embedded-cluster/k0s/etcd", hc.FilesystemPerformance.Directory)
		}
		if hc.TCPPortStatus != nil {
			if hc.TCPPortStatus.CollectorName == "Local Artifact Mirror Port" {
				foundLAMPortCollector = true
				assert.Equal(t, 50001, hc.TCPPortStatus.Port)
			}
			if hc.TCPPortStatus.CollectorName == "Kotsadm Node Port" {
				foundKotsadmPortCollector = true
				assert.Equal(t, 30002, hc.TCPPortStatus.Port)
			}
		}
	}

	assert.True(t, foundFSPCollector, "FilesystemPerformance collector not found")
	assert.True(t, foundLAMPortCollector, "Local Artifact Mirror Port collector not found")
	assert.True(t, foundKotsadmPortCollector, "Kotsadm Node Port collector not found")

	// --- validate metrics --- //
	foundInstallationStarted := false
	for _, m := range dr.Metrics {
		if in, ok := m.(metricstypes.InstallationStarted); ok {
			foundInstallationStarted = true
			assert.Contains(t, "--local-artifact-mirror-port 50001", in.Flags)
			assert.Contains(t, "--admin-console-port 30002", in.Flags)
		}
	}
	if !foundInstallationStarted {
		t.Fatalf("InstallationStarted metric not found")
	}

	kcli, err := dr.KubeClient()
	if err != nil {
		t.Fatalf("failed to create kube client: %v", err)
	}

	// --- validate installation object --- //

	in, err := kubeutils.GetLatestInstallation(context.TODO(), kcli)
	if err != nil {
		t.Fatalf("failed to get latest installation: %v", err)
	}

	assert.Equal(t, "ec-install", in.ObjectMeta.Annotations["replicated.com/disaster-recovery"])

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
