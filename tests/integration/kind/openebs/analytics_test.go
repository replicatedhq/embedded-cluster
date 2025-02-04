package openebs

import (
	"context"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/addons2/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestOpenEBS_AnalyticsDisabled(t *testing.T) {
	clusterName := util.GenerateClusterName(t)
	kubeconfig := util.SetupKindCluster(t, clusterName, nil)

	runtimeconfig.SetDataDir("/custom")

	hcli, err := helm.NewClient(helm.HelmOptions{KubeConfig: kubeconfig})
	require.NoError(t, err)
	defer hcli.Close()

	addon := &openebs.OpenEBS{}
	if err := addon.Install(context.Background(), nil, hcli, nil, nil); err != nil {
		t.Fatalf("failed to install openebs: %v", err)
	}

	deploy := util.GetDeployment(t, kubeconfig, addon.Namespace(), "openebs-localpv-provisioner")
	assert.Contains(t, deploy.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{Name: "OPENEBS_IO_ENABLE_ANALYTICS", Value: "false"},
		"openebs should not send analytics")
}
