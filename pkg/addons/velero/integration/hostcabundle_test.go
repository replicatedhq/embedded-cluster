package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

func TestHostCABundle(t *testing.T) {
	addon := &velero.Velero{
		DryRun:           true,
		HostCABundlePath: "/etc/ssl/certs/ca-certificates.crt",
	}

	hcli, err := helm.NewClient(helm.HelmOptions{})
	require.NoError(t, err, "NewClient should not return an error")

	err = addon.Install(context.Background(), nil, hcli, nil, nil)
	require.NoError(t, err, "velero.Install should not return an error")

	manifests := addon.DryRunManifests()
	require.NotEmpty(t, manifests, "DryRunManifests should not be empty")

	var deploy *appsv1.Deployment
	for _, manifest := range manifests {
		if strings.Contains(string(manifest), "# Source: velero/templates/deployment.yaml") {
			err := yaml.Unmarshal(manifest, &deploy)
			require.NoError(t, err, "Failed to unmarshal Velero deployment")
			break
		}
	}

	require.NotNil(t, deploy, "Velero deployment should not be nil")

	var volume *corev1.Volume
	for _, v := range deploy.Spec.Template.Spec.Volumes {
		if v.Name == "host-ca-bundle" {
			volume = &v
		}
	}
	if assert.NotNil(t, volume, "Volume host-ca-bundle should not be nil") {
		assert.Equal(t, volume.VolumeSource.HostPath.Path, "/etc/ssl/certs/ca-certificates.crt")
		assert.Equal(t, volume.VolumeSource.HostPath.Type, ptr.To(corev1.HostPathFileOrCreate))
	}

	var volumeMount *corev1.VolumeMount
	for _, v := range deploy.Spec.Template.Spec.Containers[0].VolumeMounts {
		if v.Name == "host-ca-bundle" {
			volumeMount = &v
		}
	}
	if assert.NotNil(t, volumeMount, "VolumeMount host-ca-bundle should not be nil") {
		assert.Equal(t, volumeMount.MountPath, "/certs/ca-certificates.crt")
	}
}
