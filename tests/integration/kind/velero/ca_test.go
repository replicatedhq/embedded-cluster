package velero

import (
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
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
	mcli := util.MetadataClient(t, kubeconfig)
	hcli := util.HelmClient(t, kubeconfig)

	domains := ecv1beta1.Domains{
		ProxyRegistryDomain: "proxy.replicated.com",
	}

	addon := &velero.Velero{
		HostCABundlePath: "/etc/ssl/certs/ca-certificates.crt",
	}

	// Helm 4 uses kstatus for resource readiness, which checks DaemonSet availability strictly
	// (NumberAvailable >= DesiredNumberScheduled). Helm 3 had a bug where DaemonSets with
	// maxUnavailable=1 and desiredNumberScheduled=1 were immediately considered ready
	// (expectedReady = 1-1 = 0, so NumberReady >= 0 always passed).
	//
	// In kind clusters the k0s-specific kubelet paths don't exist, so node-agent pods fail
	// to start. Disable hostPath mounts to allow node-agent to become available; this doesn't
	// affect the CA bundle volumes being tested.
	kindOverrides := []string{"nodeAgent:\n  disableHostPath: true\n  podVolumePath: /var/lib/kubelet/pods\n  pluginVolumePath: /var/lib/kubelet/plugins"}
	if err := addon.Install(t.Context(), t.Logf, kcli, mcli, hcli, domains, kindOverrides); err != nil {
		t.Fatalf("failed to install velero: %v", err)
	}

	veleroDeploy := util.GetDeployment(t, kubeconfig, addon.Namespace(), "velero")
	nodeAgentDaemonSet := util.GetDaemonSet(t, kubeconfig, addon.Namespace(), "node-agent")

	var volume *corev1.Volume
	for _, v := range veleroDeploy.Spec.Template.Spec.Volumes {
		if v.Name == "host-ca-bundle" {
			volume = &v
		}
	}
	if assert.NotNil(t, volume, "Velero host-ca-bundle volume should not be nil") {
		assert.Equal(t, volume.HostPath.Path, "/etc/ssl/certs/ca-certificates.crt")
		assert.Equal(t, volume.HostPath.Type, ptr.To(corev1.HostPathFileOrCreate))
	}

	var volumeMount *corev1.VolumeMount
	for _, v := range veleroDeploy.Spec.Template.Spec.Containers[0].VolumeMounts {
		if v.Name == "host-ca-bundle" {
			volumeMount = &v
		}
	}
	if assert.NotNil(t, volumeMount, "Velero host-ca-bundle volume mount should not be nil") {
		assert.Equal(t, volumeMount.MountPath, "/certs/ca-certificates.crt")
	}

	volume = nil
	for _, v := range nodeAgentDaemonSet.Spec.Template.Spec.Volumes {
		if v.Name == "host-ca-bundle" {
			volume = &v
		}
	}
	if assert.NotNil(t, volume, "Velero node agent host-ca-bundle volume should not be nil") {
		assert.Equal(t, volume.HostPath.Path, "/etc/ssl/certs/ca-certificates.crt")
		assert.Equal(t, volume.HostPath.Type, ptr.To(corev1.HostPathFileOrCreate))
	}

	volumeMount = nil
	for _, v := range nodeAgentDaemonSet.Spec.Template.Spec.Containers[0].VolumeMounts {
		if v.Name == "host-ca-bundle" {
			volumeMount = &v
		}
	}
	if assert.NotNil(t, volumeMount, "Velero node agent host-ca-bundle volume mount should not be nil") {
		assert.Equal(t, volumeMount.MountPath, "/certs/ca-certificates.crt")
	}
}
