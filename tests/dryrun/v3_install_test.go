package dryrun

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
	"testing"

	apiclient "github.com/replicatedhq/embedded-cluster/api/client"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestV3InstallHeadless_HappyPathOnline(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, hcli, nil)

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

	validateHappyPathOnline(t, hcli)

	if !t.Failed() {
		t.Logf("V3 headless online installation test passed")
	}
}

func TestV3Install_HappyPathOnline(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, hcli, nil)

	// Start installer in non-headless mode so API stays up; bypass prompts with --yes
	go func() {
		err := runInstallerCmd(
			"install",
			"--target", "linux",
			"--license", licenseFile,
			"--admin-console-password", "password123",
			"--yes",
		)
		if err != nil {
			t.Logf("installer exited with error: %v", err)
		}
	}()

	runV3Install(t, v3InstallArgs{
		managerPort:          30080,
		password:             "password123",
		isAirgap:             false,
		configValuesFile:     configFile,
		installationConfig:   apitypes.LinuxInstallationConfig{},
		ignoreHostPreflights: false,
		ignoreAppPreflights:  false,
	})

	validateHappyPathOnline(t, hcli)

	if !t.Failed() {
		t.Logf("V3 online installation test passed")
	}
}

func validateHappyPathOnline(t *testing.T, hcli *helm.MockClient) {
	t.Helper()

	adminConsoleNamespace := "fake-app-slug"

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
}

func TestV3InstallHeadless_HappyPathAirgap(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, hcli, nil)

	airgapBundleFile := airgapBundleFile(t)

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

	validateHappyPathAirgap(t, hcli, airgapBundleFile)

	if !t.Failed() {
		t.Logf("V3 headless airgap installation test passed")
	}
}

func TestV3Install_HappyPathAirgap(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, hcli, nil)

	airgapBundleFile := airgapBundleFile(t)

	// Start installer in non-headless mode so API stays up; bypass prompts with --yes
	go func() {
		err := runInstallerCmd(
			"install",
			"--target", "linux",
			"--license", licenseFile,
			"--admin-console-password", "password123",
			"--airgap-bundle", airgapBundleFile,
			"--yes",
		)
		if err != nil {
			t.Logf("installer exited with error: %v", err)
		}
	}()

	runV3Install(t, v3InstallArgs{
		managerPort:          30080,
		password:             "password123",
		isAirgap:             true,
		configValuesFile:     configFile,
		installationConfig:   apitypes.LinuxInstallationConfig{},
		ignoreHostPreflights: false,
		ignoreAppPreflights:  false,
	})

	validateHappyPathAirgap(t, hcli, airgapBundleFile)

	if !t.Failed() {
		t.Logf("V3 airgap installation test passed")
	}
}

func validateHappyPathAirgap(t *testing.T, hcli *helm.MockClient, airgapBundleFile string) {
	t.Helper()

	adminConsoleNamespace := "fake-app-slug"

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
}

func TestV3InstallHeadless_Metrics(t *testing.T) {
	licenseFile, configFile := setupV3Test(t, nil, nil)

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

	validateMetrics(t, true)

	if !t.Failed() {
		t.Logf("V3 headless install metrics test passed")
	}
}

func TestV3Install_Metrics(t *testing.T) {
	licenseFile, configFile := setupV3Test(t, nil, nil)

	// Start installer in non-headless mode so API stays up; bypass prompts with --yes
	go func() {
		err := runInstallerCmd(
			"install",
			"--target", "linux",
			"--license", licenseFile,
			"--admin-console-password", "password123",
			"--yes",
		)
		if err != nil {
			t.Logf("installer exited with error: %v", err)
		}
	}()

	runV3Install(t, v3InstallArgs{
		managerPort:          30080,
		password:             "password123",
		isAirgap:             false,
		configValuesFile:     configFile,
		installationConfig:   apitypes.LinuxInstallationConfig{},
		ignoreHostPreflights: false,
		ignoreAppPreflights:  false,
	})

	validateMetrics(t, false)

	if !t.Failed() {
		t.Logf("V3 install metrics test passed")
	}
}

func validateMetrics(t *testing.T, headless bool) {
	t.Helper()

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
				if headless {
					assert.Regexp(t, `"flags":".*--headless.+--license .+/license.yaml.+--target linux.*"`, payload)
				} else {
					assert.Regexp(t, `"flags":".*--license .+/license.yaml.+--target linux.*"`, payload)
				}
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
}

func TestV3InstallHeadless_ConfigValidationErrors(t *testing.T) {
	licenseFile, configFile := setupV3Test(t, nil, nil)

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

	if !t.Failed() {
		t.Logf("Test passed: config values validation errors are displayed to the user")
	}
}

func TestV3Install_ConfigValidationErrors(t *testing.T) {
	licenseFile, configFile := setupV3Test(t, nil, nil)

	// Override the config file with invalid values
	createInvalidConfigValuesFile(t, configFile)

	// Start installer in non-headless mode so API stays up; bypass prompts with --yes
	go func() {
		err := runInstallerCmd(
			"install",
			"--target", "linux",
			"--license", licenseFile,
			"--admin-console-password", "password123",
			"--yes",
		)
		if err != nil {
			t.Logf("installer exited with error: %v", err)
		}
	}()

	ctx := t.Context()

	// Wait for API be ready
	httpClient := insecureHTTPClient()
	waitForAPIReady(t, httpClient, fmt.Sprintf("https://localhost:%d/api/health", 30080))

	// Build API client and authenticate
	c := apiclient.New(fmt.Sprintf("https://localhost:%d", 30080), apiclient.WithHTTPClient(httpClient))
	require.NoError(t, c.Authenticate(ctx, "password123"))

	// Configure application with invalid config values
	kcv, err := helpers.ParseConfigValues(configFile)
	require.NoError(t, err, "failed to parse config values file")
	appConfigValues := apitypes.ConvertToAppConfigValues(kcv)
	_, err = c.PatchLinuxInstallAppConfigValues(ctx, appConfigValues)

	// Expect request to fail with the specific error message
	require.EqualError(t, err, `field errors: Please enter a valid email address; File Required is required`)

	if !t.Failed() {
		t.Logf("Test passed: config values validation errors are returned to the user")
	}
}

func TestV3InstallHeadless_CustomCIDR(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, hcli, nil)

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
		t.Logf("Test passed: custom CIDR correctly propagates to all external dependencies and cluster config")
	}
}

func TestV3Install_CustomCIDR(t *testing.T) {
	hcli := setupV3TestHelmClient()

	licenseFile, configFile := setupV3Test(t, hcli, nil)

	// Start installer in non-headless mode so API stays up; bypass prompts with --yes
	go func() {
		err := runInstallerCmd(
			"install",
			"--target", "linux",
			"--license", licenseFile,
			"--airgap-bundle", airgapBundleFile(t),
			"--admin-console-password", "password123",
			"--yes",
		)
		if err != nil {
			t.Logf("installer exited with error: %v", err)
		}
	}()

	runV3Install(t, v3InstallArgs{
		managerPort:      30080,
		password:         "password123",
		isAirgap:         true,
		configValuesFile: configFile,
		installationConfig: apitypes.LinuxInstallationConfig{
			GlobalCIDR: "10.2.0.0/16",
			HTTPProxy:  "http://localhost:3128",
			HTTPSProxy: "https://localhost:3128",
			NoProxy:    "localhost,127.0.0.1,10.0.0.0/8",
		},
		ignoreHostPreflights: false,
		ignoreAppPreflights:  false,
	})

	dr, err := dryrun.Load()
	if err != nil {
		t.Fatalf("fail to unmarshal dryrun output: %v", err)
	}

	validateCustomCIDR(t, dr, hcli)

	if !t.Failed() {
		t.Logf("Test passed: custom CIDR correctly propagates to all external dependencies and cluster config")
	}
}

func TestV3InstallHeadless_CustomDomains(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, hcli, nil)

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

	validateCustomDomains(t, hcli)

	t.Logf("Test passed: custom domains correctly propagate to all addon image registries and k0s cluster config images")
}

func TestV3Install_CustomDomains(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, hcli, nil)

	// Start installer in non-headless mode so API stays up; bypass prompts with --yes
	go func() {
		err := runInstallerCmd(
			"install",
			"--target", "linux",
			"--license", licenseFile,
			"--admin-console-password", "password123",
			"--yes",
		)
		if err != nil {
			t.Logf("installer exited with error: %v", err)
		}
	}()

	runV3Install(t, v3InstallArgs{
		managerPort:          30080,
		password:             "password123",
		isAirgap:             false,
		configValuesFile:     configFile,
		installationConfig:   apitypes.LinuxInstallationConfig{},
		ignoreHostPreflights: false,
		ignoreAppPreflights:  false,
	})

	validateCustomDomains(t, hcli)

	if !t.Failed() {
		t.Logf("Test passed: custom domains correctly propagate to all addon image registries and k0s cluster config images")
	}
}

func validateCustomDomains(t *testing.T, hcli *helm.MockClient) {
	t.Helper()

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
}

func TestV3InstallHeadless_CustomDataDir(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, hcli, nil)

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

	validateCustomDataDir(t, hcli, customDataDir)

	if !t.Failed() {
		t.Logf("Test passed: custom data directory correctly propagates to all external dependencies")
	}
}

func TestV3Install_CustomDataDir(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, hcli, nil)

	customDataDir := "/custom/data/dir"

	// Start installer in non-headless mode so API stays up; bypass prompts with --yes
	go func() {
		err := runInstallerCmd(
			"install",
			"--target", "linux",
			"--license", licenseFile,
			"--admin-console-password", "password123",
			"--yes",
		)
		if err != nil {
			t.Logf("installer exited with error: %v", err)
		}
	}()

	runV3Install(t, v3InstallArgs{
		managerPort:      30080,
		password:         "password123",
		isAirgap:         false,
		configValuesFile: configFile,
		installationConfig: apitypes.LinuxInstallationConfig{
			DataDirectory: customDataDir,
		},
		ignoreHostPreflights: false,
		ignoreAppPreflights:  false,
	})

	validateCustomDataDir(t, hcli, customDataDir)

	if !t.Failed() {
		t.Logf("Test passed: custom data directory correctly propagates to all external dependencies")
	}
}

func validateCustomDataDir(t *testing.T, hcli *helm.MockClient, customDataDir string) {
	t.Helper()

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
}

func TestV3InstallHeadless_CustomAdminConsolePort(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, hcli, nil)

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

	validateCustomAdminConsolePort(t, hcli, customPort)

	if !t.Failed() {
		t.Logf("Test passed: custom admin console port correctly propagates to Installation object, admin-console helm chart, and host preflights")
	}
}

func TestV3Install_CustomAdminConsolePort(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, hcli, nil)

	customPort := 30001

	// Start installer in non-headless mode so API stays up; bypass prompts with --yes
	go func() {
		err := runInstallerCmd(
			"install",
			"--target", "linux",
			"--license", licenseFile,
			"--admin-console-password", "password123",
			"--yes",
		)
		if err != nil {
			t.Logf("installer exited with error: %v", err)
		}
	}()

	runV3Install(t, v3InstallArgs{
		managerPort:      30080,
		password:         "password123",
		isAirgap:         false,
		configValuesFile: configFile,
		installationConfig: apitypes.LinuxInstallationConfig{
			AdminConsolePort: customPort,
		},
		ignoreHostPreflights: false,
		ignoreAppPreflights:  false,
	})

	validateCustomAdminConsolePort(t, hcli, customPort)

	if !t.Failed() {
		t.Logf("Test passed: custom admin console port correctly propagates to Installation object, admin-console helm chart, and host preflights")
	}
}

func validateCustomAdminConsolePort(t *testing.T, hcli *helm.MockClient, customPort int) {
	t.Helper()

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
}

func TestV3InstallHeadless_CustomLocalArtifactMirrorPort(t *testing.T) {
	licenseFile, configFile := setupV3Test(t, nil, nil)

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

	validateCustomLocalArtifactMirrorPort(t, customPort)

	if !t.Failed() {
		t.Logf("Test passed: custom local artifact mirror port correctly propagates to Installation object and host preflights")
	}
}

func TestV3Install_CustomLocalArtifactMirrorPort(t *testing.T) {
	licenseFile, configFile := setupV3Test(t, nil, nil)

	customPort := 50001

	// Start installer in non-headless mode so API stays up; bypass prompts with --yes
	go func() {
		err := runInstallerCmd(
			"install",
			"--target", "linux",
			"--license", licenseFile,
			"--admin-console-password", "password123",
			"--yes",
		)
		if err != nil {
			t.Logf("installer exited with error: %v", err)
		}
	}()

	runV3Install(t, v3InstallArgs{
		managerPort:      30080,
		password:         "password123",
		isAirgap:         false,
		configValuesFile: configFile,
		installationConfig: apitypes.LinuxInstallationConfig{
			LocalArtifactMirrorPort: customPort,
		},
		ignoreHostPreflights: false,
		ignoreAppPreflights:  false,
	})

	validateCustomLocalArtifactMirrorPort(t, customPort)

	if !t.Failed() {
		t.Logf("Test passed: custom local artifact mirror port correctly propagates to Installation object and host preflights")
	}
}

func validateCustomLocalArtifactMirrorPort(t *testing.T, customPort int) {
	t.Helper()

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
}

func TestV3InstallHeadless_ClusterConfig(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, hcli, nil)

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

	validateClusterConfig(t, hcli)

	if !t.Failed() {
		t.Logf("Test passed: cluster config with unsupported overrides, controller role name, labels, builtInExtensions, and helm extensions correctly apply to k0s cluster config and helm releases")
	}
}

func TestV3Install_ClusterConfig(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, hcli, nil)

	// Start installer in non-headless mode so API stays up; bypass prompts with --yes
	go func() {
		err := runInstallerCmd(
			"install",
			"--target", "linux",
			"--license", licenseFile,
			"--admin-console-password", "password123",
			"--yes",
		)
		if err != nil {
			t.Logf("installer exited with error: %v", err)
		}
	}()

	runV3Install(t, v3InstallArgs{
		managerPort:          30080,
		password:             "password123",
		isAirgap:             false,
		configValuesFile:     configFile,
		installationConfig:   apitypes.LinuxInstallationConfig{},
		ignoreHostPreflights: false,
		ignoreAppPreflights:  false,
	})

	validateClusterConfig(t, hcli)

	if !t.Failed() {
		t.Logf("Test passed: cluster config with unsupported overrides, controller role name, labels, builtInExtensions, and helm extensions correctly apply to k0s cluster config and helm releases")
	}
}

func validateClusterConfig(t *testing.T, hcli *helm.MockClient) {
	t.Helper()

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
	err := json.Unmarshal(k0sConfig.Spec.WorkerProfiles[0].Config.Raw, &profileConfig)
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

	// Validate builtInExtensions custom values for admin-console
	adminConsoleOpts, found := isHelmReleaseInstalled(hcli, "admin-console")
	require.True(t, found, "admin-console helm release should be installed")
	assertHelmValues(t, adminConsoleOpts.Values, map[string]any{
		"['labels']['release-custom-label']": "release-clustom-value",
	})

	// Validate extensions.helm.charts - goldpinger chart should be installed
	goldpingerOpts, found := isHelmReleaseInstalled(hcli, "goldpinger")
	require.True(t, found, "goldpinger helm release should be installed")
	assert.Equal(t, "goldpinger", goldpingerOpts.Namespace, "goldpinger should be installed in goldpinger namespace")
	assert.Equal(t, "6.1.2", goldpingerOpts.ChartVersion, "goldpinger should have version 6.1.2")
	assertHelmValues(t, goldpingerOpts.Values, map[string]any{
		"['image']['repository']": "ec-e2e-proxy.testcluster.net/anonymous/bloomberg/goldpinger",
	})
}

func TestV3InstallHeadless_RestrictiveUmask(t *testing.T) {
	// Set restrictive umask and defer restoration
	oldUmask := syscall.Umask(0o077)
	defer syscall.Umask(oldUmask)

	licenseFile, configFile := setupV3Test(t, nil, nil)

	// Run installer command with headless flag
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

	// Check that folders created in this test have the right permissions
	rc := runtimeconfig.New(nil)
	rc.SetDataDir("/var/lib/fake-app-slug")
	folderList := []string{
		runtimeconfig.EmbeddedClusterLogsSubDir(),
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

	if !t.Failed() {
		t.Logf("Test passed: folders created with correct permissions (0755) despite restrictive umask (0o077)")
	}
}

func TestV3InstallHeadless_CustomTLSConfiguration(t *testing.T) {
	licenseFile, configFile := setupV3Test(t, nil, nil)

	// Create test certificate and key files
	tmpdir := t.TempDir()
	certPath := filepath.Join(tmpdir, "test-cert.pem")
	keyPath := filepath.Join(tmpdir, "test-key.pem")

	// Valid test certificate and key data
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

	// Run installer command with TLS configuration
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--tls-cert", certPath,
		"--tls-key", keyPath,
		"--hostname", "test.example.com",
		// No need to pass --yes here because we are setting tls cert and key
	)

	require.NoError(t, err, "headless installation should succeed")

	// Load dryrun output
	dr, err := dryrun.Load()
	require.NoError(t, err, "failed to load dryrun output")

	kcli, err := dr.KubeClient()
	require.NoError(t, err, "failed to get kube client")

	// Validate that TLS secret exists
	assertSecretExists(t, kcli, "kotsadm-tls", "fake-app-slug")

	// Validate TLS secret contents
	var tlsSecret corev1.Secret
	err = kcli.Get(t.Context(), types.NamespacedName{Name: "kotsadm-tls", Namespace: "fake-app-slug"}, &tlsSecret)
	require.NoError(t, err)

	// Check secret type
	assert.Equal(t, corev1.SecretTypeTLS, tlsSecret.Type, "secret should have type kubernetes.io/tls")

	// Check certificate data
	assert.Equal(t, []byte(certData), tlsSecret.Data["tls.crt"], "certificate data should match")
	assert.Equal(t, []byte(keyData), tlsSecret.Data["tls.key"], "key data should match")

	// Check hostname in StringData
	assert.Equal(t, "test.example.com", tlsSecret.StringData["hostname"], "hostname should match")

	// Check annotations
	assert.Equal(t, "0", tlsSecret.Annotations["acceptAnonymousUploads"], "should have acceptAnonymousUploads annotation")

	if !t.Failed() {
		t.Logf("Test passed: custom TLS configuration correctly creates kotsadm-tls secret with proper certificate, key, hostname, and annotation")
	}
}

func TestV3InstallHeadless_HostPreflights_WithFailuresBlocking(t *testing.T) {
	preflightRunner := setupV3TestHostPreflightsRunnerWithFail()
	licenseFile, configFile := setupV3Test(t, nil, preflightRunner)

	// Run installer command without ignore-host-preflights flag
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
	)

	require.ErrorContains(t, err, "host preflight checks completed with failures")

	preflightRunner.AssertExpectations(t)

	if !t.Failed() {
		t.Logf("Test passed: host preflight failures blocking installation")
	}
}

func TestV3InstallHeadless_HostPreflights_WithFailuresBypass(t *testing.T) {
	preflightRunner := setupV3TestHostPreflightsRunnerWithFail()
	licenseFile, configFile := setupV3Test(t, nil, preflightRunner)

	// Run installer command with ignore-host-preflights flag
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--ignore-host-preflights",
		"--yes",
	)

	require.NoError(t, err, "headless installation should succeed")

	preflightRunner.AssertExpectations(t)

	if !t.Failed() {
		t.Logf("Test passed: --ignore-host-preflights flag correctly bypasses host preflight failures")
	}
}

func TestV3InstallHeadless_HostPreflights_Fail(t *testing.T) {
	preflightRunner := setupV3TestHostPreflightsRunnerFail()
	licenseFile, configFile := setupV3Test(t, nil, preflightRunner)

	// Run installer command without ignore-host-preflights flag
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
	)

	require.ErrorContains(t, err, "preflights failed to run")
	require.ErrorContains(t, err, "exit code 1")

	preflightRunner.AssertExpectations(t)

	if !t.Failed() {
		t.Logf("Test passed: preflights failed to run")
	}
}

func TestV3InstallHeadless_HostPreflights_FailNoBypass(t *testing.T) {
	preflightRunner := setupV3TestHostPreflightsRunnerFail()
	licenseFile, configFile := setupV3Test(t, nil, preflightRunner)

	// Run installer command without ignore-host-preflights flag
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--ignore-host-preflights",
		"--yes",
	)

	require.ErrorContains(t, err, "preflights failed to run")
	require.ErrorContains(t, err, "exit code 1")

	preflightRunner.AssertExpectations(t)

	if !t.Failed() {
		t.Logf("Test passed: preflights failed to run")
	}
}

func TestV3InstallHeadless_AppPreflights_WithFailuresBlocking(t *testing.T) {
	preflightRunner := setupV3TestAppPreflightsRunnerWithFail()
	licenseFile, configFile := setupV3Test(t, nil, preflightRunner)

	// Run installer command without ignore-app-preflights flag
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
	)

	require.ErrorContains(t, err, "app preflight checks completed with failures")

	preflightRunner.AssertExpectations(t)

	if !t.Failed() {
		t.Logf("Test passed: app preflight failures blocking installation")
	}
}

func TestV3InstallHeadless_AppPreflights_WithFailuresBypass(t *testing.T) {
	preflightRunner := setupV3TestAppPreflightsRunnerWithFail()
	licenseFile, configFile := setupV3Test(t, nil, preflightRunner)

	// Run installer command with ignore-app-preflights flag
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--ignore-app-preflights",
		"--yes",
	)

	require.NoError(t, err, "headless installation should succeed")

	preflightRunner.AssertExpectations(t)

	if !t.Failed() {
		t.Logf("Test passed: --ignore-app-preflights flag correctly bypasses app preflight failures")
	}
}

func TestV3InstallHeadless_AppPreflights_Fail(t *testing.T) {
	preflightRunner := setupV3TestAppPreflightsRunnerFail()
	licenseFile, configFile := setupV3Test(t, nil, preflightRunner)

	// Run installer command without ignore-app-preflights flag
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
	)

	require.ErrorContains(t, err, "app preflights failed to run")
	require.ErrorContains(t, err, "exit code 1")

	preflightRunner.AssertExpectations(t)

	if !t.Failed() {
		t.Logf("Test passed: app preflights execution failed")
	}
}

func TestV3InstallHeadless_AppPreflights_FailNoBypass(t *testing.T) {
	preflightRunner := setupV3TestAppPreflightsRunnerFail()
	licenseFile, configFile := setupV3Test(t, nil, preflightRunner)

	// Run installer command with ignore-app-preflights flag - execution failures cannot be bypassed
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--ignore-app-preflights",
		"--yes",
	)

	require.ErrorContains(t, err, "app preflights failed to run")
	require.ErrorContains(t, err, "exit code 1")

	preflightRunner.AssertExpectations(t)

	if !t.Failed() {
		t.Logf("Test passed: app preflights execution failure cannot be bypassed")
	}
}

func TestV3InstallHeadless_VeleroPlugin(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, hcli, nil)

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

	validateVeleroPlugin(t, hcli)

	if !t.Failed() {
		t.Logf("V3 headless velero plugin test passed")
	}
}

func TestV3Install_VeleroPlugin(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, hcli, nil)

	// Start installer in non-headless mode so API stays up; bypass prompts with --yes
	go func() {
		err := runInstallerCmd(
			"install",
			"--target", "linux",
			"--license", licenseFile,
			"--admin-console-password", "password123",
			"--yes",
		)
		if err != nil {
			t.Logf("installer exited with error: %v", err)
		}
	}()

	runV3Install(t, v3InstallArgs{
		managerPort:          30080,
		password:             "password123",
		isAirgap:             false,
		configValuesFile:     configFile,
		installationConfig:   apitypes.LinuxInstallationConfig{},
		ignoreHostPreflights: false,
		ignoreAppPreflights:  false,
	})

	validateVeleroPlugin(t, hcli)

	if !t.Failed() {
		t.Logf("V3 velero plugin test passed")
	}
}

func validateVeleroPlugin(t *testing.T, hcli *helm.MockClient) {
	t.Helper()

	// Validate velero addon is installed
	veleroOpts, found := isHelmReleaseInstalled(hcli, "velero")
	require.True(t, found, "velero helm release should be installed")

	// Validate initContainers exist and contain the plugin
	require.Contains(t, veleroOpts.Values, "initContainers", "initContainers should exist in Velero helm values")
	initContainersAny := veleroOpts.Values["initContainers"]
	require.NotNil(t, initContainersAny, "initContainers should not be nil")

	initContainers, ok := initContainersAny.([]any)
	require.True(t, ok, "initContainers should be a slice")
	require.NotEmpty(t, initContainers, "initContainers should not be empty")

	// Find the plugin container by name
	var pluginContainer map[string]any
	for _, container := range initContainers {
		if containerMap, ok := container.(map[string]any); ok {
			if name, _ := containerMap["name"].(string); name == "velero-plugin-example" {
				pluginContainer = containerMap
				break
			}
		}
	}

	require.NotNil(t, pluginContainer, "velero-plugin-example container should exist in initContainers")
	assert.Equal(t, "velero-plugin-example", pluginContainer["name"], "plugin container should have correct name")
	assert.Equal(t, "docker.io/library/velero-plugin-example:latest", pluginContainer["image"], "plugin container should have correct image")
	assert.Equal(t, "Always", pluginContainer["imagePullPolicy"], "plugin container should have correct imagePullPolicy")

	// Validate volumeMounts
	require.Contains(t, pluginContainer, "volumeMounts", "plugin container should have volumeMounts")
	volumeMountsAny := pluginContainer["volumeMounts"]
	require.NotNil(t, volumeMountsAny, "volumeMounts should not be nil")

	var volumeMounts []any
	switch v := volumeMountsAny.(type) {
	case []any:
		volumeMounts = v
	case []map[string]any:
		volumeMounts = make([]any, len(v))
		for i, vm := range v {
			volumeMounts[i] = vm
		}
	default:
		t.Fatalf("volumeMounts should be a slice, got %T", v)
	}

	require.NotEmpty(t, volumeMounts, "volumeMounts should not be empty")

	// Find the plugins volume mount
	var pluginsVolumeMount map[string]any
	for _, vm := range volumeMounts {
		if vmMap, ok := vm.(map[string]any); ok {
			if name, _ := vmMap["name"].(string); name == "plugins" {
				pluginsVolumeMount = vmMap
				break
			}
		}
	}

	require.NotNil(t, pluginsVolumeMount, "plugins volumeMount should exist")
	assert.Equal(t, "plugins", pluginsVolumeMount["name"], "volumeMount should have correct name")
	assert.Equal(t, "/target", pluginsVolumeMount["mountPath"], "volumeMount should have correct mountPath")
}

var (
	//go:embed assets/rendered-chart-preflight.yaml
	renderedChartPreflightData string

	//go:embed assets/kotskinds-config-values.yaml
	configValuesData string

	//go:embed assets/kotskinds-config-values-invalid.yaml
	configValuesInvalidData string
)

func setupV3Test(t *testing.T, hcli helm.Client, preflightRunner preflights.PreflightRunnerInterface) (string, string) {
	t.Helper()

	// Set ENABLE_V3 environment variable
	t.Setenv("ENABLE_V3", "1")

	// Ensure UI assets are available when starting API in non-headless tests
	prepareWebAssetsForTests(t)

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
		hcli = setupV3TestHelmClient()
	}

	if preflightRunner == nil {
		preflightRunner = setupV3TestPreflightsRunner()
	}

	// Initialize dryrun with mock ReplicatedAPIClient
	drFile := filepath.Join(t.TempDir(), "ec-dryrun.yaml")
	dryrun.Init(drFile, &dryrun.Client{
		ReplicatedAPIClient: &dryrun.ReplicatedAPIClient{
			License:      nil, // will return the same license that was passed in
			LicenseBytes: []byte(licenseData),
		},
		HelmClient:      hcli,
		PreflightRunner: preflightRunner,
	})
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetOutput(os.Stdout)

	// Create license file
	licenseFile := filepath.Join(t.TempDir(), "license.yaml")
	writeFile(t, licenseFile, licenseData)

	// Create config values file (required for headless)
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	createConfigValuesFile(t, configFile)

	return licenseFile, configFile
}

func setupV3TestHelmClient() *helm.MockClient {
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

func setupV3TestPreflightsRunner() *preflights.MockPreflightRunner {
	preflightRunner := &preflights.MockPreflightRunner{}
	preflightRunner.
		On("RunHostPreflights", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordHostPreflightSpec(args.Get(1).(*troubleshootv1beta2.HostPreflightSpec))
		}).
		Return(&apitypes.PreflightsOutput{
			Pass: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
		}, "", nil)
	preflightRunner.
		On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordAppPreflightSpec(args.Get(1).(*troubleshootv1beta2.PreflightSpec))
		}).
		Return(&apitypes.PreflightsOutput{
			Pass: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
		}, "", nil)
	return preflightRunner
}

func setupV3TestHostPreflightsRunnerWithFail() *preflights.MockPreflightRunner {
	opts := preflights.RunOptions{
		PreflightBinaryPath: "/var/lib/fake-app-slug/bin/kubectl-preflight",
		ProxySpec:           nil,
		ExtraPaths:          []string{"/var/lib/fake-app-slug/bin"},
	}

	preflightRunner := &preflights.MockPreflightRunner{}
	preflightRunner.
		On("RunHostPreflights", mock.Anything, mock.AnythingOfType("*v1beta2.HostPreflightSpec"), opts).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordHostPreflightSpec(args.Get(1).(*troubleshootv1beta2.HostPreflightSpec))
		}).
		Return(&apitypes.PreflightsOutput{
			Pass: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
			Fail: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check failed",
				},
			},
		}, "", nil)
	preflightRunner.
		On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).
		Maybe(). // Will run if bypass flag is set
		Run(func(args mock.Arguments) {
			dryrun.RecordAppPreflightSpec(args.Get(1).(*troubleshootv1beta2.PreflightSpec))
		}).
		Return(&apitypes.PreflightsOutput{
			Pass: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
		}, "", nil)
	return preflightRunner
}

func setupV3TestHostPreflightsRunnerFail() *preflights.MockPreflightRunner {
	opts := preflights.RunOptions{
		PreflightBinaryPath: "/var/lib/fake-app-slug/bin/kubectl-preflight",
		ProxySpec:           nil,
		ExtraPaths:          []string{"/var/lib/fake-app-slug/bin"},
	}

	preflightRunner := &preflights.MockPreflightRunner{}
	preflightRunner.
		On("RunHostPreflights", mock.Anything, mock.AnythingOfType("*v1beta2.HostPreflightSpec"), opts).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordHostPreflightSpec(args.Get(1).(*troubleshootv1beta2.HostPreflightSpec))
		}).
		Return(nil, "exit code 1", errors.New("preflights failed to run"))
	return preflightRunner
}

func setupV3TestAppPreflightsRunnerWithFail() *preflights.MockPreflightRunner {
	preflightRunner := &preflights.MockPreflightRunner{}
	preflightRunner.
		On("RunHostPreflights", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordHostPreflightSpec(args.Get(1).(*troubleshootv1beta2.HostPreflightSpec))
		}).
		Return(&apitypes.PreflightsOutput{
			Pass: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
		}, "", nil)
	preflightRunner.
		On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordAppPreflightSpec(args.Get(1).(*troubleshootv1beta2.PreflightSpec))
		}).
		Return(&apitypes.PreflightsOutput{
			Pass: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
			Fail: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check failed",
				},
			},
		}, "", nil)
	return preflightRunner
}

func setupV3TestAppPreflightsRunnerFail() *preflights.MockPreflightRunner {
	preflightRunner := &preflights.MockPreflightRunner{}
	preflightRunner.
		On("RunHostPreflights", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordHostPreflightSpec(args.Get(1).(*troubleshootv1beta2.HostPreflightSpec))
		}).
		Return(&apitypes.PreflightsOutput{
			Pass: []apitypes.PreflightsRecord{
				{
					Title:   "Test Check",
					Message: "Test check passed",
				},
			},
		}, "", nil)
	preflightRunner.
		On("RunAppPreflights", mock.Anything, mock.Anything, mock.Anything).
		Once().
		Run(func(args mock.Arguments) {
			dryrun.RecordAppPreflightSpec(args.Get(1).(*troubleshootv1beta2.PreflightSpec))
		}).
		Return(nil, "exit code 1", errors.New("app preflights failed to run"))
	return preflightRunner
}

// prepareWebAssetsForTests creates a minimal UI template and enables dev mode,
// so the web server can start without embedded assets.
func prepareWebAssetsForTests(t *testing.T) {
	t.Helper()
	t.Setenv("EC_DEV_ENV", "1")

	indexPath := "web/dist/index.html"
	if err := os.MkdirAll(filepath.Dir(indexPath), 0o755); err != nil {
		t.Fatalf("fail to create test web dist directory: %v", err)
	}

	// Minimal template that satisfies web.loadHTMLTemplate
	const indexHTML = "<!doctype html><html><head><meta charset=\"utf-8\"><title>{{ .Title }}</title></head><body><div id=\"root\"></div></body></html>\n"
	if err := os.WriteFile(indexPath, []byte(indexHTML), 0o644); err != nil {
		t.Fatalf("fail to write test index.html: %v", err)
	}
}

// createConfigValuesFile creates a config values file that passes validation
func createConfigValuesFile(t *testing.T, filename string) {
	t.Helper()

	writeFile(t, filename, configValuesData)
}

// createInvalidConfigValuesFile creates a config values file that fails validation
func createInvalidConfigValuesFile(t *testing.T, filename string) {
	t.Helper()

	writeFile(t, filename, configValuesInvalidData)
}

// v3InstallArgs are the configurable request arguments for the reusable non-headless flow
type v3InstallArgs struct {
	managerPort          int
	password             string
	isAirgap             bool
	configValuesFile     string
	installationConfig   apitypes.LinuxInstallationConfig
	ignoreHostPreflights bool
	ignoreAppPreflights  bool
}

// runV3Install executes the non-headless user flow against the API using the provided arguments.
func runV3Install(t *testing.T, args v3InstallArgs) {
	t.Helper()

	ctx := t.Context()

	// Wait for API be ready
	httpClient := insecureHTTPClient()
	waitForAPIReady(t, httpClient, fmt.Sprintf("https://localhost:%d/api/health", args.managerPort))

	// Build API client and authenticate
	c := apiclient.New(fmt.Sprintf("https://localhost:%d", args.managerPort), apiclient.WithHTTPClient(httpClient))
	require.NoError(t, c.Authenticate(ctx, args.password))

	// Configure application with config values
	kcv, err := helpers.ParseConfigValues(args.configValuesFile)
	require.NoError(t, err, "failed to parse config values file")
	appConfigValues := apitypes.ConvertToAppConfigValues(kcv)
	_, err = c.PatchLinuxInstallAppConfigValues(ctx, appConfigValues)
	require.NoError(t, err)

	// Configure installation settings and wait until completed
	_, err = c.ConfigureLinuxInstallation(ctx, args.installationConfig)
	require.NoError(t, err)
	assertEventuallySucceeded(t, "installation configuration", func() (apitypes.State, string, error) {
		st, err := c.GetLinuxInstallationStatus(ctx)
		if err != nil {
			return "", "", err
		}
		return st.State, st.Description, nil
	})

	// Run host preflights and wait for completion
	_, err = c.RunLinuxInstallHostPreflights(ctx)
	require.NoError(t, err)
	assertEventuallySucceeded(t, "host preflights", func() (apitypes.State, string, error) {
		st, err := c.GetLinuxInstallHostPreflightsStatus(ctx)
		if err != nil {
			return "", "", err
		}
		return st.Status.State, st.Status.Description, nil
	})

	// Setup infrastructure and wait for completion
	_, err = c.SetupLinuxInfra(ctx, args.ignoreHostPreflights)
	require.NoError(t, err)
	assertEventuallySucceeded(t, "infrastructure setup", func() (apitypes.State, string, error) {
		st, err := c.GetLinuxInfraStatus(ctx)
		if err != nil {
			return "", "", err
		}
		return st.Status.State, st.Status.Description, nil
	})

	// If airgap, process airgap and wait for completion
	if args.isAirgap {
		_, err = c.ProcessLinuxAirgap(ctx)
		require.NoError(t, err)
		assertEventuallySucceeded(t, "airgap processing", func() (apitypes.State, string, error) {
			st, err := c.GetLinuxAirgapStatus(ctx)
			if err != nil {
				return "", "", err
			}
			return st.Status.State, st.Status.Description, nil
		})
	}

	// Run app preflights and wait for completion
	_, err = c.RunLinuxInstallAppPreflights(ctx)
	require.NoError(t, err)
	assertEventuallySucceeded(t, "application preflights", func() (apitypes.State, string, error) {
		st, err := c.GetLinuxInstallAppPreflightsStatus(ctx)
		if err != nil {
			return "", "", err
		}
		return st.Status.State, st.Status.Description, nil
	})

	// Install application and wait for completion
	_, err = c.InstallLinuxApp(ctx, args.ignoreAppPreflights)
	require.NoError(t, err)
	assertEventuallySucceeded(t, "application install", func() (apitypes.State, string, error) {
		st, err := c.GetLinuxAppInstallStatus(ctx)
		if err != nil {
			return "", "", err
		}
		return st.Status.State, st.Status.Description, nil
	})

	// Dump and load dryrun output for inspection/assertions
	require.NoError(t, dryrun.Dump(), "fail to dump dryrun output")
}
