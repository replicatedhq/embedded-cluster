package dryrun

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	nodeutil "k8s.io/component-helpers/node/util"
)

func TestDefaultInstallation(t *testing.T) {
	testDefaultInstallationImpl(t)
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func testDefaultInstallationImpl(t *testing.T) {
	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons + Goldpinger extension
		hcli.On("Install", mock.Anything, mock.Anything).Times(5).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	dr := dryrunInstall(t, &dryrun.Client{HelmClient: hcli})

	kcli, err := dr.KubeClient()
	if err != nil {
		t.Fatalf("failed to create kube client: %v", err)
	}

	// --- validate installation object --- //
	in, err := kubeutils.GetLatestInstallation(context.TODO(), kcli)
	if err != nil {
		t.Fatalf("failed to get latest installation: %v", err)
	}

	assert.NotEmpty(t, in.Spec.ClusterID)
	assert.Equal(t, "80-32767", in.Spec.RuntimeConfig.Network.NodePortRange)
	assert.Equal(t, "10.244.0.0/16", dr.Flags["cidr"])
	assert.Equal(t, "10.244.0.0/17", in.Spec.RuntimeConfig.Network.PodCIDR)
	assert.Equal(t, "10.244.128.0/17", in.Spec.RuntimeConfig.Network.ServiceCIDR)
	assert.Equal(t, 30000, in.Spec.RuntimeConfig.AdminConsole.Port)
	assert.Equal(t, "/var/lib/embedded-cluster", in.Spec.RuntimeConfig.DataDir)
	assert.Equal(t, 50000, in.Spec.RuntimeConfig.LocalArtifactMirror.Port)
	assert.Equal(t, "ec-install", in.Labels["replicated.com/disaster-recovery"])

	// --- validate addons --- //

	// openebs
	assert.Equal(t, "Install", hcli.Calls[0].Method)
	openebsOpts := hcli.Calls[0].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "openebs", openebsOpts.ReleaseName)
	assertHelmValues(t, openebsOpts.Values, map[string]interface{}{
		"['localpv-provisioner'].localpv.basePath":         "/var/lib/embedded-cluster/openebs-local",
		"['localpv-provisioner'].helperPod.image.registry": "fake-replicated-proxy.test.net/",
		"['localpv-provisioner'].localpv.image.registry":   "fake-replicated-proxy.test.net/",
		"['preUpgradeHook'].image.registry":                "fake-replicated-proxy.test.net",
	})

	// embedded cluster operator
	assert.Equal(t, "Install", hcli.Calls[1].Method)
	operatorOpts := hcli.Calls[1].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "embedded-cluster-operator", operatorOpts.ReleaseName)
	assertHelmValues(t, operatorOpts.Values, map[string]interface{}{
		"embeddedClusterID": in.Spec.ClusterID,
		"image.repository":  "fake-replicated-proxy.test.net/anonymous/replicated/embedded-cluster-operator-image",
	})

	// velero
	assert.Equal(t, "Install", hcli.Calls[2].Method)
	veleroOpts := hcli.Calls[2].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "velero", veleroOpts.ReleaseName)
	assertHelmValues(t, veleroOpts.Values, map[string]interface{}{
		"nodeAgent.podVolumePath": "/var/lib/embedded-cluster/k0s/kubelet/pods",
		"image.repository":        "fake-replicated-proxy.test.net/library/velero",
	})

	// admin console
	assert.Equal(t, "Install", hcli.Calls[3].Method)
	adminConsoleOpts := hcli.Calls[3].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "admin-console", adminConsoleOpts.ReleaseName)
	assertHelmValues(t, adminConsoleOpts.Values, map[string]interface{}{
		"isMultiNodeEnabled":     true,
		"kurlProxy.nodePort":     float64(30000),
		"embeddedClusterID":      in.Spec.ClusterID,
		"embeddedClusterDataDir": "/var/lib/embedded-cluster",
		"embeddedClusterK0sDir":  "/var/lib/embedded-cluster/k0s",
		// kotsadm resources overrides
		"kotsadm.resources.limits.memory":   "4Gi",
		"kotsadm.resources.requests.cpu":    "200m",
		"kotsadm.resources.requests.memory": "300Mi",
		// rqlite resources overrides
		"rqlite.resources.limits.memory":   "3Gi",
		"rqlite.resources.requests.cpu":    "150m",
		"rqlite.resources.requests.memory": "512Mi",
	})
	assertHelmValuePrefixes(t, adminConsoleOpts.Values, map[string]string{
		"images.kotsadm":    "fake-replicated-proxy.test.net/anonymous",
		"images.kurlProxy":  "fake-replicated-proxy.test.net/anonymous",
		"images.migrations": "fake-replicated-proxy.test.net/anonymous",
		"images.rqlite":     "fake-replicated-proxy.test.net/anonymous",
	})

	// --- validate os env --- //
	assertEnv(t, dr.OSEnv, map[string]string{
		"TMPDIR":     "/var/lib/embedded-cluster/tmp",
		"KUBECONFIG": "/var/lib/embedded-cluster/k0s/pki/admin.conf",
	})

	// --- validate log directory (V2 static path) --- //
	logDir := runtimeconfig.EmbeddedClusterLogsSubDir()
	assert.Equal(t, "/var/log/embedded-cluster", logDir, "V2 should use static log directory")

	// --- validate commands --- //
	// Get expected hostname to validate it's included in the kubelet args
	expectedHostname, err := nodeutil.GetHostname("")
	require.NoError(t, err, "could not get hostname")

	assertCommands(t, dr.Commands,
		[]interface{}{
			regexp.MustCompile(fmt.Sprintf(`k0s install controller .* --kubelet-extra-args --node-ip=.* --hostname-override=%s .*--data-dir /var/lib/embedded-cluster/k0s .*--disable-components konnectivity-server,update-prober`, regexp.QuoteMeta(expectedHostname))),
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
			title: "InstallationStarted",
			validate: func(payload string) {
				assert.Contains(t, payload, `"entryCommand":"install"`)
				assert.Regexp(t, `"flags":"--license .+/license.yaml --yes"`, payload)
				assert.Contains(t, payload, `"isExitEvent":false`)
				assert.Contains(t, payload, `"eventType":"InstallationStarted"`)
			},
		},
		{
			title: "GenericEvent",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":true`)
				assert.Contains(t, payload, `"eventType":"InstallationSucceeded"`)
			},
		},
	})

	// --- validate cluster resources --- //

	assertConfigMapExists(t, kcli, "embedded-cluster-host-support-bundle", "kotsadm")
	assertSecretExists(t, kcli, "kotsadm-password", "kotsadm")
	assertSecretExists(t, kcli, "cloud-credentials", "velero")

	// --- validate k0s cluster config --- //
	k0sConfig := readK0sConfig(t)

	assert.Equal(t, "10.244.0.0/17", k0sConfig.Spec.Network.PodCIDR)
	assert.Equal(t, "10.244.128.0/17", k0sConfig.Spec.Network.ServiceCIDR)
	assert.Contains(t, k0sConfig.Spec.API.SANs, "kubernetes.default.svc.cluster.local")

	assert.Contains(t, k0sConfig.Spec.Images.MetricsServer.Image, "fake-replicated-proxy.test.net/library")
	assert.Contains(t, k0sConfig.Spec.Images.KubeProxy.Image, "fake-replicated-proxy.test.net/library")
	assert.Contains(t, k0sConfig.Spec.Images.CoreDNS.Image, "fake-replicated-proxy.test.net/library")
	assert.Contains(t, k0sConfig.Spec.Images.Pause.Image, "fake-replicated-proxy.test.net/library")
	assert.Contains(t, k0sConfig.Spec.Images.Calico.CNI.Image, "fake-replicated-proxy.test.net/library")
	assert.Contains(t, k0sConfig.Spec.Images.Calico.Node.Image, "fake-replicated-proxy.test.net/library")
	assert.Contains(t, k0sConfig.Spec.Images.Calico.KubeControllers.Image, "fake-replicated-proxy.test.net/library")

	// validate unsupported overrides were applied
	assert.Equal(t, "testing-overrides-k0s-name", k0sConfig.Name, "k0s config name should be set from unsupported-overrides")

	// telemetry
	assert.NotNil(t, k0sConfig.Spec.Telemetry, "telemetry config should exist from unsupported-overrides")
	require.NotNil(t, k0sConfig.Spec.Telemetry.Enabled, "telemetry enabled field should exist")
	assert.False(t, *k0sConfig.Spec.Telemetry.Enabled, "telemetry should be enabled from unsupported-overrides")

	// api extraArgs
	require.NotNil(t, k0sConfig.Spec.API, "api config should exist")
	require.NotNil(t, k0sConfig.Spec.API.ExtraArgs, "api extraArgs should exist")
	assert.Equal(t, "test-value", k0sConfig.Spec.API.ExtraArgs["test-key"], "api extraArgs should contain test-key from unsupported-overrides")

	// worker profiles
	require.Len(t, k0sConfig.Spec.WorkerProfiles, 1, "workerProfiles should have one profile from unsupported-overrides")
	assert.Equal(t, "ip-forward", k0sConfig.Spec.WorkerProfiles[0].Name, "workerProfile name should be set from unsupported-overrides")
	require.NotNil(t, k0sConfig.Spec.WorkerProfiles[0].Config, "workerProfile config should exist")

	var profileConfig map[string]interface{}
	err = json.Unmarshal(k0sConfig.Spec.WorkerProfiles[0].Config.Raw, &profileConfig)
	require.NoError(t, err, "should be able to unmarshal workerProfile config")
	sysctls := profileConfig["allowedUnsafeSysctls"].([]interface{})
	assert.Equal(t, "net.ipv4.ip_forward", sysctls[0], "allowedUnsafeSysctls should contain net.ipv4.ip_forward from unsupported-overrides")
}

func TestCustomDataDir(t *testing.T) {
	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons + Goldpinger extension
		hcli.On("Install", mock.Anything, mock.Anything).Times(5).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	dr := dryrunInstall(t,
		&dryrun.Client{HelmClient: hcli},
		"--data-dir", "/custom/data/dir",
	)

	// --- validate addons --- //

	// openebs
	assert.Equal(t, "Install", hcli.Calls[0].Method)
	openebsOpts := hcli.Calls[0].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "openebs", openebsOpts.ReleaseName)
	assertHelmValues(t, openebsOpts.Values, map[string]interface{}{
		"['localpv-provisioner'].localpv.basePath": "/custom/data/dir/openebs-local",
	})

	// embedded cluster operator
	assert.Equal(t, "Install", hcli.Calls[1].Method)
	operatorOpts := hcli.Calls[1].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "embedded-cluster-operator", operatorOpts.ReleaseName)

	// velero
	assert.Equal(t, "Install", hcli.Calls[2].Method)
	veleroOpts := hcli.Calls[2].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "velero", veleroOpts.ReleaseName)
	assertHelmValues(t, veleroOpts.Values, map[string]interface{}{
		"nodeAgent.podVolumePath": "/custom/data/dir/k0s/kubelet/pods",
	})

	// admin console
	assert.Equal(t, "Install", hcli.Calls[3].Method)
	adminConsoleOpts := hcli.Calls[3].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "admin-console", adminConsoleOpts.ReleaseName)
	assertHelmValues(t, adminConsoleOpts.Values, map[string]interface{}{
		"embeddedClusterDataDir": "/custom/data/dir",
		"embeddedClusterK0sDir":  "/custom/data/dir/k0s",
	})

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

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestCustomPortsInstallation(t *testing.T) {
	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons + Goldpinger extension
		hcli.On("Install", mock.Anything, mock.Anything).Times(5).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	dr := dryrunInstall(t,
		&dryrun.Client{HelmClient: hcli},
		"--local-artifact-mirror-port", "50001",
		"--admin-console-port", "30002",
	)

	// --- validate addons --- //

	// openebs
	assert.Equal(t, "Install", hcli.Calls[0].Method)
	openebsOpts := hcli.Calls[0].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "openebs", openebsOpts.ReleaseName)

	// embedded cluster operator
	assert.Equal(t, "Install", hcli.Calls[1].Method)
	operatorOpts := hcli.Calls[1].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "embedded-cluster-operator", operatorOpts.ReleaseName)

	// velero
	assert.Equal(t, "Install", hcli.Calls[2].Method)
	veleroOpts := hcli.Calls[2].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "velero", veleroOpts.ReleaseName)

	// admin console
	assert.Equal(t, "Install", hcli.Calls[3].Method)
	adminConsoleOpts := hcli.Calls[3].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "admin-console", adminConsoleOpts.ReleaseName)
	assertHelmValues(t, adminConsoleOpts.Values, map[string]interface{}{
		"kurlProxy.nodePort": float64(30002),
	})

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
			title: "GenericEvent",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":true`)
				assert.Contains(t, payload, `"eventType":"InstallationSucceeded"`)
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

	assert.Equal(t, 30002, in.Spec.RuntimeConfig.AdminConsole.Port)
	assert.Equal(t, 50001, in.Spec.RuntimeConfig.LocalArtifactMirror.Port)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

var (
	//go:embed assets/values.yaml
	valuesYaml []byte
)

func valuesFile(t *testing.T) string {
	t.Helper()

	valuesYamlFilename := filepath.Join(t.TempDir(), "values.yaml")
	writeFile(t, valuesYamlFilename, string(valuesYaml))
	return valuesYamlFilename
}

func TestConfigValuesInstallation(t *testing.T) {
	hcli := &helm.MockClient{}

	mock.InOrder(
		// 5 addons + Goldpinger extension
		hcli.On("Install", mock.Anything, mock.Anything).Times(6).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	airgapBundle := airgapBundleFile(t)

	vf := valuesFile(t)
	dr := dryrunInstall(t,
		&dryrun.Client{HelmClient: hcli},
		"--config-values", vf,
		"--airgap-bundle", airgapBundle,
	)

	// --- validate commands --- //
	assertCommands(t, dr.Commands,
		[]interface{}{
			// kots cli install command
			regexp.MustCompile(fmt.Sprintf(`install fake-app-slug/fake-channel-slug .* --airgap-bundle %s --config-values %s`, airgapBundle, vf)),
		},
		false,
	)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestRestrictiveUmask(t *testing.T) {
	oldUmask := syscall.Umask(0o077)
	defer syscall.Umask(oldUmask)

	testDefaultInstallationImpl(t)

	// check that folders created in this test have the right permissions
	rc := runtimeconfig.New(nil)
	folderList := []string{
		rc.EmbeddedClusterHomeDirectory(),
		rc.EmbeddedClusterBinsSubDir(),
		rc.EmbeddedClusterChartsSubDir(),
		rc.PathToEmbeddedClusterBinary("kubectl-preflight"),
	}
	gotFailure := false
	for _, folder := range folderList {
		stat, err := os.Stat(folder)
		if err != nil {
			t.Logf("failed to stat %s: %v", folder, err)
			gotFailure = true
			continue
		}
		if stat.Mode().Perm() != 0755 {
			t.Logf("expected folder %s to have mode 0755, got %O", folder, stat.Mode().Perm())
			gotFailure = true
		}
	}
	if gotFailure {
		t.Fatalf("at least one folder had incorrect permissions")
	}

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestCustomCidrInstallation(t *testing.T) {
	hcli := &helm.MockClient{}

	mock.InOrder(
		// 5 addons + Goldpinger extension
		hcli.On("Install", mock.Anything, mock.Anything).Times(6).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	dr := dryrunInstall(t,
		&dryrun.Client{HelmClient: hcli},
		"--cidr", "10.2.0.0/16",
		"--airgap-bundle", airgapBundleFile(t),
		"--http-proxy", "http://localhost:3128",
		"--https-proxy", "https://localhost:3128",
		"--no-proxy", "localhost,127.0.0.1,10.0.0.0/8",
	)

	validateCustomCIDR(t, &dr, hcli)

	if !t.Failed() {
		t.Logf("Test passed: custom CIDR correctly propagates to all external dependencies")
	}
}

// this test is to ensure that when no domains are provided in the cluster config that the domains from the embedded release file are used
func TestNoDomains(t *testing.T) {
	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons + Goldpinger extension
		hcli.On("Install", mock.Anything, mock.Anything).Times(5).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	dr := dryrunInstallWithClusterConfig(t,
		&dryrun.Client{HelmClient: hcli},
		clusterConfigNoDomainsData,
	)

	// --- validate addons --- //

	// openebs
	assert.Equal(t, "Install", hcli.Calls[0].Method)
	openebsOpts := hcli.Calls[0].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "openebs", openebsOpts.ReleaseName)
	assertHelmValues(t, openebsOpts.Values, map[string]interface{}{
		"['localpv-provisioner'].localpv.basePath":         "/var/lib/embedded-cluster/openebs-local",
		"['localpv-provisioner'].helperPod.image.registry": "proxy.staging.replicated.com/",
		"['localpv-provisioner'].localpv.image.registry":   "proxy.staging.replicated.com/",
		"['preUpgradeHook'].image.registry":                "proxy.staging.replicated.com",
	})

	// embedded cluster operator
	assert.Equal(t, "Install", hcli.Calls[1].Method)
	operatorOpts := hcli.Calls[1].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "embedded-cluster-operator", operatorOpts.ReleaseName)
	assertHelmValues(t, operatorOpts.Values, map[string]interface{}{
		"image.repository": "proxy.staging.replicated.com/anonymous/replicated/embedded-cluster-operator-image",
	})

	// velero
	assert.Equal(t, "Install", hcli.Calls[2].Method)
	veleroOpts := hcli.Calls[2].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "velero", veleroOpts.ReleaseName)
	assertHelmValues(t, veleroOpts.Values, map[string]interface{}{
		"nodeAgent.podVolumePath": "/var/lib/embedded-cluster/k0s/kubelet/pods",
		"image.repository":        "proxy.staging.replicated.com/library/velero",
	})

	// admin console
	assert.Equal(t, "Install", hcli.Calls[3].Method)
	adminConsoleOpts := hcli.Calls[3].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "admin-console", adminConsoleOpts.ReleaseName)
	assertHelmValues(t, adminConsoleOpts.Values, map[string]interface{}{
		"kurlProxy.nodePort": float64(30000),
	})
	assertHelmValuePrefixes(t, adminConsoleOpts.Values, map[string]string{
		"images.kotsadm":    "proxy.staging.replicated.com/anonymous",
		"images.kurlProxy":  "proxy.staging.replicated.com/anonymous",
		"images.migrations": "proxy.staging.replicated.com/anonymous",
		"images.rqlite":     "proxy.staging.replicated.com/anonymous",
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
	// expected to be empty
	assert.Equal(t, "", in.Spec.Config.Domains.ProxyRegistryDomain)
	assert.Equal(t, "", in.Spec.Config.Domains.ReplicatedAppDomain)
	assert.Equal(t, "", in.Spec.Config.Domains.ReplicatedRegistryDomain)

	// --- validate k0s cluster config --- //
	k0sConfig := readK0sConfig(t)

	assert.Contains(t, k0sConfig.Spec.Images.MetricsServer.Image, "proxy.staging.replicated.com/library")
	assert.Contains(t, k0sConfig.Spec.Images.KubeProxy.Image, "proxy.staging.replicated.com/library")
	assert.Contains(t, k0sConfig.Spec.Images.CoreDNS.Image, "proxy.staging.replicated.com/library")
	assert.Contains(t, k0sConfig.Spec.Images.Pause.Image, "proxy.staging.replicated.com/library")
	assert.Contains(t, k0sConfig.Spec.Images.Calico.CNI.Image, "proxy.staging.replicated.com/library")
	assert.Contains(t, k0sConfig.Spec.Images.Calico.Node.Image, "proxy.staging.replicated.com/library")
	assert.Contains(t, k0sConfig.Spec.Images.Calico.KubeControllers.Image, "proxy.staging.replicated.com/library")

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func findHostCABundle(t *testing.T) string {
	// From https://github.com/golang/go/blob/go1.24.3/src/crypto/x509/root_linux.go
	certFiles := []string{
		"/etc/ssl/certs/ca-certificates.crt",                // Debian/Ubuntu/Gentoo etc.
		"/etc/pki/tls/certs/ca-bundle.crt",                  // Fedora/RHEL 6
		"/etc/ssl/ca-bundle.pem",                            // OpenSUSE
		"/etc/pki/tls/cacert.pem",                           // OpenELEC
		"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem", // CentOS/RHEL 7
		"/etc/ssl/cert.pem",                                 // Alpine Linux
	}

	for _, file := range certFiles {
		if _, err := os.Stat(file); err == nil {
			return file
		}
	}

	t.Fatalf("no host CA bundle found")
	return ""
}

func TestTLSConfigurationInstallation(t *testing.T) {
	// Create test certificate and key files
	tmpdir := t.TempDir()
	certPath := filepath.Join(tmpdir, "test-cert.pem")
	keyPath := filepath.Join(tmpdir, "test-key.pem")

	// Valid test certificate and key data (same as unit tests)
	certData := `-----BEGIN CERTIFICATE-----
MIIDizCCAnOgAwIBAgIUJaAILNY7l9MR4mfMP4WiUObo6TIwDQYJKoZIhvcNAQEL
BQAwVTELMAkGA1UEBhMCVVMxDTALBgNVBAgMBFRlc3QxDTALBgNVBAcMBFRlc3Qx
DTALBgNVBAoMBFRlc3QxGTAXBgNVBAMMEHRlc3QuZXhhbXBsZS5jb20wHhcNMjUw
ODE5MTcwNTU4WhcNMjYwODE5MTcwNTU4WjBVMQswCQYDVQQGEwJVUzENMAsGA1UE
CAwEVGVzdDENMAsGA1UEBwwEVGVzdDENMAsGA1UECgwEVGVzdDEZMBcGA1UEAwwQ
dGVzdC5leGFtcGxlLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
AMhkRyxUJE4JLrTbqq/Etdvd2osmkZJA5GXCRkWcGLBppNNqO1v8K0zy5dV9jgno
gjeQD2nTqZ++vmzR3wPObeB6MJY+2SYtFHvnT3G9HR4DcSX3uHUOBDjbUsW0OT6z
weT3t3eTVqNIY96rZRHz9VYrdC4EPlWyfoYTCHceZey3AqSgHWnHIxVaATWT/LFQ
yvRRlEBNf7/M5NX0qis91wKgGwe6u+P/ebmT1cXURufM0jSAMUbDIqr73Qq5m6t4
fv6/8XKAiVpA1VcACvR79kTi6hYMls88ShHuYLJK175ZQfkeJx77TI/UebALL9CZ
SCI1B08SMZOsr9GQMOKNIl8CAwEAAaNTMFEwHQYDVR0OBBYEFCQWAH7mJ0w4Iehv
PL72t8GCJ90uMB8GA1UdIwQYMBaAFCQWAH7mJ0w4IehvPL72t8GCJ90uMA8GA1Ud
EwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEBAFfEICcE4eFZkRfjcEkvrJ3T
KmMikNP2nPXv3h5Ie0DpprejPkDyOWe+UJBanYwAf8xXVwRTmE5PqQhEik2zTBlN
N745Izq1cUYIlyt9GHHycx384osYHKkGE9lAPEvyftlc9hCLSu/FVQ3+8CGwGm9i
cFNYLx/qrKkJxT0Lohi7VCAf7+S9UWjIiLaETGlejm6kPNLRZ0VoxIPgUmqePXfp
6gY5FSIzvH1kZ+bPZ3nqsGyT1l7TsubeTPDDGhpKgIFzcJX9WeY//bI4q1SpU1Fl
koNnBhDuuJxjiafIFCz4qVlf0kmRrz4jeXGXym8IjxUq0EpMgxGuSIkguPKiwFQ=
-----END CERTIFICATE-----`

	keyData := `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDIZEcsVCROCS60
26qvxLXb3dqLJpGSQORlwkZFnBiwaaTTajtb/CtM8uXVfY4J6II3kA9p06mfvr5s
0d8Dzm3gejCWPtkmLRR7509xvR0eA3El97h1DgQ421LFtDk+s8Hk97d3k1ajSGPe
q2UR8/VWK3QuBD5Vsn6GEwh3HmXstwKkoB1pxyMVWgE1k/yxUMr0UZRATX+/zOTV
9KorPdcCoBsHurvj/3m5k9XF1EbnzNI0gDFGwyKq+90KuZureH7+v/FygIlaQNVX
AAr0e/ZE4uoWDJbPPEoR7mCySte+WUH5Hice+0yP1HmwCy/QmUgiNQdPEjGTrK/R
kDDijSJfAgMBAAECggEAHnl1g23GWaG22yU+110cZPPfrOKwJ6Q7t6fsRODAtm9S
dB5HKa13LkwQHL/rzmDwEKAVX/wi4xrAXc8q0areddFPO0IShuY7I76hC8R9PZe7
aNE72X1IshbUhyFpxTnUBkyPt50OA2XaXj4FcE3/5NtV3zug+SpcaGpTkr3qNS24
0Qf5X8AA1STec81c4BaXc8GgLsXz/4kWUSiwK0fjXcIpHkW28gtUyVmYu3FAPSdo
4bKdbqNUiYxF+JYLCQ9PyvFAqy7EhFLM4QkMICnSBNqNCPq3hVOr8K4V9luNnAmS
oU5gEHXmGM8a+kkdvLoZn3dO5tRk8ctV0vnLMYnXrQKBgQDl4/HDbv3oMiqS9nJK
+vQ7/yzLUb00fVzvWbvSLdEfGCgbRlDRKkNMgI5/BnFTJcbG5o3rIdBW37FY3iAy
p4iIm+VGiDz4lFApAQdiQXk9d2/mfB9ZVryUsKskvk6WTjom6+BRSvakqe2jIa/i
udnMFNGkJj6HzZqss1LKDiR5DQKBgQDfJqj5AlCyNUxjokWMH0BapuBVSHYZnxxD
xR5xX/5Q5fKDBpp4hMn8vFS4L8a5mCOBUPbuxEj7KY0Ho5bqYWmt+HyxP5TvDS9h
ZqgDdJuWdLB4hfzlUKekufFrpALvUT4AbmYdQ+ufkggU0mWGCfKaijlk4Hy/VRH7
w5ConbJWGwKBgADkF0XIoldKCnwzVFISEuxAmu3WzULs0XVkBaRU5SCXuWARr7J/
1W7weJzpa3sFBHY04ovsv5/2kftkMP/BQng1EnhpgsL74Cuog1zQICYq1lYwWPbB
rU1uOduUmT1f5D3OYDowbjBJMFCXitT4H235Dq7yLv/bviO5NjLuRxnpAoGBAJBj
LnA4jEhS7kOFiuSYkAZX9c2Y3jnD1wEOuZz4VNC5iMo46phSq3Np1JN87mPGSirx
XWWvAd3py8QGmK69KykTIHN7xX1MFb07NDlQKSAYDttdLv6dymtumQRiEjgRZEHZ
LR+AhCQy1CHM5T3uj9ho2awpCO6wN7uklaRUrUDDAoGBAK/EPsIxm5yj+kFIc/qk
SGwCw13pfbshh9hyU6O//h3czLnN9dgTllfsC7qqxsgrMCVZO9ZIfh5eb44+p7Id
r3glM4yhSJwf/cAWmt1A7DGOYnV7FF2wkDJJPX/Vag1uEsqrzwnAdFBymK5dwDsu
oxhVqyhpk86rf0rT5DcD/sBw
-----END PRIVATE KEY-----`

	writeFile(t, certPath, certData)
	writeFile(t, keyPath, keyData)

	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons + Goldpinger extension
		hcli.On("Install", mock.Anything, mock.Anything).Times(5).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	dr := dryrunInstall(t,
		&dryrun.Client{HelmClient: hcli},
		"--tls-cert", certPath,
		"--tls-key", keyPath,
		"--hostname", "test.example.com",
	)

	kcli, err := dr.KubeClient()
	require.NoError(t, err)

	// --- validate that TLS secret exists --- //
	assertSecretExists(t, kcli, "kotsadm-tls", "kotsadm")

	// --- validate TLS secret contents --- //
	var tlsSecret corev1.Secret
	err = kcli.Get(context.TODO(), types.NamespacedName{Name: "kotsadm-tls", Namespace: "kotsadm"}, &tlsSecret)
	require.NoError(t, err)

	// Check secret type
	assert.Equal(t, corev1.SecretTypeTLS, tlsSecret.Type)

	// Check certificate data
	assert.Equal(t, []byte(certData), tlsSecret.Data["tls.crt"])
	assert.Equal(t, []byte(keyData), tlsSecret.Data["tls.key"])

	// Check hostname in StringData
	assert.Equal(t, "test.example.com", tlsSecret.StringData["hostname"])

	// Check labels
	assert.Equal(t, "true", tlsSecret.Labels["kots.io/kotsadm"])
	assert.Equal(t, "infra", tlsSecret.Labels["replicated.com/disaster-recovery"])
	assert.Equal(t, "admin-console", tlsSecret.Labels["replicated.com/disaster-recovery-chart"])

	// Check annotations
	assert.Equal(t, "0", tlsSecret.Annotations["acceptAnonymousUploads"])

	// --- validate metrics include TLS flags --- //
	assertMetrics(t, dr.Metrics, []struct {
		title    string
		validate func(string)
	}{
		{
			title: "InstallationStarted",
			validate: func(payload string) {
				assert.Contains(t, payload, "--tls-cert")
				assert.Contains(t, payload, "--tls-key")
				assert.Contains(t, payload, "--hostname test.example.com")
			},
		},
		{
			title: "GenericEvent",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":true`)
				assert.Contains(t, payload, `"eventType":"InstallationSucceeded"`)
			},
		},
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestIgnoreAppPreflightsInstallation(t *testing.T) {
	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons + Goldpinger extension
		hcli.On("Install", mock.Anything, mock.Anything).Times(5).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	dr := dryrunInstall(t,
		&dryrun.Client{HelmClient: hcli},
		"--ignore-app-preflights",
	)

	// --- validate commands --- //
	assertCommands(t, dr.Commands,
		[]any{
			regexp.MustCompile(`install fake-app-slug/fake-channel-slug .* --skip-preflights`),
		},
		false,
	)

	// --- validate metrics --- //
	assertMetrics(t, dr.Metrics, []struct {
		title    string
		validate func(string)
	}{
		{
			title: "InstallationStarted",
			validate: func(payload string) {
				assert.Contains(t, payload, "--ignore-app-preflights")
			},
		},
		{
			title: "GenericEvent",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":true`)
				assert.Contains(t, payload, `"eventType":"InstallationSucceeded"`)
			},
		},
	})

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func TestVeleroPluginsInstallation(t *testing.T) {
	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons (no extensions in this cluster config)
		hcli.On("Install", mock.Anything, mock.Anything).Times(4).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	dryrunInstallWithClusterConfig(t, &dryrun.Client{HelmClient: hcli}, clusterConfigWithVeleroPluginsData)

	// --- validate velero addon --- //
	veleroOpts, found := isHelmReleaseInstalled(hcli, "velero")
	require.True(t, found, "velero helm release should be installed")

	// Validate basic Velero values
	assertHelmValues(t, veleroOpts.Values, map[string]interface{}{
		"nodeAgent.podVolumePath": "/var/lib/embedded-cluster/k0s/kubelet/pods",
	})

	// Validate plugin configuration
	validateVeleroPlugin(t, hcli)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}
