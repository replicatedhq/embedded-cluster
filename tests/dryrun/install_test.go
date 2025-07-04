package dryrun

import (
	"context"
	_ "embed"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDefaultInstallation(t *testing.T) {
	testDefaultInstallationImpl(t)
	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

func testDefaultInstallationImpl(t *testing.T) {
	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons
		hcli.On("Install", mock.Anything, mock.Anything).Times(4).Return(nil, nil),
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
	assert.Equal(t, "ec-install", in.ObjectMeta.Labels["replicated.com/disaster-recovery"])

	// --- validate addons --- //

	// openebs
	assert.Equal(t, "Install", hcli.Calls[0].Method)
	openebsOpts := hcli.Calls[0].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "openebs", openebsOpts.ReleaseName)
	assertHelmValues(t, openebsOpts.Values, map[string]interface{}{
		"['localpv-provisioner'].localpv.basePath":         "/var/lib/embedded-cluster/openebs-local",
		"['localpv-provisioner'].helperPod.image.registry": "fake-replicated-proxy.test.net/anonymous/",
		"['localpv-provisioner'].localpv.image.registry":   "fake-replicated-proxy.test.net/anonymous/",
		"['preUpgradeHook'].image.registry":                "fake-replicated-proxy.test.net/anonymous",
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
		"image.repository":        "fake-replicated-proxy.test.net/anonymous/replicated/ec-velero",
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
			title: "InstallationStarted",
			validate: func(payload string) {
				assert.Contains(t, payload, `"entryCommand":"install"`)
				assert.Regexp(t, `"flags":"--license .+/license.yaml --yes"`, payload)
				assert.Contains(t, payload, `"isExitEvent":false`)
				assert.Contains(t, payload, `"eventType":"InstallationStarted"`)
			},
		},
		{
			title: "InstallationSucceeded",
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

	assert.Contains(t, k0sConfig.Spec.Images.MetricsServer.Image, "fake-replicated-proxy.test.net/anonymous")
	assert.Contains(t, k0sConfig.Spec.Images.KubeProxy.Image, "fake-replicated-proxy.test.net/anonymous")
	assert.Contains(t, k0sConfig.Spec.Images.CoreDNS.Image, "fake-replicated-proxy.test.net/anonymous")
	assert.Contains(t, k0sConfig.Spec.Images.Pause.Image, "fake-replicated-proxy.test.net/anonymous")
	assert.Contains(t, k0sConfig.Spec.Images.Calico.CNI.Image, "fake-replicated-proxy.test.net/anonymous")
	assert.Contains(t, k0sConfig.Spec.Images.Calico.Node.Image, "fake-replicated-proxy.test.net/anonymous")
	assert.Contains(t, k0sConfig.Spec.Images.Calico.KubeControllers.Image, "fake-replicated-proxy.test.net/anonymous")
}

func TestCustomDataDir(t *testing.T) {
	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons
		hcli.On("Install", mock.Anything, mock.Anything).Times(4).Return(nil, nil),
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
		// 4 addons
		hcli.On("Install", mock.Anything, mock.Anything).Times(4).Return(nil, nil),
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
	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons
		hcli.On("Install", mock.Anything, mock.Anything).Times(4).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	vf := valuesFile(t)
	dr := dryrunInstall(t,
		&dryrun.Client{HelmClient: hcli},
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
		// 4 addons
		hcli.On("Install", mock.Anything, mock.Anything).Times(4).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	dr := dryrunInstall(t,
		&dryrun.Client{HelmClient: hcli},
		"--cidr", "10.2.0.0/16",
	)

	// --- validate commands --- //
	assertCommands(t, dr.Commands,
		[]interface{}{
			"firewall-cmd --info-zone ec-net",
			"firewall-cmd --add-source 10.2.0.0/17 --permanent --zone ec-net",
			"firewall-cmd --add-source 10.2.128.0/17 --permanent --zone ec-net",
			"firewall-cmd --reload",
		},
		false,
	)

	// --- validate k0s cluster config --- //
	k0sConfig := readK0sConfig(t)

	assert.Equal(t, "10.2.0.0/17", k0sConfig.Spec.Network.PodCIDR)
	assert.Equal(t, "10.2.128.0/17", k0sConfig.Spec.Network.ServiceCIDR)

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// this test is to ensure that when no domains are provided in the cluster config that the domains from the embedded release file are used
func TestNoDomains(t *testing.T) {
	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons
		hcli.On("Install", mock.Anything, mock.Anything).Times(4).Return(nil, nil),
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
		"['localpv-provisioner'].helperPod.image.registry": "proxy.staging.replicated.com/anonymous/",
		"['localpv-provisioner'].localpv.image.registry":   "proxy.staging.replicated.com/anonymous/",
		"['preUpgradeHook'].image.registry":                "proxy.staging.replicated.com/anonymous",
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
		"image.repository":        "proxy.staging.replicated.com/anonymous/replicated/ec-velero",
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

	assert.Contains(t, k0sConfig.Spec.Images.MetricsServer.Image, "proxy.staging.replicated.com/anonymous")
	assert.Contains(t, k0sConfig.Spec.Images.KubeProxy.Image, "proxy.staging.replicated.com/anonymous")
	assert.Contains(t, k0sConfig.Spec.Images.CoreDNS.Image, "proxy.staging.replicated.com/anonymous")
	assert.Contains(t, k0sConfig.Spec.Images.Pause.Image, "proxy.staging.replicated.com/anonymous")
	assert.Contains(t, k0sConfig.Spec.Images.Calico.CNI.Image, "proxy.staging.replicated.com/anonymous")
	assert.Contains(t, k0sConfig.Spec.Images.Calico.Node.Image, "proxy.staging.replicated.com/anonymous")
	assert.Contains(t, k0sConfig.Spec.Images.Calico.KubeControllers.Image, "proxy.staging.replicated.com/anonymous")

	t.Logf("%s: test complete", time.Now().Format(time.RFC3339))
}

// this test is to verify HTTP proxy + CA bundle configuration together in Helm values for addons
func TestHTTPProxyWithCABundleConfiguration(t *testing.T) {
	hostCABundle := findHostCABundle(t)

	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons
		hcli.On("Install", mock.Anything, mock.Anything).Times(4).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	// Set HTTP proxy environment variables
	t.Setenv("HTTP_PROXY", "http://localhost:3128")
	t.Setenv("HTTPS_PROXY", "https://localhost:3128")
	t.Setenv("NO_PROXY", "localhost,127.0.0.1,10.0.0.0/8")

	dr := dryrunInstall(t, &dryrun.Client{HelmClient: hcli})

	// --- validate addons --- //

	// embedded cluster operator
	assert.Equal(t, "Install", hcli.Calls[1].Method)
	operatorOpts := hcli.Calls[1].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "embedded-cluster-operator", operatorOpts.ReleaseName)

	// NO_PROXY is calculated
	val, err := helm.GetValue(operatorOpts.Values, "extraEnv")
	require.NoError(t, err)
	var noProxy string
	for _, v := range val.([]map[string]any) {
		if v["name"] == "NO_PROXY" {
			noProxy = v["value"].(string)
		}
	}
	assert.NotEmpty(t, noProxy)
	assert.Contains(t, noProxy, "10.0.0.0/8")

	assertHelmValues(t, operatorOpts.Values, map[string]any{
		"extraEnv": []map[string]any{
			{
				"name":  "HTTP_PROXY",
				"value": "http://localhost:3128",
			},
			{
				"name":  "HTTPS_PROXY",
				"value": "https://localhost:3128",
			},
			{
				"name":  "NO_PROXY",
				"value": noProxy,
			},
			{
				"name":  "SSL_CERT_DIR",
				"value": "/certs",
			},
			{
				"name":  "PRIVATE_CA_BUNDLE_PATH",
				"value": "/certs/ca-certificates.crt",
			},
		},
		"extraVolumes": []map[string]any{{
			"name": "host-ca-bundle",
			"hostPath": map[string]any{
				"path": hostCABundle,
				"type": "FileOrCreate",
			},
		}},
		"extraVolumeMounts": []map[string]any{{
			"mountPath": "/certs/ca-certificates.crt",
			"name":      "host-ca-bundle",
		}},
	})

	// velero
	assert.Equal(t, "Install", hcli.Calls[2].Method)
	veleroOpts := hcli.Calls[2].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "velero", veleroOpts.ReleaseName)

	assertHelmValues(t, veleroOpts.Values, map[string]any{
		"configuration.extraEnvVars": []map[string]any{
			{
				"name":  "HTTP_PROXY",
				"value": "http://localhost:3128",
			},
			{
				"name":  "HTTPS_PROXY",
				"value": "https://localhost:3128",
			},
			{
				"name":  "NO_PROXY",
				"value": noProxy,
			},
			{
				"name":  "SSL_CERT_DIR",
				"value": "/certs",
			},
		},
		"extraVolumes": []map[string]any{{
			"name": "host-ca-bundle",
			"hostPath": map[string]any{
				"path": hostCABundle,
				"type": "FileOrCreate",
			},
		}},
		"extraVolumeMounts": []map[string]any{{
			"mountPath": "/certs/ca-certificates.crt",
			"name":      "host-ca-bundle",
		}},
		"nodeAgent.extraVolumes": []map[string]any{{
			"name": "host-ca-bundle",
			"hostPath": map[string]any{
				"path": hostCABundle,
				"type": "FileOrCreate",
			},
		}},
		"nodeAgent.extraVolumeMounts": []map[string]any{{
			"mountPath": "/certs/ca-certificates.crt",
			"name":      "host-ca-bundle",
		}},
	})

	// admin console
	assert.Equal(t, "Install", hcli.Calls[3].Method)
	adminConsoleOpts := hcli.Calls[3].Arguments[1].(helm.InstallOptions)
	assert.Equal(t, "admin-console", adminConsoleOpts.ReleaseName)

	assertHelmValues(t, adminConsoleOpts.Values, map[string]any{
		"extraEnv": []map[string]any{
			{
				"name":  "ENABLE_IMPROVED_DR",
				"value": "true",
			},
			{
				"name":  "SSL_CERT_CONFIGMAP",
				"value": "kotsadm-private-cas",
			},
			{
				"name":  "HTTP_PROXY",
				"value": "http://localhost:3128",
			},
			{
				"name":  "HTTPS_PROXY",
				"value": "https://localhost:3128",
			},
			{
				"name":  "NO_PROXY",
				"value": noProxy,
			},
			{
				"name":  "SSL_CERT_DIR",
				"value": "/certs",
			},
		},
		"extraVolumes": []map[string]any{{
			"name": "host-ca-bundle",
			"hostPath": map[string]any{
				"path": hostCABundle,
				"type": "FileOrCreate",
			},
		}},
		"extraVolumeMounts": []map[string]any{{
			"mountPath": "/certs/ca-certificates.crt",
			"name":      "host-ca-bundle",
		}},
	})

	// --- validate host preflight spec --- //
	assertCollectors(t, dr.HostPreflightSpec.Collectors, map[string]struct {
		match    func(*troubleshootv1beta2.HostCollect) bool
		validate func(*troubleshootv1beta2.HostCollect)
	}{
		"http-replicated-app": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.HTTP != nil && hc.HTTP.CollectorName == "http-replicated-app"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, "https://localhost:3128", hc.HTTP.Get.Proxy)
			},
		},
		"http-proxy-replicated-com": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.HTTP != nil && hc.HTTP.CollectorName == "http-proxy-replicated-com"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, "https://localhost:3128", hc.HTTP.Get.Proxy)
			},
		},
	})

	// --- validate cluster resources --- //
	kcli, err := dr.KubeClient()
	if err != nil {
		t.Fatalf("failed to create kube client: %v", err)
	}

	// --- validate installation object --- //
	in, err := kubeutils.GetLatestInstallation(context.TODO(), kcli)
	if err != nil {
		t.Fatalf("failed to get latest installation: %v", err)
	}

	assert.Equal(t, hostCABundle, in.Spec.RuntimeConfig.HostCABundlePath)

	var caConfigMap corev1.ConfigMap
	if err = kcli.Get(context.TODO(), client.ObjectKey{Namespace: "kotsadm", Name: "kotsadm-private-cas"}, &caConfigMap); err != nil {
		t.Fatalf("failed to get kotsadm-private-cas configmap: %v", err)
	}
	assert.Contains(t, caConfigMap.Data, "ca_0.crt", "kotsadm-private-cas configmap should contain ca_0.crt")

	// Verify some metrics were captured
	assert.NotEmpty(t, dr.Metrics)
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
