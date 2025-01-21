package openebs

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestOpenEBS_AnalyticsDisabled(t *testing.T) {
	clusterName := util.GenerateClusterName(t)
	kubeconfig := util.SetupKindCluster(t, clusterName, nil)

	addon := openebs.OpenEBS{}
	runtimeconfig.SetDataDir("/custom")
	charts, _, err := addon.GenerateHelmConfig(nil, false)
	require.NoError(t, err, "failed to generate helm config")

	chart := charts[0]
	namespace := chart.TargetNS

	helmValuesFile := util.WriteHelmValuesFile(t, chart.Values)

	util.HelmInstall(t, kubeconfig, namespace, chart.Name, chart.Version, chart.ChartName, helmValuesFile)

	deploy := util.GetDeployment(t, kubeconfig, namespace, "openebs-localpv-provisioner")
	assert.Contains(t, deploy.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{Name: "OPENEBS_IO_ENABLE_ANALYTICS", Value: "false"},
		"openebs should not send analytics")
}
