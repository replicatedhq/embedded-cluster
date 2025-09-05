package integration

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

func TestHostCABundle(t *testing.T) {
	chartLocation, err := filepath.Abs("../../../../operator/charts/embedded-cluster-operator")
	require.NoError(t, err, "Failed to get chart location")

	addon := &embeddedclusteroperator.EmbeddedClusterOperator{
		DryRun:                true,
		ChartLocationOverride: chartLocation,
		ClusterID:             "123",
		HostCABundlePath:      "/etc/ssl/certs/ca-certificates.crt",
	}

	hcli, err := helm.NewClient(helm.HelmOptions{
		HelmPath:   "helm", // use the helm binary in PATH
		K8sVersion: "v1.26.0",
	})
	require.NoError(t, err, "NewClient should not return an error")

	err = addon.Install(context.Background(), t.Logf, nil, nil, hcli, ecv1beta1.Domains{}, nil)
	require.NoError(t, err, "embeddedclusteroperator.Install should not return an error")

	manifests := addon.DryRunManifests()
	require.NotEmpty(t, manifests, "DryRunManifests should not be empty")

	var deployment *appsv1.Deployment
	for _, manifest := range manifests {
		if strings.Contains(string(manifest), "# Source: embedded-cluster-operator/templates/embedded-cluster-operator-deployment.yaml") {
			err := yaml.Unmarshal(manifest, &deployment)
			require.NoError(t, err, "Failed to unmarshal EmbeddedClusterOperator deployment")
		}
	}

	require.NotNil(t, deployment, "EmbeddedClusterOperator deployment should not be nil")

	var volume *corev1.Volume
	for _, v := range deployment.Spec.Template.Spec.Volumes {
		if v.Name == "host-ca-bundle" {
			volume = &v
		}
	}
	if assert.NotNil(t, volume, "EmbeddedClusterOperator host-ca-bundle volume should not be nil") {
		assert.Equal(t, volume.HostPath.Path, "/etc/ssl/certs/ca-certificates.crt")
		assert.Equal(t, volume.HostPath.Type, ptr.To(corev1.HostPathFileOrCreate))
	}

	var volumeMount *corev1.VolumeMount
	for _, v := range deployment.Spec.Template.Spec.Containers[0].VolumeMounts {
		if v.Name == "host-ca-bundle" {
			volumeMount = &v
		}
	}
	if assert.NotNil(t, volumeMount, "EmbeddedClusterOperator host-ca-bundle volume mount should not be nil") {
		assert.Equal(t, volumeMount.MountPath, "/certs/ca-certificates.crt")
	}

	// Check the SSL_CERT_DIR environment variable is set correctly
	var sslCertDirEnv *corev1.EnvVar
	for _, env := range deployment.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "SSL_CERT_DIR" {
			sslCertDirEnv = &env
		}
	}
	if assert.NotNil(t, sslCertDirEnv, "SSL_CERT_DIR environment variable should not be nil") {
		assert.Equal(t, sslCertDirEnv.Value, "/certs")
	}
}
