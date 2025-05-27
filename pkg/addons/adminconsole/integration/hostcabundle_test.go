package integration

import (
	"context"
	"strings"
	"testing"

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
		HostCABundlePath: "/etc/ssl/certs/ca-certificates.crt",
	}

	hcli, err := helm.NewClient(helm.HelmOptions{})
	require.NoError(t, err, "NewClient should not return an error")

	err = addon.Install(context.Background(), nil, hcli, nil, nil)
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
		assert.Equal(t, volume.VolumeSource.HostPath.Path, "/etc/ssl/certs/ca-certificates.crt")
		assert.Equal(t, volume.VolumeSource.HostPath.Type, ptr.To(corev1.HostPathFileOrCreate))
	}

	// Check for host-ca-bundle volume mount
	var volumeMount *corev1.VolumeMount
	for _, v := range adminDeployment.Spec.Template.Spec.Containers[0].VolumeMounts {
		if v.Name == "host-ca-bundle" {
			volumeMount = &v
		}
	}
	if assert.NotNil(t, volumeMount, "Admin Console host-ca-bundle volume mount should not be nil") {
		assert.Equal(t, volumeMount.MountPath, "/certs/ca-certificates.crt")
	}

	// Check for SSL_CERT_DIR environment variable
	var sslCertDirEnv *corev1.EnvVar
	for _, env := range adminDeployment.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "SSL_CERT_DIR" {
			sslCertDirEnv = &env
		}
	}
	if assert.NotNil(t, sslCertDirEnv, "Admin Console SSL_CERT_DIR environment variable should not be nil") {
		assert.Equal(t, sslCertDirEnv.Value, "/certs")
	}
}
