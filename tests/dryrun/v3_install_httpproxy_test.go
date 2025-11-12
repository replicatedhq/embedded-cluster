package dryrun

import (
	"os"
	"regexp"
	"testing"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/dryrun/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestV3InstallHeadless_HTTPProxy(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, setupV3TestOpts{
		helmClient: hcli,
	})

	hostCABundle := findHostCABundle(t)

	// Run installer command with headless flag and required arguments
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
		"--http-proxy", "http://localhost:3128",
		"--https-proxy", "https://localhost:3128",
		"--no-proxy", "localhost,127.0.0.1,10.0.0.0/8",
	)

	require.NoError(t, err, "headless installation should succeed")

	dr, err := dryrun.Load()
	if err != nil {
		t.Fatalf("fail to unmarshal dryrun output: %v", err)
	}

	validateHTTPProxyWithCABundle(t, dr, hcli, hostCABundle)

	if !t.Failed() {
		t.Logf("Test passed: HTTP proxy configuration correctly propagates to Installation object, environment variables, and Helm charts")
	}
}

func TestV3Install_HTTPProxy(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, setupV3TestOpts{
		helmClient: hcli,
	})

	hostCABundle := findHostCABundle(t)

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

	validateHTTPProxyWithCABundle(t, dr, hcli, hostCABundle)

	if !t.Failed() {
		t.Logf("Test passed: HTTP proxy configuration correctly propagates to Installation object, environment variables, and Helm charts")
	}
}

func TestV3InstallHeadless_HTTPProxyPrecedence(t *testing.T) {
	hcli := setupV3TestHelmClient()
	licenseFile, configFile := setupV3Test(t, setupV3TestOpts{
		helmClient: hcli,
	})

	// Set HTTP proxy environment variables that should be overridden by flags
	t.Setenv("HTTP_PROXY", "http://localhost-env:3128")
	t.Setenv("HTTPS_PROXY", "https://localhost-env:3128")
	t.Setenv("NO_PROXY", "localhost-env,127.0.0.1,10.0.0.0/8")

	// Run installer command with headless flag and required arguments
	err := runInstallerCmd(
		"install",
		"--headless",
		"--target", "linux",
		"--license", licenseFile,
		"--config-values", configFile,
		"--admin-console-password", "password123",
		"--yes",
		"--http-proxy", "http://localhost:3128",
		"--https-proxy", "https://localhost:3128",
		"--no-proxy", "localhost,127.0.0.1,10.0.0.0/8",
	)

	require.NoError(t, err, "headless installation should succeed")

	dr, err := dryrun.Load()
	if err != nil {
		t.Fatalf("fail to unmarshal dryrun output: %v", err)
	}

	validateHTTPProxy(t, dr, hcli)

	if !t.Failed() {
		t.Logf("Test passed: CLI flags/API config correctly take precedence over environment variables")
	}
}

func validateHTTPProxy(t *testing.T, dr *types.DryRun, hcli *helm.MockClient) {
	t.Helper()

	expectedHTTPProxy := "http://localhost:3128"
	expectedHTTPSProxy := "https://localhost:3128"
	expectedNoProxy := "localhost,127.0.0.1,10.0.0.0/8"

	// Validate Installation object
	kcli, err := dr.KubeClient()
	require.NoError(t, err, "failed to get kube client")

	in, err := kubeutils.GetLatestInstallation(t.Context(), kcli)
	require.NoError(t, err, "failed to get latest installation")

	require.NotNil(t, in.Spec.RuntimeConfig, "RuntimeConfig should not be nil")
	require.NotNil(t, in.Spec.RuntimeConfig.Proxy, "Proxy config should not be nil")
	assert.Equal(t, expectedHTTPProxy, in.Spec.RuntimeConfig.Proxy.HTTPProxy, "Installation HTTPProxy should match")
	assert.Equal(t, expectedHTTPSProxy, in.Spec.RuntimeConfig.Proxy.HTTPSProxy, "Installation HTTPSProxy should match")
	assert.Contains(t, in.Spec.RuntimeConfig.Proxy.NoProxy, expectedNoProxy, "Installation NoProxy should contain expected values")

	// Validate OS environment variables
	assert.Equal(t, expectedHTTPProxy, dr.OSEnv["HTTP_PROXY"], "OS env HTTP_PROXY should match")
	assert.Equal(t, expectedHTTPSProxy, dr.OSEnv["HTTPS_PROXY"], "OS env HTTPS_PROXY should match")
	assert.Contains(t, dr.OSEnv["NO_PROXY"], expectedNoProxy, "OS env NO_PROXY should contain expected values")

	// Validate http-proxy.conf systemd file
	proxyConfPath := "/etc/systemd/system/k0scontroller.service.d/http-proxy.conf"
	proxyConfContent, err := os.ReadFile(proxyConfPath)
	require.NoError(t, err, "failed to read http-proxy.conf file")
	proxyConfContentStr := string(proxyConfContent)
	assert.Contains(t, proxyConfContentStr, `Environment="HTTP_PROXY=`+expectedHTTPProxy+`"`, "http-proxy.conf should contain HTTP_PROXY")
	assert.Contains(t, proxyConfContentStr, `Environment="HTTPS_PROXY=`+expectedHTTPSProxy+`"`, "http-proxy.conf should contain HTTPS_PROXY")
	assert.Contains(t, proxyConfContentStr, `Environment="NO_PROXY=`, "http-proxy.conf should contain NO_PROXY")
	assert.Contains(t, proxyConfContentStr, expectedNoProxy, "http-proxy.conf NO_PROXY should contain expected values")

	// Validate proxy settings in operator chart
	operatorOpts, foundOperator := isHelmReleaseInstalled(hcli, "embedded-cluster-operator")
	require.True(t, foundOperator, "embedded-cluster-operator helm release should be installed")

	httpProxyValue, ok := getHelmExtraEnvValue(t, operatorOpts.Values, "extraEnv", "HTTP_PROXY")
	require.True(t, ok, "HTTP_PROXY env var not found in operator opts")
	assert.Equal(t, expectedHTTPProxy, httpProxyValue, "operator HTTP_PROXY should match")

	httpsProxyValue, ok := getHelmExtraEnvValue(t, operatorOpts.Values, "extraEnv", "HTTPS_PROXY")
	require.True(t, ok, "HTTPS_PROXY env var not found in operator opts")
	assert.Equal(t, expectedHTTPSProxy, httpsProxyValue, "operator HTTPS_PROXY should match")

	noProxyValue, ok := getHelmExtraEnvValue(t, operatorOpts.Values, "extraEnv", "NO_PROXY")
	require.True(t, ok, "NO_PROXY env var not found in operator opts")
	assert.Contains(t, noProxyValue, expectedNoProxy, "operator NO_PROXY should contain expected values")

	// Validate proxy settings in velero chart
	veleroOpts, foundVelero := isHelmReleaseInstalled(hcli, "velero")
	require.True(t, foundVelero, "velero helm release should be installed")

	httpProxyValue, ok = getHelmExtraEnvValue(t, veleroOpts.Values, "configuration.extraEnvVars", "HTTP_PROXY")
	require.True(t, ok, "HTTP_PROXY env var not found in velero opts")
	assert.Equal(t, expectedHTTPProxy, httpProxyValue, "velero HTTP_PROXY should match")

	httpsProxyValue, ok = getHelmExtraEnvValue(t, veleroOpts.Values, "configuration.extraEnvVars", "HTTPS_PROXY")
	require.True(t, ok, "HTTPS_PROXY env var not found in velero opts")
	assert.Equal(t, expectedHTTPSProxy, httpsProxyValue, "velero HTTPS_PROXY should match")

	noProxyValue, ok = getHelmExtraEnvValue(t, veleroOpts.Values, "configuration.extraEnvVars", "NO_PROXY")
	require.True(t, ok, "NO_PROXY env var not found in velero opts")
	assert.Contains(t, noProxyValue, expectedNoProxy, "velero NO_PROXY should contain expected values")

	// Validate proxy settings in admin console chart
	adminConsoleOpts, foundAdminConsole := isHelmReleaseInstalled(hcli, "admin-console")
	require.True(t, foundAdminConsole, "admin-console helm release should be installed")

	httpProxyValue, ok = getHelmExtraEnvValue(t, adminConsoleOpts.Values, "extraEnv", "HTTP_PROXY")
	require.True(t, ok, "HTTP_PROXY env var not found in admin console opts")
	assert.Equal(t, expectedHTTPProxy, httpProxyValue, "admin console HTTP_PROXY should match")

	httpsProxyValue, ok = getHelmExtraEnvValue(t, adminConsoleOpts.Values, "extraEnv", "HTTPS_PROXY")
	require.True(t, ok, "HTTPS_PROXY env var not found in admin console opts")
	assert.Equal(t, expectedHTTPSProxy, httpsProxyValue, "admin console HTTPS_PROXY should match")

	noProxyValue, ok = getHelmExtraEnvValue(t, adminConsoleOpts.Values, "extraEnv", "NO_PROXY")
	require.True(t, ok, "NO_PROXY env var not found in admin console opts")
	assert.Contains(t, noProxyValue, expectedNoProxy, "admin console NO_PROXY should contain expected values")

	// Validate host preflight spec includes proxy settings
	assertCollectors(t, dr.HostPreflightSpec.Collectors, map[string]struct {
		match    func(*troubleshootv1beta2.HostCollect) bool
		validate func(*troubleshootv1beta2.HostCollect)
	}{
		"http-replicated-app": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.HTTP != nil && hc.HTTP.CollectorName == "http-replicated-app"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, expectedHTTPSProxy, hc.HTTP.Get.Proxy, "http-replicated-app collector should use HTTPS proxy")
			},
		},
		"http-proxy-replicated-com": {
			match: func(hc *troubleshootv1beta2.HostCollect) bool {
				return hc.HTTP != nil && hc.HTTP.CollectorName == "http-proxy-replicated-com"
			},
			validate: func(hc *troubleshootv1beta2.HostCollect) {
				assert.Equal(t, expectedHTTPSProxy, hc.HTTP.Get.Proxy, "http-proxy-replicated-com collector should use HTTPS proxy")
			},
		},
	})

	// Validate proxy connection attempts in log output (if captured)
	// Note: V3 tests may not capture log output in the same way as non-V3 tests
	if dr.LogOutput != "" {
		re := regexp.MustCompile(`WARNING: Failed to check for newer app versions.*: proxyconnect tcp: dial tcp .*:3128: connect: connection refused`)
		assert.Regexp(t, re, dr.LogOutput, "log output should show proxy connection attempt")
	}

	// Verify metrics were captured
	assert.NotEmpty(t, dr.Metrics, "metrics should be captured during installation")
}

func validateHTTPProxyWithCABundle(t *testing.T, dr *types.DryRun, hcli *helm.MockClient, hostCABundle string) {
	t.Helper()

	// First validate all standard proxy settings
	validateHTTPProxy(t, dr, hcli)

	// Get dynamic kotsadm namespace
	kcli, err := dr.KubeClient()
	require.NoError(t, err)
	expectedNamespace, err := runtimeconfig.KotsadmNamespace(t.Context(), kcli)
	require.NoError(t, err)

	// Validate Installation object has CA bundle path
	in, err := kubeutils.GetLatestInstallation(t.Context(), kcli)
	require.NoError(t, err)
	assert.Equal(t, hostCABundle, in.Spec.RuntimeConfig.HostCABundlePath, "Installation should have HostCABundlePath set")

	// Validate kotsadm-private-cas ConfigMap exists and contains CA bundle
	var caConfigMap corev1.ConfigMap
	err = kcli.Get(t.Context(), client.ObjectKey{Namespace: expectedNamespace, Name: "kotsadm-private-cas"}, &caConfigMap)
	require.NoError(t, err, "kotsadm-private-cas configmap should exist")
	assert.Contains(t, caConfigMap.Data, "ca_0.crt", "kotsadm-private-cas configmap should contain ca_0.crt")

	// Validate operator chart has SSL_CERT_DIR and PRIVATE_CA_BUNDLE_PATH
	operatorOpts, foundOperator := isHelmReleaseInstalled(hcli, "embedded-cluster-operator")
	require.True(t, foundOperator, "embedded-cluster-operator helm release should be installed")

	sslCertDir, ok := getHelmExtraEnvValue(t, operatorOpts.Values, "extraEnv", "SSL_CERT_DIR")
	require.True(t, ok, "SSL_CERT_DIR env var not found in operator opts")
	assert.Equal(t, "/certs", sslCertDir, "operator SSL_CERT_DIR should be /certs")

	caBundlePath, ok := getHelmExtraEnvValue(t, operatorOpts.Values, "extraEnv", "PRIVATE_CA_BUNDLE_PATH")
	require.True(t, ok, "PRIVATE_CA_BUNDLE_PATH env var not found in operator opts")
	assert.Equal(t, "/certs/ca-certificates.crt", caBundlePath, "operator PRIVATE_CA_BUNDLE_PATH should be /certs/ca-certificates.crt")

	// Validate operator has CA bundle volume and volume mount
	assertHelmValues(t, operatorOpts.Values, map[string]any{
		"extraVolumes": []map[string]any{{
			"name": "host-ca-bundle",
			"hostPath": map[string]any{
				"path": hostCABundle,
				"type": "FileOrCreate",
			}},
		},
		"extraVolumeMounts": []map[string]any{{
			"mountPath": "/certs/ca-certificates.crt",
			"name":      "host-ca-bundle",
		}},
	})

	// Validate velero chart has SSL_CERT_DIR and CA bundle volumes
	veleroOpts, foundVelero := isHelmReleaseInstalled(hcli, "velero")
	require.True(t, foundVelero, "velero helm release should be installed")

	sslCertDir, ok = getHelmExtraEnvValue(t, veleroOpts.Values, "configuration.extraEnvVars", "SSL_CERT_DIR")
	require.True(t, ok, "SSL_CERT_DIR env var not found in velero opts")
	assert.Equal(t, "/certs", sslCertDir, "velero SSL_CERT_DIR should be /certs")

	assertHelmValues(t, veleroOpts.Values, map[string]any{
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

	// Validate admin console chart has SSL_CERT_CONFIGMAP and SSL_CERT_DIR
	adminConsoleOpts, foundAdminConsole := isHelmReleaseInstalled(hcli, "admin-console")
	require.True(t, foundAdminConsole, "admin-console helm release should be installed")

	sslCertConfigMap, ok := getHelmExtraEnvValue(t, adminConsoleOpts.Values, "extraEnv", "SSL_CERT_CONFIGMAP")
	require.True(t, ok, "SSL_CERT_CONFIGMAP env var not found in admin console opts")
	assert.Equal(t, "kotsadm-private-cas", sslCertConfigMap, "admin console SSL_CERT_CONFIGMAP should be kotsadm-private-cas")

	sslCertDir, ok = getHelmExtraEnvValue(t, adminConsoleOpts.Values, "extraEnv", "SSL_CERT_DIR")
	require.True(t, ok, "SSL_CERT_DIR env var not found in admin console opts")
	assert.Equal(t, "/certs", sslCertDir, "admin console SSL_CERT_DIR should be /certs")

	// Validate admin console has CA bundle volume and volume mount
	extraVolumes, err := helm.GetValue(adminConsoleOpts.Values, "extraVolumes")
	require.NoError(t, err, "extraVolumes should exist in admin console opts")
	foundVolume := false
	for _, vol := range extraVolumes.([]any) {
		volMap := vol.(map[string]any)
		if volMap["name"] == "host-ca-bundle" {
			foundVolume = true
			hostPath := volMap["hostPath"].(map[string]any)
			assert.Equal(t, hostCABundle, hostPath["path"])
			assert.Equal(t, "FileOrCreate", hostPath["type"])
		}
	}
	assert.True(t, foundVolume, "admin console should have host-ca-bundle volume")

	extraVolumeMounts, err := helm.GetValue(adminConsoleOpts.Values, "extraVolumeMounts")
	require.NoError(t, err, "extraVolumeMounts should exist in admin console opts")
	foundMount := false
	for _, mount := range extraVolumeMounts.([]any) {
		mountMap := mount.(map[string]any)
		if mountMap["name"] == "host-ca-bundle" {
			foundMount = true
			assert.Equal(t, "/certs/ca-certificates.crt", mountMap["mountPath"])
		}
	}
	assert.True(t, foundMount, "admin console should have host-ca-bundle volume mount")
}
