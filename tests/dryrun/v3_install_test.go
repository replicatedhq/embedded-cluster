package dryrun

import (
	_ "embed"
	"os"
	"path/filepath"
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

	// Validate Installation object has correct AirGap settings
	in, err := kubeutils.GetLatestInstallation(t.Context(), kcli)
	require.NoError(t, err, "failed to get latest installation")
	assert.False(t, in.Spec.AirGap, "Installation.Spec.AirGap should be false for online installations")
	assert.Equal(t, int64(0), in.Spec.AirgapUncompressedSize, "Installation.Spec.AirgapUncompressedSize should be 0 for online installations")

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

	// Validate that embedded-cluster-path-usage collector is NOT present for online installations
	// This collector is only needed for airgap installations to check disk space for bundle processing
	for _, analyzer := range dr.HostPreflightSpec.Analyzers {
		if analyzer.DiskUsage != nil && analyzer.DiskUsage.CheckName == "Airgap Storage Space" {
			assert.Fail(t, "Airgap Storage Space analyzer should not be present in online installations")
		}
	}

	// Validate that registry addon is NOT installed for online installations
	_, found := isHelmReleaseInstalled(hcli, "docker-registry")
	require.False(t, found, "docker-registry helm release should not be installed")

	// Validate that isAirgap helm value is set to false in admin console chart
	adminConsoleOpts, found := isHelmReleaseInstalled(hcli, "admin-console")
	require.True(t, found, "admin-console helm release should be installed")
	assertHelmValues(t, adminConsoleOpts.Values, map[string]interface{}{
		"isAirgap": false,
	})

	// Validate that isAirgap helm value is not set in embedded-cluster-operator chart for online installations
	operatorOpts, found := isHelmReleaseInstalled(hcli, "embedded-cluster-operator")
	require.True(t, found, "embedded-cluster-operator helm release should be installed")
	_, hasIsAirgap := operatorOpts.Values["isAirgap"]
	assert.False(t, hasIsAirgap, "embedded-cluster-operator should not have isAirgap helm value for online installations")

	// Validate that registry-creds secret is NOT created for online installations
	assertSecretNotExists(t, kcli, "registry-creds", adminConsoleNamespace)

	t.Logf("Test passed: headless online installation does not create registry addon or registry-creds secret")
}

func TestV3InstallHeadless_HappyPathAirgap(t *testing.T) {
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
		"--airgap-bundle", airgapBundleFile(t),
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
	assertHelmValues(t, adminConsoleOpts.Values, map[string]interface{}{
		"isAirgap": true,
	})

	// Validate that isAirgap helm value is set to "true" in embedded-cluster-operator chart for airgap installations
	operatorOpts, found := isHelmReleaseInstalled(hcli, "embedded-cluster-operator")
	require.True(t, found, "embedded-cluster-operator helm release should be installed")
	assertHelmValues(t, operatorOpts.Values, map[string]interface{}{
		"isAirgap": "true",
	})

	// Validate that registry-creds secret IS created for airgap installations
	assertSecretExists(t, kcli, "registry-creds", adminConsoleNamespace)

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
