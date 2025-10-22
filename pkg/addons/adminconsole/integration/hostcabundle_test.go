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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

func TestHostCABundle(t *testing.T) {
	addon := &adminconsole.AdminConsole{
		DryRun:           true,
		HostCABundlePath: filepath.Join(t.TempDir(), "ca-certificates.crt"),
		KotsadmNamespace: "my-app-namespace",
	}

	err := os.WriteFile(addon.HostCABundlePath, []byte("test"), 0644)
	require.NoError(t, err, "Failed to write CA bundle file")

	hcli, err := helm.NewClient(helm.HelmOptions{})
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

	// Check for host-ca-bundle volume
	var volume *corev1.Volume
	for _, v := range adminDeployment.Spec.Template.Spec.Volumes {
		if v.Name == "host-ca-bundle" {
			volume = &v
		}
	}
	if assert.NotNil(t, volume, "Admin Console host-ca-bundle volume should not be nil") {
		assert.Equal(t, addon.HostCABundlePath, volume.HostPath.Path)
		assert.Equal(t, ptr.To(corev1.HostPathFileOrCreate), volume.HostPath.Type)
	}

	// Check for host-ca-bundle volume mount
	var volumeMount *corev1.VolumeMount
	for _, v := range adminDeployment.Spec.Template.Spec.Containers[0].VolumeMounts {
		if v.Name == "host-ca-bundle" {
			volumeMount = &v
		}
	}
	if assert.NotNil(t, volumeMount, "Admin Console host-ca-bundle volume mount should not be nil") {
		assert.Equal(t, "/certs/ca-certificates.crt", volumeMount.MountPath)
	}

	// Check for SSL_CERT_DIR environment variable
	var sslCertDirEnv *corev1.EnvVar
	for _, env := range adminDeployment.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "SSL_CERT_DIR" {
			sslCertDirEnv = &env
		}
	}
	if assert.NotNil(t, sslCertDirEnv, "Admin Console SSL_CERT_DIR environment variable should not be nil") {
		assert.Equal(t, "/certs", sslCertDirEnv.Value)
	}

	// Check for kotsadm-private-cas configmap
	var configmap *corev1.ConfigMap
	for _, manifest := range manifests {
		manifestStr := string(manifest)
		if strings.Contains(manifestStr, "ca_0.crt") {
			err := yaml.Unmarshal(manifest, &configmap)
			require.NoError(t, err, "Failed to unmarshal Admin Console deployment")
			break
		}
	}

	if assert.NotNil(t, configmap, "kotsadm-private-cas configmap should not be nil") {
		assert.Equal(t, "test", configmap.Data["ca_0.crt"])
	}
}
