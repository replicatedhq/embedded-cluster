package dryrun

import (
	"context"
	"os"
	"regexp"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/dryrun"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

	hcli.AssertExpectations(t)

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

	// --- validate environment variables --- //
	assert.Equal(t, "http://localhost:3128", os.Getenv("HTTP_PROXY"))
	assert.Equal(t, "https://localhost:3128", os.Getenv("HTTPS_PROXY"))
	assert.Equal(t, noProxy, os.Getenv("NO_PROXY"))

	// --- validate requests use http proxy --- //
	// TODO: use mocks to test this
	re := regexp.MustCompile(`WARNING: Failed to check for newer app versions.*: proxyconnect tcp: dial tcp \[::1\]:3128: connect: connection refused`)
	assert.Regexp(t, re, dr.LogOutput)

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

func TestHTTPProxyValidateEnvVarsFromFlags(t *testing.T) {
	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons
		hcli.On("Install", mock.Anything, mock.Anything).Times(4).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	// Set HTTP proxy args
	args := []string{
		"--http-proxy", "http://localhost:3128",
		"--https-proxy", "https://localhost:3128",
		"--no-proxy", "localhost,127.0.0.1,10.0.0.0/8",
	}

	dr := dryrunInstall(t, &dryrun.Client{HelmClient: hcli}, args...)

	hcli.AssertExpectations(t)

	// --- get no proxy from operator values --- //

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

	// --- validate environment variables --- //
	assert.Equal(t, "http://localhost:3128", os.Getenv("HTTP_PROXY"))
	assert.Equal(t, "https://localhost:3128", os.Getenv("HTTPS_PROXY"))
	assert.Equal(t, noProxy, os.Getenv("NO_PROXY"))

	// --- validate requests use http proxy --- //
	// TODO: use mocks to test this
	re := regexp.MustCompile(`WARNING: Failed to check for newer app versions.*: proxyconnect tcp: dial tcp \[::1\]:3128: connect: connection refused`)
	assert.Regexp(t, re, dr.LogOutput)
}

func TestHTTPProxyValidateEnvVarsPrecedence(t *testing.T) {
	hcli := &helm.MockClient{}

	mock.InOrder(
		// 4 addons
		hcli.On("Install", mock.Anything, mock.Anything).Times(4).Return(nil, nil),
		hcli.On("Close").Once().Return(nil),
	)

	// Set HTTP proxy environment variables
	t.Setenv("HTTP_PROXY", "http://localhost-env:3128")
	t.Setenv("HTTPS_PROXY", "https://localhost-env:3128")
	t.Setenv("NO_PROXY", "localhost-env,127.0.0.1,10.0.0.0/8")

	// Set HTTP proxy args
	args := []string{
		"--http-proxy", "http://localhost:3128",
		"--https-proxy", "https://localhost:3128",
		"--no-proxy", "localhost,127.0.0.1,10.0.0.0/8",
	}

	dr := dryrunInstall(t, &dryrun.Client{HelmClient: hcli}, args...)

	hcli.AssertExpectations(t)

	// --- get no proxy from operator values --- //

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

	// --- validate environment variables --- //
	assert.Equal(t, "http://localhost:3128", os.Getenv("HTTP_PROXY"))
	assert.Equal(t, "https://localhost:3128", os.Getenv("HTTPS_PROXY"))
	assert.Equal(t, noProxy, os.Getenv("NO_PROXY"))

	// --- validate requests use http proxy --- //
	// TODO: use mocks to test this
	re := regexp.MustCompile(`WARNING: Failed to check for newer app versions.*: proxyconnect tcp: dial tcp \[::1\]:3128: connect: connection refused`)
	assert.Regexp(t, re, dr.LogOutput)
}
