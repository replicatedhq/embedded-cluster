package integration

import (
	"context"
	"testing"
	"time"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/cluster/docker"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	dryruntypes "github.com/replicatedhq/embedded-cluster/pkg/dryrun/types"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
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
	defer tc.Destroy()

	t.Logf("%s: dryrun installing embedded-cluster on node 0", time.Now().Format(time.RFC3339))
	line := []string{"dryrun-install.sh", "--local-artifact-mirror-port", "50001", "--admin-console-port", "30002"}
	stdout, stderr, err := tc.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to dryrun install embedded-cluster: %v: %s: %s", err, stdout, stderr)
	}

	line = []string{"cat", "ec-dryrun.yaml"}
	stdout, stderr, err = tc.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to get dryrun output: %v: %s: %s", err, stdout, stderr)
	}

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
	foundInstallationSucceeded := false
	for _, m := range dr.Metrics {
		if m.Title == "InstallationStarted" {
			foundInstallationStarted = true
			assert.Contains(t, m.Payload, "--local-artifact-mirror-port 50001")
			assert.Contains(t, m.Payload, "--admin-console-port 30002")
		}
		if m.Title == "InstallationSucceeded" {
			foundInstallationSucceeded = true
		}
	}
	if !foundInstallationStarted {
		t.Errorf("InstallationStarted metric not found")
	}
	if !foundInstallationSucceeded {
		t.Errorf("InstallationSucceeded metric not found")
	}

	// --- validate cluster resources exist --- //
	kcli, err := dr.KubeClient()
	if err != nil {
		t.Fatalf("failed to create kube client: %v", err)
	}

	assert.True(t, configMapExists(t, kcli, "embedded-cluster-host-support-bundle", "kotsadm"))
	assert.True(t, secretExists(t, kcli, "kotsadm-password", "kotsadm"))
	assert.True(t, secretExists(t, kcli, "cloud-credentials", "velero"))

	// --- validate installation object --- //
	in, err := kubeutils.GetLatestInstallation(context.TODO(), kcli)
	if err != nil {
		t.Fatalf("failed to get latest installation: %v", err)
	}

	assert.Equal(t, "80-32767", in.Spec.Network.NodePortRange)
	assert.Equal(t, "10.244.0.0/16", dr.Flags["cidr"])
	assert.Equal(t, "10.244.0.0/17", in.Spec.Network.PodCIDR)
	assert.Equal(t, "10.244.128.0/17", in.Spec.Network.ServiceCIDR)
	assert.Equal(t, 30002, in.Spec.RuntimeConfig.AdminConsole.Port)
	assert.Equal(t, "/var/lib/embedded-cluster", in.Spec.RuntimeConfig.DataDir)
	assert.Equal(t, 50001, in.Spec.RuntimeConfig.LocalArtifactMirror.Port)
	assert.Equal(t, "ec-install", in.ObjectMeta.Labels["replicated.com/disaster-recovery"])

	// --- validate k0s cluster config --- //
	line = []string{"cat", defaults.PathToK0sConfig()}
	stdout, stderr, err = tc.RunCommandOnNode(0, line)
	if err != nil {
		t.Fatalf("fail to get k0s config: %v: %s: %s", err, stdout, stderr)
	}

	k0sConfig := k0sv1beta1.ClusterConfig{}
	if err := yaml.Unmarshal([]byte(stdout), &k0sConfig); err != nil {
		t.Fatalf("fail to unmarshal k0s config: %v", err)
	}

	assert.Equal(t, "10.244.0.0/17", k0sConfig.Spec.Network.PodCIDR)
	assert.Equal(t, "10.244.128.0/17", k0sConfig.Spec.Network.ServiceCIDR)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
