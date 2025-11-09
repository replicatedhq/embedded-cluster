package dryrun

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestV3InstallHeadless_HappyPathOnline(t *testing.T) {
	hcli := setupV3HeadlessTestHelmClient()
	licenseFile, configFile := setupV3HeadlessTest(t, hcli)

	adminConsoleNamespace := "fake-app-slug"

	// Run installer command with headless flag and required arguments
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
	)

	require.NoError(t, err, "headless installation should succeed")

	// Load dryrun output to validate registry resources are NOT created
	dr, err := dryrun.Load()
	require.NoError(t, err, "failed to load dryrun output")

	kcli, err := dr.KubeClient()
	require.NoError(t, err, "failed to get kube client")

	// Validate Installation object
	in, err := kubeutils.GetLatestInstallation(t.Context(), kcli)
	require.NoError(t, err, "failed to get latest installation")
	assert.NotEmpty(t, in.Spec.ClusterID, "Installation.Spec.ClusterID should be set")
	assert.False(t, in.Spec.AirGap, "Installation.Spec.AirGap should be false for online installations")
	assert.Equal(t, int64(0), in.Spec.AirgapUncompressedSize, "Installation.Spec.AirgapUncompressedSize should be 0 for online installations")
	assert.Equal(t, "80-32767", in.Spec.RuntimeConfig.Network.NodePortRange, "Installation.Spec.RuntimeConfig.Network.NodePortRange should be set to default range")

	// Validate that HTTP collectors are present in host preflight spec for online installations
	assertCollectors(t, dr.HostPreflightSpec.Collectors, map[string]struct {
		match    func(*troubleshootv1beta2.HostCollect) bool
		validate func(*troubleshootv1beta2.HostCollect)
	}{
		"http-replicated-app": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.HTTP != nil && hc.HTTP.CollectorName == "http-replicated-app"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.NotEmpty(t, hc.HTTP.Get.URL, "http-replicated-app collector should have a URL")
				assert.Equal(t, "false", hc.HTTP.Exclude.String(), "http-replicated-app collector should not be excluded in online installations")
			},
		},
		"http-proxy-replicated-com": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.HTTP != nil && hc.HTTP.CollectorName == "http-proxy-replicated-com"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.NotEmpty(t, hc.HTTP.Get.URL, "http-proxy-replicated-com collector should have a URL")
				assert.Equal(t, "false", hc.HTTP.Exclude.String(), "http-proxy-replicated-com collector should not be excluded in online installations")
			},
		},
	})

	// Validate that embedded-cluster-path-usage collector uses default data directory
	assertCollectors(t, dr.HostPreflightSpec.Collectors, map[string]struct {
		match    func(*troubleshootv1beta2.HostCollect) bool
		validate func(*troubleshootv1beta2.HostCollect)
	}{
		"embedded-cluster-path-usage": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.DiskUsage != nil && hc.DiskUsage.CollectorName == "embedded-cluster-path-usage"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, "/var/lib/fake-app-slug", hc.DiskUsage.Path, "embedded-cluster-path-usage collector should use default data directory")
			},
		},
	})

	// Validate that Airgap Storage Space analyzer is NOT present for online installations
	// This analyzer is only needed for airgap installations to check disk space for bundle processing
	for _, analyzer := range dr.HostPreflightSpec.Analyzers {
		if analyzer.DiskUsage != nil && analyzer.DiskUsage.CheckName == "Airgap Storage Space" {
			assert.Fail(t, "Airgap Storage Space analyzer should not be present in online installations")
		}
	}

	// Validate addons

	// Validate embedded-cluster-operator addon
	operatorOpts, found := isHelmReleaseInstalled(hcli, "embedded-cluster-operator")
	require.True(t, found, "embedded-cluster-operator helm release should be installed")
	assertHelmValues(t, operatorOpts.Values, map[string]any{
		"embeddedClusterID": in.Spec.ClusterID,
	})
	// Validate that isAirgap helm value is not set in embedded-cluster-operator chart for online installations
	_, hasIsAirgap := operatorOpts.Values["isAirgap"]
	assert.False(t, hasIsAirgap, "embedded-cluster-operator should not have isAirgap helm value for online installations")

	// Validate admin-console addon
	adminConsoleOpts, found := isHelmReleaseInstalled(hcli, "admin-console")
	require.True(t, found, "admin-console helm release should be installed")
	assertHelmValues(t, adminConsoleOpts.Values, map[string]any{
		"isAirgap":           false,
		"isMultiNodeEnabled": true,
		"embeddedClusterID":  in.Spec.ClusterID,
	})

	// Validate that registry addon is NOT installed for online installations
	_, found = isHelmReleaseInstalled(hcli, "docker-registry")
	require.False(t, found, "docker-registry helm release should not be installed")

	// Validate that registry-creds secret is NOT created for online installations
	assertSecretNotExists(t, kcli, "registry-creds", adminConsoleNamespace)

	// Validate OS environment variables use default data directory
	assertEnv(t, dr.OSEnv, map[string]string{
		"TMPDIR":     "/var/lib/fake-app-slug/tmp",
		"KUBECONFIG": "/var/lib/fake-app-slug/k0s/pki/admin.conf",
	})

	// Validate host preflight spec uses default ports
	assertCollectors(t, dr.HostPreflightSpec.Collectors, map[string]struct {
		match    func(*troubleshootv1beta2.HostCollect) bool
		validate func(*troubleshootv1beta2.HostCollect)
	}{
		"Kotsadm Node Port": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.TCPPortStatus != nil && hc.TCPPortStatus.CollectorName == "Kotsadm Node Port"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, 30000, hc.TCPPortStatus.Port, "Kotsadm Node Port collector should use default admin console port")
			},
		},
		"Local Artifact Mirror Port": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.TCPPortStatus != nil && hc.TCPPortStatus.CollectorName == "Local Artifact Mirror Port"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, 50000, hc.TCPPortStatus.Port, "Local Artifact Mirror Port collector should use default port")
			},
		},
	})

	// Validate that KOTS CLI install command is present
	assertCommands(t, dr.Commands,
		[]any{
			regexp.MustCompile(`kubectl-kots.* install fake-app-slug/fake-channel-slug .*`),
		},
		false,
	)

	t.Logf("Test passed: headless online installation does not create registry addon or registry-creds secret")
}

func TestV3InstallHeadless_HappyPathAirgap(t *testing.T) {
	hcli := setupV3HeadlessTestHelmClient()
	licenseFile, configFile := setupV3HeadlessTest(t, hcli)

	airgapBundleFile := airgapBundleFile(t)

	adminConsoleNamespace := "fake-app-slug"

	// Run installer command with headless flag and required arguments
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--airgap-bundle", airgapBundleFile,
		"--yes",
	)

	require.NoError(t, err, "headless installation should succeed")

	// Load dryrun output to validate registry resources ARE created
	dr, err := dryrun.Load()
	require.NoError(t, err, "failed to load dryrun output")

	kcli, err := dr.KubeClient()
	require.NoError(t, err, "failed to get kube client")

	// Validate Installation object has correct AirGap settings
	in, err := kubeutils.GetLatestInstallation(t.Context(), kcli)
	require.NoError(t, err, "failed to get latest installation")
	assert.True(t, in.Spec.AirGap, "Installation.Spec.AirGap should be true for airgap installations")
	// TODO: fix this test
	// assert.Greater(t, in.Spec.AirgapUncompressedSize, int64(0), "Installation.Spec.AirgapUncompressedSize should be greater than 0 for airgap installations")

	// Validate that HTTP collectors are NOT present in host preflight spec for airgap installations
	// These collectors check connectivity to replicated.app and proxy.replicated.com which are excluded in airgap mode
	assertCollectors(t, dr.HostPreflightSpec.Collectors, map[string]struct {
		match    func(*troubleshootv1beta2.HostCollect) bool
		validate func(*troubleshootv1beta2.HostCollect)
	}{
		"http-replicated-app": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.HTTP != nil && hc.HTTP.CollectorName == "http-replicated-app"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.NotEmpty(t, hc.HTTP.Get.URL, "http-replicated-app collector should have a URL")
				assert.Equal(t, "true", hc.HTTP.Exclude.String(), "http-replicated-app collector should be excluded in airgap installations")
			},
		},
		"http-proxy-replicated-com": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.HTTP != nil && hc.HTTP.CollectorName == "http-proxy-replicated-com"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.NotEmpty(t, hc.HTTP.Get.URL, "http-proxy-replicated-com collector should have a URL")
				assert.Equal(t, "true", hc.HTTP.Exclude.String(), "http-proxy-replicated-com collector should be excluded in airgap installations")
			},
		},
	})

	// Validate that Airgap Storage Space analyzer IS present for airgap installations
	// This analyzer checks if there's sufficient disk space to process the airgap bundle
	assertAnalyzers(t, dr.HostPreflightSpec.Analyzers, map[string]struct {
		match    func(*troubleshootv1beta2.HostAnalyze) bool
		validate func(*troubleshootv1beta2.HostAnalyze)
	}{
		"Airgap Storage Space": {
			match: func(hc *troubleshootv1beta2.HostAnalyze) bool {
				return hc.DiskUsage != nil && hc.DiskUsage.CheckName == "Airgap Storage Space"
			},
			validate: func(hc *troubleshootv1beta2.HostAnalyze) {
				assert.Equal(t, "Airgap Storage Space", hc.DiskUsage.CheckName, "Airgap Storage Space analyzer should check airgap storage space")
			},
		},
	})

	// Validate that registry addon IS installed for airgap installations
	_, found := isHelmReleaseInstalled(hcli, "docker-registry")
	require.True(t, found, "docker-registry helm release should be installed")

	// Validate that isAirgap helm value is set to true in admin console chart
	adminConsoleOpts, found := isHelmReleaseInstalled(hcli, "admin-console")
	require.True(t, found, "admin-console helm release should be installed")
	assertHelmValues(t, adminConsoleOpts.Values, map[string]any{
		"isAirgap": true,
	})

	// Validate that isAirgap helm value is set to "true" in embedded-cluster-operator chart for airgap installations
	operatorOpts, found := isHelmReleaseInstalled(hcli, "embedded-cluster-operator")
	require.True(t, found, "embedded-cluster-operator helm release should be installed")
	assertHelmValues(t, operatorOpts.Values, map[string]any{
		"isAirgap": "true",
	})

	// Validate that registry-creds secret IS created for airgap installations
	assertSecretExists(t, kcli, "registry-creds", adminConsoleNamespace)

	// Validate that KOTS CLI install command includes --airgap-bundle flag for airgap installations
	// The --airgap-bundle flag flows through: Installer → Install Controller → App Install Manager
	// The App Install Manager uses it to set kotscli.InstallOptions.AirgapBundle (install.go:68)
	// This ensures the KOTS installer receives the airgap bundle path
	assertCommands(t, dr.Commands,
		[]any{
			// KOTS install command should contain --airgap-bundle with the correct path
			regexp.MustCompile(fmt.Sprintf(`kubectl-kots.* install fake-app-slug/fake-channel-slug .* --airgap-bundle %s`, regexp.QuoteMeta(airgapBundleFile))),
		},
		false,
	)

	t.Logf("Test passed: headless airgap installation creates registry addon and registry-creds secret")
}

func TestV3InstallHeadless_Metrics(t *testing.T) {
	licenseFile, configFile := setupV3HeadlessTest(t, nil)

	// Run installer command with headless flag and required arguments
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
	)

	require.NoError(t, err, "headless installation should succeed")

	dr, err := dryrun.Load()
	if err != nil {
		t.Fatalf("fail to unmarshal dryrun output: %v", err)
	}

	// --- validate metrics --- //
	assertMetrics(t, dr.Metrics, []struct {
		title    string
		validate func(string)
	}{
		{
			title: "InstallationStarted",
			validate: func(payload string) {
				assert.Contains(t, payload, `"entryCommand":"install"`)
				assert.Regexp(t, `"flags":".*--headless.+--license .+/license.yaml.+--target linux.*"`, payload)
				assert.Contains(t, payload, `"isExitEvent":false`)
				assert.Contains(t, payload, `"eventType":"InstallationStarted"`)
			},
		},
		{
			title: "GenericEvent",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":false`)
				assert.Contains(t, payload, `"eventType":"PreflightsSucceeded"`)
			},
		},
		{
			title: "GenericEvent",
			validate: func(payload string) {
				assert.Contains(t, payload, `"isExitEvent":false`)
				assert.Contains(t, payload, `"eventType":"AppPreflightsSucceeded"`)
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

	t.Logf("Test passed: metrics are recorded correctly")
}

func TestV3InstallHeadless_ConfigValidationErrors(t *testing.T) {
	licenseFile, configFile := setupV3HeadlessTest(t, nil)

	// Override the config file with invalid values
	createInvalidConfigValuesFile(t, configFile)

	// Run installer command with headless flag and invalid config values
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
	)

	// Expect the command to fail with the specific error message
	require.EqualError(t, err, `application configuration validation failed: field errors:
  - Field 'text_required_with_regex': Please enter a valid email address
  - Field 'file_required': File Required is required`)

	t.Logf("Test passed: config values validation errors are displayed to the user")
}

func TestV3InstallHeadless_CustomCIDR(t *testing.T) {
	hcli := setupV3HeadlessTestHelmClient()

	licenseFile, configFile := setupV3HeadlessTest(t, hcli)

	// Run installer command with custom CIDR and proxy settings
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--cidr", "10.2.0.0/16",
		"--airgap-bundle", airgapBundleFile(t),
		"--http-proxy", "http://localhost:3128",
		"--https-proxy", "https://localhost:3128",
		"--no-proxy", "localhost,127.0.0.1,10.0.0.0/8",
		"--yes",
	)
	require.NoError(t, err, "headless installation should succeed")

	dr, err := dryrun.Load()
	if err != nil {
		t.Fatalf("fail to unmarshal dryrun output: %v", err)
	}

	validateCustomCIDR(t, dr, hcli)

	if !t.Failed() {
		t.Logf("Test passed: custom CIDR correctly propagates to all external dependencies")
	}
}

func TestV3InstallHeadless_CustomDomains(t *testing.T) {
	hcli := setupV3HeadlessTestHelmClient()
	licenseFile, configFile := setupV3HeadlessTest(t, hcli)

	// Run installer command with headless flag and required arguments
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
	)

	require.NoError(t, err, "headless installation should succeed")

	// Validate addon image registries/repositories use custom domains

	// Validate openebs addon uses custom domain
	openebsOpts, found := isHelmReleaseInstalled(hcli, "openebs")
	require.True(t, found, "openebs helm release should be installed")
	assertHelmValues(t, openebsOpts.Values, map[string]any{
		"['localpv-provisioner'].helperPod.image.registry": "fake-replicated-proxy.test.net/",
		"['localpv-provisioner'].localpv.image.registry":   "fake-replicated-proxy.test.net/",
		"['preUpgradeHook'].image.registry":                "fake-replicated-proxy.test.net",
	})

	// Validate embedded-cluster-operator addon uses custom domain
	operatorOpts, found := isHelmReleaseInstalled(hcli, "embedded-cluster-operator")
	require.True(t, found, "embedded-cluster-operator helm release should be installed")
	assertHelmValues(t, operatorOpts.Values, map[string]any{
		"image.repository": "fake-replicated-proxy.test.net/anonymous/replicated/embedded-cluster-operator-image",
	})

	// Validate velero addon uses custom domain
	veleroOpts, found := isHelmReleaseInstalled(hcli, "velero")
	require.True(t, found, "velero helm release should be installed")
	assertHelmValues(t, veleroOpts.Values, map[string]any{
		"image.repository": "fake-replicated-proxy.test.net/library/velero",
	})

	// Validate admin-console addon uses custom domain for all images
	adminConsoleOpts, found := isHelmReleaseInstalled(hcli, "admin-console")
	require.True(t, found, "admin-console helm release should be installed")
	assertHelmValuePrefixes(t, adminConsoleOpts.Values, map[string]string{
		"images.kotsadm":    "fake-replicated-proxy.test.net/anonymous",
		"images.kurlProxy":  "fake-replicated-proxy.test.net/anonymous",
		"images.migrations": "fake-replicated-proxy.test.net/anonymous",
		"images.rqlite":     "fake-replicated-proxy.test.net/anonymous",
	})

	// Validate k0s cluster config images use custom domain
	k0sConfig := readK0sConfig(t)
	assert.Contains(t, k0sConfig.Spec.Images.MetricsServer.Image, "fake-replicated-proxy.test.net/library", "MetricsServer image should use custom domain")
	assert.Contains(t, k0sConfig.Spec.Images.KubeProxy.Image, "fake-replicated-proxy.test.net/library", "KubeProxy image should use custom domain")
	assert.Contains(t, k0sConfig.Spec.Images.CoreDNS.Image, "fake-replicated-proxy.test.net/library", "CoreDNS image should use custom domain")
	assert.Contains(t, k0sConfig.Spec.Images.Pause.Image, "fake-replicated-proxy.test.net/library", "Pause image should use custom domain")
	assert.Contains(t, k0sConfig.Spec.Images.Calico.CNI.Image, "fake-replicated-proxy.test.net/library", "Calico CNI image should use custom domain")
	assert.Contains(t, k0sConfig.Spec.Images.Calico.Node.Image, "fake-replicated-proxy.test.net/library", "Calico Node image should use custom domain")
	assert.Contains(t, k0sConfig.Spec.Images.Calico.KubeControllers.Image, "fake-replicated-proxy.test.net/library", "Calico KubeControllers image should use custom domain")

	t.Logf("Test passed: custom domains correctly propagate to all addon image registries and k0s cluster config images")
}

func TestV3InstallHeadless_CustomDataDir(t *testing.T) {
	hcli := setupV3HeadlessTestHelmClient()
	licenseFile, configFile := setupV3HeadlessTest(t, hcli)

	customDataDir := "/custom/data/dir"

	// Run installer command with custom data directory
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--data-dir", customDataDir,
		"--yes",
	)

	require.NoError(t, err, "headless installation should succeed")

	// Load dryrun output
	dr, err := dryrun.Load()
	require.NoError(t, err, "failed to load dryrun output")

	kcli, err := dr.KubeClient()
	require.NoError(t, err, "failed to get kube client")

	// Validate Installation object
	in, err := kubeutils.GetLatestInstallation(t.Context(), kcli)
	require.NoError(t, err, "failed to get latest installation")
	assert.Equal(t, customDataDir, in.Spec.RuntimeConfig.DataDir, "Installation.Spec.RuntimeConfig.DataDir should use custom data directory")

	// Validate addons use custom data directory

	// Validate openebs addon uses custom data directory
	openebsOpts, found := isHelmReleaseInstalled(hcli, "openebs")
	require.True(t, found, "openebs helm release should be installed")
	assertHelmValues(t, openebsOpts.Values, map[string]any{
		"['localpv-provisioner'].localpv.basePath": customDataDir + "/openebs-local",
	})

	// Validate velero addon uses custom data directory
	veleroOpts, found := isHelmReleaseInstalled(hcli, "velero")
	require.True(t, found, "velero helm release should be installed")
	assertHelmValues(t, veleroOpts.Values, map[string]any{
		"nodeAgent.podVolumePath": customDataDir + "/k0s/kubelet/pods",
	})

	// Validate admin-console addon uses custom data directory
	adminConsoleOpts, found := isHelmReleaseInstalled(hcli, "admin-console")
	require.True(t, found, "admin-console helm release should be installed")
	assertHelmValues(t, adminConsoleOpts.Values, map[string]any{
		"embeddedClusterDataDir": customDataDir,
		"embeddedClusterK0sDir":  customDataDir + "/k0s",
	})

	// Validate OS environment variables use custom data directory
	assertEnv(t, dr.OSEnv, map[string]string{
		"TMPDIR":     customDataDir + "/tmp",
		"KUBECONFIG": customDataDir + "/k0s/pki/admin.conf",
	})

	// Validate commands use custom data directory
	assertCommands(t, dr.Commands,
		[]any{
			regexp.MustCompile(fmt.Sprintf(`k0s install controller .* --data-dir %s/k0s`, regexp.QuoteMeta(customDataDir))),
		},
		false,
	)

	// Validate host preflight spec uses custom data directory
	assertCollectors(t, dr.HostPreflightSpec.Collectors, map[string]struct {
		match    func(*troubleshootv1beta2.HostCollect) bool
		validate func(*troubleshootv1beta2.HostCollect)
	}{
		"embedded-cluster-path-usage": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.DiskUsage != nil && hc.DiskUsage.CollectorName == "embedded-cluster-path-usage"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, customDataDir, hc.DiskUsage.Path, "embedded-cluster-path-usage collector should use custom data directory")
			},
		},
		"FilesystemPerformance": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.FilesystemPerformance != nil
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, customDataDir+"/k0s/etcd", hc.FilesystemPerformance.Directory, "FilesystemPerformance collector should use custom data directory")
			},
		},
	})

	t.Logf("Test passed: custom data directory correctly propagates to all external dependencies")
}

func TestV3InstallHeadless_CustomAdminConsolePort(t *testing.T) {
	hcli := setupV3HeadlessTestHelmClient()
	licenseFile, configFile := setupV3HeadlessTest(t, hcli)

	customPort := 30001

	// Run installer command with custom admin console port
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--admin-console-port", fmt.Sprintf("%d", customPort),
		"--yes",
	)

	require.NoError(t, err, "headless installation should succeed")

	dr, err := dryrun.Load()
	require.NoError(t, err, "failed to load dryrun output")

	kcli, err := dr.KubeClient()
	require.NoError(t, err, "failed to get kube client")

	// Validate Installation object uses custom admin console port
	in, err := kubeutils.GetLatestInstallation(t.Context(), kcli)
	require.NoError(t, err, "failed to get latest installation")
	assert.Equal(t, customPort, in.Spec.RuntimeConfig.AdminConsole.Port, "Installation.Spec.RuntimeConfig.AdminConsole.Port should match custom port")

	// Validate admin-console addon uses custom port
	adminConsoleOpts, found := isHelmReleaseInstalled(hcli, "admin-console")
	require.True(t, found, "admin-console helm release should be installed")
	assertHelmValues(t, adminConsoleOpts.Values, map[string]any{
		"kurlProxy.nodePort": float64(customPort),
	})

	// Validate host preflight spec uses custom admin console port
	assertCollectors(t, dr.HostPreflightSpec.Collectors, map[string]struct {
		match    func(*troubleshootv1beta2.HostCollect) bool
		validate func(*troubleshootv1beta2.HostCollect)
	}{
		"Kotsadm Node Port": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.TCPPortStatus != nil && hc.TCPPortStatus.CollectorName == "Kotsadm Node Port"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, customPort, hc.TCPPortStatus.Port, "Kotsadm Node Port collector should use custom admin console port")
			},
		},
	})

	t.Logf("Test passed: custom admin console port correctly propagates to Installation object, admin-console helm chart, and host preflights")
}

func TestV3InstallHeadless_CustomLocalArtifactMirror(t *testing.T) {
	hcli := setupV3HeadlessTestHelmClient()
	licenseFile, configFile := setupV3HeadlessTest(t, hcli)

	customPort := 50001

	// Run installer command with custom local artifact mirror port
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--local-artifact-mirror-port", fmt.Sprintf("%d", customPort),
		"--yes",
	)

	require.NoError(t, err, "headless installation should succeed")

	dr, err := dryrun.Load()
	require.NoError(t, err, "failed to load dryrun output")

	kcli, err := dr.KubeClient()
	require.NoError(t, err, "failed to get kube client")

	// Validate Installation object uses custom local artifact mirror port
	in, err := kubeutils.GetLatestInstallation(t.Context(), kcli)
	require.NoError(t, err, "failed to get latest installation")
	assert.Equal(t, customPort, in.Spec.RuntimeConfig.LocalArtifactMirror.Port, "Installation.Spec.RuntimeConfig.LocalArtifactMirror.Port should match custom port")

	// Validate host preflight spec uses custom local artifact mirror port
	assertCollectors(t, dr.HostPreflightSpec.Collectors, map[string]struct {
		match    func(*troubleshootv1beta2.HostCollect) bool
		validate func(*troubleshootv1beta2.HostCollect)
	}{
		"Local Artifact Mirror Port": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.TCPPortStatus != nil && hc.TCPPortStatus.CollectorName == "Local Artifact Mirror Port"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, customPort, hc.TCPPortStatus.Port, "Local Artifact Mirror Port collector should use custom port")
			},
		},
	})

	t.Logf("Test passed: custom local artifact mirror port correctly propagates to Installation object and host preflights")
}

func TestV3InstallHeadless_ClusterConfig(t *testing.T) {
	hcli := setupV3HeadlessTestHelmClient()
	licenseFile, configFile := setupV3HeadlessTest(t, hcli)

	// Run installer command with headless flag and required arguments
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
	)

	require.NoError(t, err, "headless installation should succeed")

	// Validate k0s cluster config has unsupported overrides applied
	k0sConfig := readK0sConfig(t)

	// Validate k0s config name override
	assert.Equal(t, "testing-overrides-k0s-name", k0sConfig.Name, "k0s config name should be set from unsupported-overrides")

	// Validate telemetry override
	assert.NotNil(t, k0sConfig.Spec.Telemetry, "telemetry config should exist from unsupported-overrides")
	require.NotNil(t, k0sConfig.Spec.Telemetry.Enabled, "telemetry enabled field should exist")
	assert.False(t, *k0sConfig.Spec.Telemetry.Enabled, "telemetry should be disabled from unsupported-overrides")

	// Validate api extraArgs override
	require.NotNil(t, k0sConfig.Spec.API, "api config should exist")
	require.NotNil(t, k0sConfig.Spec.API.ExtraArgs, "api extraArgs should exist")
	assert.Equal(t, "test-value", k0sConfig.Spec.API.ExtraArgs["test-key"], "api extraArgs should contain test-key from unsupported-overrides")

	// Validate worker profiles override
	require.Len(t, k0sConfig.Spec.WorkerProfiles, 1, "workerProfiles should have one profile from unsupported-overrides")
	assert.Equal(t, "ip-forward", k0sConfig.Spec.WorkerProfiles[0].Name, "workerProfile name should be set from unsupported-overrides")
	require.NotNil(t, k0sConfig.Spec.WorkerProfiles[0].Config, "workerProfile config should exist")

	var profileConfig map[string]any
	err = json.Unmarshal(k0sConfig.Spec.WorkerProfiles[0].Config.Raw, &profileConfig)
	require.NoError(t, err, "should be able to unmarshal workerProfile config")
	sysctls := profileConfig["allowedUnsafeSysctls"].([]any)
	assert.Equal(t, "net.ipv4.ip_forward", sysctls[0], "allowedUnsafeSysctls should contain net.ipv4.ip_forward from unsupported-overrides")

	// Validate controller role name and labels are passed to k0s install command
	dr, err := dryrun.Load()
	require.NoError(t, err, "failed to load dryrun output")

	// Find the k0s install controller command and validate labels
	k0sInstallCmd := findCommand(t, dr.Commands, regexp.MustCompile(`k0s install controller`))
	require.NotNil(t, k0sInstallCmd, "k0s install controller command should exist")

	// Validate all labels are present (order doesn't matter since they're comma-separated)
	assert.Regexp(t, `--labels.*test-label-key=test-label-value`, k0sInstallCmd.Cmd, "k0s install command should contain test-label-key label")
	assert.Regexp(t, `--labels.*another-label=another-value`, k0sInstallCmd.Cmd, "k0s install command should contain another-label label")
	assert.Regexp(t, `--labels.*kots\.io/embedded-cluster-role-0=test-controller-role`, k0sInstallCmd.Cmd, "k0s install command should contain controller role name label")

	t.Logf("Test passed: cluster config with unsupported overrides, controller role name, and labels correctly apply to k0s cluster config and commands")
}

var (
	//go:embed assets/rendered-chart-preflight.yaml
	renderedChartPreflightData string

	//go:embed assets/kotskinds-config-values.yaml
	configValuesData string

	//go:embed assets/kotskinds-config-values-invalid.yaml
	configValuesInvalidData string
)

func setupV3HeadlessTest(t *testing.T, hcli helm.Client) (string, string) {
	// Set ENABLE_V3 environment variable
	t.Setenv("ENABLE_V3", "1")

	// Setup release data with V3-specific release data
	if err := release.SetReleaseDataForTests(map[string][]byte{
		"release.yaml":        []byte(releaseData),
		"cluster-config.yaml": []byte(clusterConfigData),
		"application.yaml":    []byte(applicationData),
		"config.yaml":         []byte(configData),
		"chart.yaml":          []byte(helmChartData),
		"nginx-app-0.1.0.tgz": []byte(helmChartArchiveData),
	}); err != nil {
		t.Fatalf("fail to set release data: %v", err)
	}

	if hcli == nil {
		hcli = setupV3HeadlessTestHelmClient()
	}

	// Initialize dryrun with mock ReplicatedAPIClient
	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	dryrun.Init(drFile, &dryrun.Client{
		ReplicatedAPIClient: &dryrun.ReplicatedAPIClient{
			License:      nil, // will return the same license that was passed in
			LicenseBytes: []byte(licenseData),
		},
		HelmClient: hcli,
	})
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetOutput(os.Stdout)

	// Create license file
	licenseFile := filepath.Join(t.TempDir(), "license.yaml")
	require.NoError(t, os.WriteFile(licenseFile, []byte(licenseData), 0644))

	// Create config values file (required for headless)
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	createConfigValuesFile(t, configFile)

	return licenseFile, configFile
}

func setupV3HeadlessTestHelmClient() *helm.MockClient {
	hcli := &helm.MockClient{}
	hcli.On("Install", mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	hcli.
		On("Render", mock.Anything, mock.MatchedBy(func(opts helm.InstallOptions) bool {
			return opts.ReleaseName == "nginx-app"
		})).
		Return([][]byte{[]byte(renderedChartPreflightData)}, nil).
		Maybe()
	hcli.On("Close").Return(nil).Maybe()

	return hcli
}

// createConfigValuesFile creates a config values file that passes validation
func createConfigValuesFile(t *testing.T, filename string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filename, []byte(configValuesData), 0644))
}

// createInvalidConfigValuesFile creates a config values file that fails validation
func createInvalidConfigValuesFile(t *testing.T, filename string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filename, []byte(configValuesInvalidData), 0644))
}
