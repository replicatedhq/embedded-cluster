package velero

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// TODO: this should test creating a backup storage location and possibly a backup
func TestVelero_HostCABundle(t *testing.T) {
	util.SetupCtrlLogging(t)

	clusterName := util.GenerateClusterName(t)
	kubeconfig := util.SetupKindCluster(t, clusterName, nil)

	kcli := util.CtrlClient(t, kubeconfig)
	hcli := util.HelmClient(t, kubeconfig)

	addon := &velero.Velero{
		HostCABundlePath: "/etc/ssl/certs/ca-certificates.crt",
	}
	if err := addon.Install(t.Context(), kcli, hcli, nil, nil); err != nil {
		t.Fatalf("failed to install velero: %v", err)
	}

	deploy := util.GetDeployment(t, kubeconfig, addon.Namespace(), "velero")

	var volume *corev1.Volume
	for _, v := range deploy.Spec.Template.Spec.Volumes {
		if v.Name == "host-ca-bundle" {
			volume = &v
		}
	}
	if assert.NotNil(t, volume, "Volume host-ca-bundle not found") {
		assert.Equal(t, volume.VolumeSource.HostPath.Path, "/etc/ssl/certs/ca-certificates.crt")
		assert.Equal(t, volume.VolumeSource.HostPath.Type, ptr.To(corev1.HostPathFileOrCreate))
	}

	var volumeMount *corev1.VolumeMount
	for _, v := range deploy.Spec.Template.Spec.Containers[0].VolumeMounts {
		if v.Name == "host-ca-bundle" {
			volumeMount = &v
		}
	}
	if assert.NotNil(t, volumeMount, "VolumeMount host-ca-bundle not found") {
		assert.Equal(t, volumeMount.MountPath, "/certs/ca-certificates.crt")
	}
}
