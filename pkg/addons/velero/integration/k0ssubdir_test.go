package integration

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestK0sDir(t *testing.T) {
	k0sDir := filepath.Join(t.TempDir(), "other-k0s")

	addon := &velero.Velero{
		DryRun:     true,
		K0sDataDir: k0sDir,
	}

	hcli, err := helm.NewClient(helm.HelmOptions{
		HelmPath: "helm", // use the helm binary in PATH
	})
	require.NoError(t, err, "NewClient should not return an error")

	err = addon.Install(context.Background(), t.Logf, nil, nil, hcli, ecv1beta1.Domains{}, nil)
	require.NoError(t, err, "velero.Install should not return an error")

	manifests := addon.DryRunManifests()
	require.NotEmpty(t, manifests, "DryRunManifests should not be empty")

	var nodeAgentDaemonSet *appsv1.DaemonSet
	for _, manifest := range manifests {
		if strings.Contains(string(manifest), "# Source: velero/templates/node-agent-daemonset.yaml") {
			err := yaml.Unmarshal(manifest, &nodeAgentDaemonSet)
			require.NoError(t, err, "Failed to unmarshal Velero node agent daemonset")
		}
	}

	require.NotNil(t, nodeAgentDaemonSet, "NodeAgent daemonset should not be nil")

	var hostPodsVolume, hostPluginsVolume *corev1.Volume
	for _, v := range nodeAgentDaemonSet.Spec.Template.Spec.Volumes {
		if v.Name == "host-pods" {
			hostPodsVolume = &v
		}
		if v.Name == "host-plugins" {
			hostPluginsVolume = &v
		}
	}
	if assert.NotNil(t, hostPodsVolume, "Velero host-pods volume should not be nil") {
		assert.Equal(t, hostPodsVolume.HostPath.Path, k0sDir+"/kubelet/pods")
	}
	if assert.NotNil(t, hostPluginsVolume, "Velero host-plugins volume should not be nil") {
		assert.Equal(t, hostPluginsVolume.HostPath.Path, k0sDir+"/kubelet/plugins")
	}
}
