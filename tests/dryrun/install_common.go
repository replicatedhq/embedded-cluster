package dryrun

import (
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"os"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/dryrun/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func validateCustomCIDR(t *testing.T, dr *types.DryRun, hcli *helm.MockClient) {
	t.Helper()

	// Validate k0s cluster config
	k0sConfig := readK0sConfig(t)

	assert.Equal(t, "10.2.0.0/17", k0sConfig.Spec.Network.PodCIDR)
	assert.Equal(t, "10.2.128.0/17", k0sConfig.Spec.Network.ServiceCIDR)

	// Validate Installation object
	kcli, err := dr.KubeClient()
	require.NoError(t, err, "failed to get kube client")

	in, err := kubeutils.GetLatestInstallation(t.Context(), kcli)
	require.NoError(t, err, "failed to get latest installation")

	assert.Equal(t, "10.2.0.0/17", in.Spec.RuntimeConfig.Network.PodCIDR)
	assert.Equal(t, "10.2.128.0/17", in.Spec.RuntimeConfig.Network.ServiceCIDR)

	// Validate registry
	// Lower band index 10
	expectedRegistryIP := "10.2.128.11"

	var registrySecret corev1.Secret
	err = kcli.Get(t.Context(), client.ObjectKey{Name: "registry-tls", Namespace: "registry"}, &registrySecret)
	require.NoError(t, err, "failed to get registry TLS secret")

	certData, ok := registrySecret.StringData["tls.crt"]
	require.True(t, ok, "registry TLS secret must contain tls.crt")

	// Parse certificate and verify it contains the expected IP
	block, _ := pem.Decode([]byte(certData))
	require.NotNil(t, block, "failed to decode certificate PEM")
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err, "failed to parse certificate")

	// Check if certificate contains the expected registry IP (convert to strings for comparison)
	ipStrings := make([]string, len(cert.IPAddresses))
	for i, ip := range cert.IPAddresses {
		ipStrings[i] = ip.String()
	}
	assert.Contains(t, ipStrings, expectedRegistryIP, "certificate should contain registry IP %s, found IPs: %v", expectedRegistryIP, cert.IPAddresses)

	// Validate CIDRs in NO_PROXY OS env var
	noProxy := dr.OSEnv["NO_PROXY"]
	assert.Contains(t, noProxy, "10.2.0.0/17")
	assert.Contains(t, noProxy, "10.2.128.0/17")

	// Validate CIDRs in NO_PROXY Helm value of operator chart
	operatorOpts, foundOperator := isHelmReleaseInstalled(hcli, "embedded-cluster-operator")
	require.True(t, foundOperator, "embedded-cluster-operator helm release should be installed")

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

	// Validate custom CIDR was used for registry service cluster IP
	registryOpts, foundRegistry := isHelmReleaseInstalled(hcli, "docker-registry")
	require.True(t, foundRegistry, "docker-registry helm release should be installed")

	assertHelmValues(t, registryOpts.Values, map[string]any{
		"service.clusterIP": expectedRegistryIP,
	})

	// Validate CIDRs in NO_PROXY Helm value of velero chart
	veleroOpts, foundVelero := isHelmReleaseInstalled(hcli, "velero")
	require.True(t, foundVelero, "velero helm release should be installed")

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

	// Validate CIDRs in NO_PROXY Helm value of admin console chart
	adminConsoleOpts, foundAdminConsole := isHelmReleaseInstalled(hcli, "admin-console")
	require.True(t, foundAdminConsole, "admin-console helm release should be installed")

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

	// Validate custom CIDRs in NO_PROXY in http-proxy.conf file
	proxyConfPath := "/etc/systemd/system/k0scontroller.service.d/http-proxy.conf"
	proxyConfContent, err := os.ReadFile(proxyConfPath)
	require.NoError(t, err, "failed to read http-proxy.conf file")
	proxyConfContentStr := string(proxyConfContent)
	assert.Contains(t, proxyConfContentStr, `Environment="NO_PROXY=`, "http-proxy.conf should contain NO_PROXY environment variable")
	assert.Contains(t, proxyConfContentStr, "10.2.0.0/17", "http-proxy.conf NO_PROXY should contain pod CIDR")
	assert.Contains(t, proxyConfContentStr, "10.2.128.0/17", "http-proxy.conf NO_PROXY should contain service CIDR")

	// Validate commands include firewall rules
	assertCommands(t, dr.Commands,
		[]any{
			"firewall-cmd --info-zone ec-net",
			"firewall-cmd --add-source 10.2.0.0/17 --permanent --zone ec-net",
			"firewall-cmd --add-source 10.2.128.0/17 --permanent --zone ec-net",
			"firewall-cmd --reload",
		},
		false,
	)

	// Validate host preflight spec has correct CIDR substitutions
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
}
