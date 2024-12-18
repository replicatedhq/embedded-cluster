package dryrun

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestDefaultInstallation(t *testing.T) {
	dr := dryrunInstall(t)

	// --- validate os env --- //
	assertEnv(t, dr.OSEnv, map[string]string{
		"TMPDIR":     "/var/lib/embedded-cluster/tmp",
		"KUBECONFIG": "/var/lib/embedded-cluster/k0s/pki/admin.conf",
	})

	// --- validate commands --- //
	assertCommands(t, dr.Commands,
		[]interface{}{
			regexp.MustCompile(`k0s install controller .* --data-dir /var/lib/embedded-cluster/k0s`),
		},
		false,
	)

	// --- validate host preflight spec --- //
	assertCollectors(t, dr.HostPreflightSpec.Collectors, map[string]struct {
		match    func(*troubleshootv1beta2.HostCollect) bool
		validate func(*troubleshootv1beta2.HostCollect)
	}{
		"FilesystemPerformance": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.FilesystemPerformance != nil
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, "/var/lib/embedded-cluster/k0s/etcd", hc.FilesystemPerformance.Directory)
			},
		},
		"LAM TCPPortStatus": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.TCPPortStatus != nil && hc.TCPPortStatus.CollectorName == "Local Artifact Mirror Port"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, 50000, hc.TCPPortStatus.Port)
			},
		},
		"Kotsadm TCPPortStatus": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.TCPPortStatus != nil && hc.TCPPortStatus.CollectorName == "Kotsadm Node Port"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, 30000, hc.TCPPortStatus.Port)
			},
		},
	})

	// --- validate metrics --- //
	assertMetrics(t, dr.Metrics, []struct {
		title    string
		validate func(string)
	}{
		{
			title:    "InstallationStarted",
			validate: func(payload string) {},
		},
		{
			title:    "InstallationSucceeded",
			validate: func(payload string) {},
		},
	})

	// --- validate cluster resources --- //
	kcli, err := dr.KubeClient()
	if err != nil {
		t.Fatalf("failed to create kube client: %v", err)
	}

	assertConfigMapExists(t, kcli, "embedded-cluster-host-support-bundle", "kotsadm")
	assertSecretExists(t, kcli, "kotsadm-password", "kotsadm")
	assertSecretExists(t, kcli, "cloud-credentials", "velero")

	// --- validate installation object --- //
	in, err := kubeutils.GetLatestInstallation(context.TODO(), kcli)
	if err != nil {
		t.Fatalf("failed to get latest installation: %v", err)
	}

	assert.Equal(t, "80-32767", in.Spec.Network.NodePortRange)
	assert.Equal(t, "10.244.0.0/16", dr.Flags["cidr"])
	assert.Equal(t, "10.244.0.0/17", in.Spec.Network.PodCIDR)
	assert.Equal(t, "10.244.128.0/17", in.Spec.Network.ServiceCIDR)
	assert.Equal(t, 30000, in.Spec.RuntimeConfig.AdminConsole.Port)
	assert.Equal(t, "/var/lib/embedded-cluster", in.Spec.RuntimeConfig.DataDir)
	assert.Equal(t, 50000, in.Spec.RuntimeConfig.LocalArtifactMirror.Port)
	assert.Equal(t, "ec-install", in.ObjectMeta.Labels["replicated.com/disaster-recovery"])

	// --- validate k0s cluster config --- //
	k0sConfig := readK0sConfig(t)

	assert.Equal(t, "10.244.0.0/17", k0sConfig.Spec.Network.PodCIDR)
	assert.Equal(t, "10.244.128.0/17", k0sConfig.Spec.Network.ServiceCIDR)
	assert.Contains(t, k0sConfig.Spec.API.SANs, "kubernetes.default.svc.cluster.local")

	assertHelmValues(t, k0sConfig, "openebs", map[string]interface{}{
		"['localpv-provisioner'].localpv.basePath": "/var/lib/embedded-cluster/openebs-local",
	})
	assertHelmValues(t, k0sConfig, "velero", map[string]interface{}{
		"nodeAgent.podVolumePath": "/var/lib/embedded-cluster/k0s/kubelet/pods",
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestCustomDataDir(t *testing.T) {
	dr := dryrunInstall(t,
		"--data-dir", "/custom/data/dir",
	)

	// --- validate os env --- //
	assertEnv(t, dr.OSEnv, map[string]string{
		"TMPDIR":     "/custom/data/dir/tmp",
		"KUBECONFIG": "/custom/data/dir/k0s/pki/admin.conf",
	})

	// --- validate commands --- //
	assertCommands(t, dr.Commands,
		[]interface{}{
			regexp.MustCompile(`k0s install controller .* --data-dir /custom/data/dir/k0s`),
		},
		false,
	)

	// --- validate host preflight spec --- //
	assertCollectors(t, dr.HostPreflightSpec.Collectors, map[string]struct {
		match    func(*troubleshootv1beta2.HostCollect) bool
		validate func(*troubleshootv1beta2.HostCollect)
	}{
		"FilesystemPerformance": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.FilesystemPerformance != nil
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, "/custom/data/dir/k0s/etcd", hc.FilesystemPerformance.Directory)
			},
		},
	})

	// --- validate installation object --- //
	kcli, err := dr.KubeClient()
	if err != nil {
		t.Fatalf("failed to create kube client: %v", err)
	}
	in, err := kubeutils.GetLatestInstallation(context.TODO(), kcli)
	if err != nil {
		t.Fatalf("failed to get latest installation: %v", err)
	}
	assert.Equal(t, "/custom/data/dir", in.Spec.RuntimeConfig.DataDir)

	// --- validate k0s cluster config --- //
	k0sConfig := readK0sConfig(t)

	assertHelmValues(t, k0sConfig, "openebs", map[string]interface{}{
		"['localpv-provisioner'].localpv.basePath": "/custom/data/dir/openebs-local",
	})
	assertHelmValues(t, k0sConfig, "velero", map[string]interface{}{
		"nodeAgent.podVolumePath": "/custom/data/dir/k0s/kubelet/pods",
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestCustomPortsInstallation(t *testing.T) {
	dr := dryrunInstall(t,
		"--local-artifact-mirror-port", "50001",
		"--admin-console-port", "30002",
	)

	// --- validate host preflight spec --- //
	assertCollectors(t, dr.HostPreflightSpec.Collectors, map[string]struct {
		match    func(*troubleshootv1beta2.HostCollect) bool
		validate func(*troubleshootv1beta2.HostCollect)
	}{
		"LAM TCPPortStatus": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.TCPPortStatus != nil && hc.TCPPortStatus.CollectorName == "Local Artifact Mirror Port"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, 50001, hc.TCPPortStatus.Port)
			},
		},
		"Kotsadm TCPPortStatus": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.TCPPortStatus != nil && hc.TCPPortStatus.CollectorName == "Kotsadm Node Port"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, 30002, hc.TCPPortStatus.Port)
			},
		},
	})

	// --- validate metrics --- //
	assertMetrics(t, dr.Metrics, []struct {
		title    string
		validate func(string)
	}{
		{
			title: "InstallationStarted",
			validate: func(payload string) {
				assert.Contains(t, payload, "--local-artifact-mirror-port 50001")
				assert.Contains(t, payload, "--admin-console-port 30002")
			},
		},
		{
			title:    "InstallationSucceeded",
			validate: func(payload string) {},
		},
	})

	// --- validate installation object --- //
	kcli, err := dr.KubeClient()
	if err != nil {
		t.Fatalf("failed to create kube client: %v", err)
	}
	in, err := kubeutils.GetLatestInstallation(context.TODO(), kcli)
	if err != nil {
		t.Fatalf("failed to get latest installation: %v", err)
	}

	assert.Equal(t, 30002, in.Spec.RuntimeConfig.AdminConsole.Port)
	assert.Equal(t, 50001, in.Spec.RuntimeConfig.LocalArtifactMirror.Port)

	// --- validate k0s cluster config --- //
	k0sConfig := readK0sConfig(t)

	assertHelmValues(t, k0sConfig, "admin-console", map[string]interface{}{
		"kurlProxy.nodePort": float64(30002),
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

var (
	//go:embed assets/values.yaml
	valuesYaml []byte
)

func valuesFile(t *testing.T) string {
	valuesYamlFilename := filepath.Join(t.TempDir(), "values.yaml")
	require.NoError(t, os.WriteFile(valuesYamlFilename, valuesYaml, 0644))
	return valuesYamlFilename
}

func TestConfigValuesInstallation(t *testing.T) {
	vf := valuesFile(t)
	dr := dryrunInstall(t,
		"--config-values", vf,
	)

	// --- validate metrics --- //
	assertMetrics(t, dr.Metrics, []struct {
		title    string
		validate func(string)
	}{
		{
			title: "InstallationStarted",
			validate: func(payload string) {
				assert.Contains(t, payload, fmt.Sprintf("--config-values %s", vf))
			},
		},
		{
			title:    "InstallationSucceeded",
			validate: func(payload string) {},
		},
	})

	// --- validate commands --- //
	assertCommands(t, dr.Commands,
		[]interface{}{
			regexp.MustCompile(fmt.Sprintf(`install fake-app-slug/fake-channel-slug .* --config-values %s`, vf)),
		},
		false,
	)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
