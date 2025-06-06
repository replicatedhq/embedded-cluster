package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestK0sDir(t *testing.T) {
	hcli, err := helm.NewClient(helm.HelmOptions{})
	require.NoError(t, err, "NewClient should not return an error")

	rc := runtimeconfig.New(nil)

	addon := velero.New(
		velero.WithLogFunc(t.Logf),
		velero.WithClients(nil, nil, hcli),
		velero.WithRuntimeConfig(rc),
	)

	opts := types.InstallOptions{
		IsDryRun: true,
	}

	err = addon.Install(context.Background(), nil, opts, nil)
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
		assert.Equal(t, hostPodsVolume.VolumeSource.HostPath.Path, rc.EmbeddedClusterK0sSubDir()+"/kubelet/pods")
	}
	if assert.NotNil(t, hostPluginsVolume, "Velero host-plugins volume should not be nil") {
		assert.Equal(t, hostPluginsVolume.VolumeSource.HostPath.Path, rc.EmbeddedClusterK0sSubDir()+"/kubelet/plugins")
	}
}
