package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestLinux_Airgap(t *testing.T) {
	dataDir := t.TempDir()

	addon := &adminconsole.AdminConsole{
		DryRun: true,

		IsAirgap:           true,
		IsHA:               false,
		IsMultiNodeEnabled: true,
		Proxy:              nil,
		AdminConsolePort:   8080,

		ClusterID:        "123",
		ServiceCIDR:      "10.0.0.0/24",
		HostCABundlePath: filepath.Join(t.TempDir(), "ca-certificates.crt"),
		DataDir:          dataDir,
		K0sDataDir:       filepath.Join(dataDir, "k0s"),

		Password:      "password",
		TLSCertBytes:  []byte("cert"),
		TLSKeyBytes:   []byte("key"),
		Hostname:      "admin-console",
		KotsInstaller: nil,
	}

	err := os.WriteFile(addon.HostCABundlePath, []byte("test"), 0644)
	require.NoError(t, err, "Failed to write CA bundle file")

	hcli, err := helm.NewClient(helm.HelmOptions{
		HelmPath:   "helm", // use the helm binary in PATH
		K8sVersion: "v1.26.0",
	})
	require.NoError(t, err, "NewClient should not return an error")

	err = addon.Install(context.Background(), t.Logf, nil, nil, hcli, ecv1beta1.Domains{}, nil)
	require.NoError(t, err, "adminconsole.Install should not return an error")

	manifests := addon.DryRunManifests()
	require.NotEmpty(t, manifests, "DryRunManifests should not be empty")

	var adminDeployment *appsv1.Deployment
	for _, manifest := range manifests {
		manifestStr := string(manifest)
		// Look for the kotsadm deployment by its template source
		if strings.Contains(manifestStr, "# Source: admin-console/templates/kotsadm-deployment.yaml") {
			err := yaml.Unmarshal(manifest, &adminDeployment)
			require.NoError(t, err, "Failed to unmarshal Admin Console deployment")
			break
		}
	}

	require.NotNil(t, adminDeployment, "Admin Console deployment should not be nil")

	// Check for environment variables
	var embeddedClusterEnv, embeddedClusterDataDirEnv, embeddedClusterK0sDirEnv, sslCertConfigmapEnv, enableImprovedDREnv *corev1.EnvVar
	for _, env := range adminDeployment.Spec.Template.Spec.Containers[0].Env {
		switch env.Name {
		case "EMBEDDED_CLUSTER_ID":
			embeddedClusterEnv = &env
		case "EMBEDDED_CLUSTER_DATA_DIR":
			embeddedClusterDataDirEnv = &env
		case "EMBEDDED_CLUSTER_K0S_DIR":
			embeddedClusterK0sDirEnv = &env
		case "SSL_CERT_CONFIGMAP":
			sslCertConfigmapEnv = &env
		case "ENABLE_IMPROVED_DR":
			enableImprovedDREnv = &env
		}
	}
	if assert.NotNil(t, embeddedClusterEnv, "Admin Console EMBEDDED_CLUSTER_ID environment variable should not be nil") {
		assert.Equal(t, "123", embeddedClusterEnv.Value)
	}
	if assert.NotNil(t, embeddedClusterDataDirEnv, "Admin Console EMBEDDED_CLUSTER_DATA_DIR environment variable should not be nil") {
		assert.Equal(t, dataDir, embeddedClusterDataDirEnv.Value)
	}
	if assert.NotNil(t, embeddedClusterK0sDirEnv, "Admin Console EMBEDDED_CLUSTER_K0S_DIR environment variable should not be nil") {
		assert.Equal(t, filepath.Join(dataDir, "k0s"), embeddedClusterK0sDirEnv.Value)
	}
	if assert.NotNil(t, sslCertConfigmapEnv, "Admin Console SSL_CERT_CONFIGMAP environment variable should not be nil") {
		assert.Equal(t, "kotsadm-private-cas", sslCertConfigmapEnv.Value)
	}
	if assert.NotNil(t, enableImprovedDREnv, "Admin Console ENABLE_IMPROVED_DR environment variable should not be nil") {
		assert.Equal(t, "true", enableImprovedDREnv.Value)
	}

	var registrySecret *corev1.Secret
	for _, manifest := range manifests {
		manifestStr := string(manifest)
		if strings.Contains(manifestStr, "registry-creds") {
			err := yaml.Unmarshal(manifest, &registrySecret)
			require.NoError(t, err, "Failed to unmarshal registry secret")
		}
	}

	require.NotNil(t, registrySecret, "registry-creds secret should not be nil")
}
