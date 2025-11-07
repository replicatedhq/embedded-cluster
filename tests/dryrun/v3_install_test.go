package dryrun

import (
	"crypto/x509"
	_ "embed"
	"encoding/pem"
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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestV3InstallHeadless_HappyPathOnline(t *testing.T) {
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

	// Expect the command to fail with the specific error message
	require.NoError(t, err, "headless installation should succeed")

	// PersistentPostRunE is not called when the command fails, so we need to dump the dryrun output manually
	require.NoError(t, dryrun.Dump(), "fail to dump dryrun output")

	t.Logf("Test passed: headless installation correctly returns not implemented error")
}

func TestV3InstallHeadless_HappyPathAirgap(t *testing.T) {
	licenseFile, configFile := setupV3HeadlessTest(t, nil)

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

	// Expect the command to fail with the specific error message
	require.NoError(t, err, "headless installation should succeed")

	// PersistentPostRunE is not called when the command fails, so we need to dump the dryrun output manually
	require.NoError(t, dryrun.Dump(), "fail to dump dryrun output")

	t.Logf("Test passed: headless installation correctly returns not implemented error")
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

	// Expect the command to fail with the specific error message
	require.NoError(t, err, "headless installation should succeed")

	// PersistentPostRunE is not called when the command fails, so we need to dump the dryrun output manually
	require.NoError(t, dryrun.Dump(), "fail to dump dryrun output")

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

	// PersistentPostRunE is not called when the command fails, so we need to dump the dryrun output manually
	require.NoError(t, dryrun.Dump(), "fail to dump dryrun output")

	dr, err := dryrun.Load()
	if err != nil {
		t.Fatalf("fail to unmarshal dryrun output: %v", err)
	}

	// --- validate k0s cluster config --- //
	k0sConfig := readK0sConfig(t)

	assert.Equal(t, "10.2.0.0/17", k0sConfig.Spec.Network.PodCIDR)
	assert.Equal(t, "10.2.128.0/17", k0sConfig.Spec.Network.ServiceCIDR)

	// --- validate installation object --- //
	kcli, err := dr.KubeClient()
	require.NoError(t, err, "get kube client")

	in, err := kubeutils.GetLatestInstallation(t.Context(), kcli)
	if err != nil {
		t.Fatalf("failed to get latest installation: %v", err)
	}

	assert.Equal(t, "10.2.0.0/17", in.Spec.RuntimeConfig.Network.PodCIDR)
	assert.Equal(t, "10.2.128.0/17", in.Spec.RuntimeConfig.Network.ServiceCIDR)

	// --- validate registry --- //
	expectedRegistryIP := "10.2.128.11" // lower band index 10

	var registrySecret corev1.Secret
	err = kcli.Get(t.Context(), types.NamespacedName{Name: "registry-tls", Namespace: "registry"}, &registrySecret)
	require.NoError(t, err, "get registry TLS secret")

	certData, ok := registrySecret.StringData["tls.crt"]
	require.True(t, ok, "registry TLS secret must contain tls.crt")

	// parse certificate and verify it contains the expected IP
	block, _ := pem.Decode([]byte(certData))
	require.NotNil(t, block, "failed to decode certificate PEM")
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err, "failed to parse certificate")

	// check if certificate contains the expected registry IP (convert to strings for comparison)
	ipStrings := make([]string, len(cert.IPAddresses))
	for i, ip := range cert.IPAddresses {
		ipStrings[i] = ip.String()
	}
	assert.Contains(t, ipStrings, expectedRegistryIP, "certificate should contain registry IP %s, found IPs: %v", expectedRegistryIP, cert.IPAddresses)

	// --- validate cidrs in NO_PROXY OS env var --- //
	noProxy := dr.OSEnv["NO_PROXY"]
	assert.Contains(t, noProxy, "10.2.0.0/17")
	assert.Contains(t, noProxy, "10.2.128.0/17")

	// --- validate cidrs in NO_PROXY Helm value of operator chart --- //
	var operatorOpts helm.InstallOptions
	foundOperator := false
	for _, call := range hcli.Calls {
		if call.Method == "Install" {
			opts := call.Arguments[1].(helm.InstallOptions)
			if opts.ReleaseName == "embedded-cluster-operator" {
				operatorOpts = opts
				foundOperator = true
				break
			}
		}
	}
	require.True(t, foundOperator, "embedded-cluster-operator install call not found")

	found := false
	for _, env := range operatorOpts.Values["extraEnv"].([]map[string]any) {
		if env["name"] == "NO_PROXY" {
			noProxyValue, ok := env["value"].(string)
			require.True(t, ok, "NO_PROXY value should be a string")
			assert.Contains(t, noProxyValue, "10.2.0.0/17", "NO_PROXY should contain pod CIDR")
			assert.Contains(t, noProxyValue, "10.2.128.0/17", "NO_PROXY should contain service CIDR")
			found = true
		}
	}
	assert.True(t, found, "NO_PROXY env var not found in operator opts")

	// --- validate custom cidr was used for registry service cluster IP --- //
	var registryOpts helm.InstallOptions
	foundRegistry := false
	for _, call := range hcli.Calls {
		if call.Method == "Install" {
			opts := call.Arguments[1].(helm.InstallOptions)
			if opts.ReleaseName == "docker-registry" {
				registryOpts = opts
				foundRegistry = true
				break
			}
		}
	}
	require.True(t, foundRegistry, "docker-registry install call not found")

	assertHelmValues(t, registryOpts.Values, map[string]any{
		"service.clusterIP": expectedRegistryIP,
	})

	// --- validate cidrs in NO_PROXY Helm value of velero chart --- //
	var veleroOpts helm.InstallOptions
	foundVelero := false
	for _, call := range hcli.Calls {
		if call.Method == "Install" {
			opts := call.Arguments[1].(helm.InstallOptions)
			if opts.ReleaseName == "velero" {
				veleroOpts = opts
				foundVelero = true
				break
			}
		}
	}
	require.True(t, foundVelero, "velero install call not found")

	found = false
	extraEnvVars, err := helm.GetValue(veleroOpts.Values, "configuration.extraEnvVars")
	require.NoError(t, err)

	for _, env := range extraEnvVars.([]map[string]any) {
		if env["name"] == "NO_PROXY" {
			noProxyValue, ok := env["value"].(string)
			require.True(t, ok, "NO_PROXY value should be a string")
			assert.Contains(t, noProxyValue, "10.2.0.0/17", "NO_PROXY should contain pod CIDR")
			assert.Contains(t, noProxyValue, "10.2.128.0/17", "NO_PROXY should contain service CIDR")
			found = true
		}
	}
	assert.True(t, found, "NO_PROXY env var not found in velero opts")

	// --- validate cidrs in NO_PROXY Helm value of admin console chart --- //
	var adminConsoleOpts helm.InstallOptions
	foundAdminConsole := false
	for _, call := range hcli.Calls {
		if call.Method == "Install" {
			opts := call.Arguments[1].(helm.InstallOptions)
			if opts.ReleaseName == "admin-console" {
				adminConsoleOpts = opts
				foundAdminConsole = true
				break
			}
		}
	}
	require.True(t, foundAdminConsole, "admin-console install call not found")

	found = false
	for _, env := range adminConsoleOpts.Values["extraEnv"].([]map[string]any) {
		if env["name"] == "NO_PROXY" {
			noProxyValue, ok := env["value"].(string)
			require.True(t, ok, "NO_PROXY value should be a string")
			assert.Contains(t, noProxyValue, "10.2.0.0/17", "NO_PROXY should contain pod CIDR")
			assert.Contains(t, noProxyValue, "10.2.128.0/17", "NO_PROXY should contain service CIDR")
			found = true
		}
	}
	assert.True(t, found, "NO_PROXY env var not found in admin console opts")

	// --- validate custom cidrs in NO_PROXY in http-proxy.conf file --- //
	proxyConfPath := "/etc/systemd/system/k0scontroller.service.d/http-proxy.conf"
	proxyConfContent, err := os.ReadFile(proxyConfPath)
	require.NoError(t, err, "failed to read http-proxy.conf file")
	proxyConfContentStr := string(proxyConfContent)
	assert.Contains(t, proxyConfContentStr, `Environment="NO_PROXY=`, "http-proxy.conf should contain NO_PROXY environment variable")
	assert.Contains(t, proxyConfContentStr, "10.2.0.0/17", "http-proxy.conf NO_PROXY should contain pod CIDR")
	assert.Contains(t, proxyConfContentStr, "10.2.128.0/17", "http-proxy.conf NO_PROXY should contain service CIDR")

	// --- validate commands include firewall rules --- //
	assertCommands(t, dr.Commands,
		[]any{
			"firewall-cmd --info-zone ec-net",
			"firewall-cmd --add-source 10.2.0.0/17 --permanent --zone ec-net",
			"firewall-cmd --add-source 10.2.128.0/17 --permanent --zone ec-net",
			"firewall-cmd --reload",
		},
		false,
	)

	// --- validate host preflight spec has correct CIDR substitutions --- //
	// When --cidr is used, the GlobalCIDR is set and Pod/Service CIDR collectors are excluded
	assertCollectors(t, dr.HostPreflightSpec.Collectors, map[string]struct {
		match    func(*troubleshootv1beta2.HostCollect) bool
		validate func(*troubleshootv1beta2.HostCollect)
	}{
		"CIDR": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.SubnetAvailable != nil && hc.SubnetAvailable.CollectorName == "CIDR"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, "10.2.0.0/16", hc.SubnetAvailable.CIDRRangeAlloc, "Global CIDR should be correctly substituted")
			},
		},
		"Network Namespace Connectivity": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.NetworkNamespaceConnectivity != nil && hc.NetworkNamespaceConnectivity.CollectorName == "check-network-namespace-connectivity"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, "10.2.0.0/17", hc.NetworkNamespaceConnectivity.FromCIDR, "FromCIDR should be pod CIDR")
				assert.Equal(t, "10.2.128.0/17", hc.NetworkNamespaceConnectivity.ToCIDR, "ToCIDR should be service CIDR")
			},
		},
	})

	t.Logf("Test passed: custom CIDR correctly propagates to all external dependencies")
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
