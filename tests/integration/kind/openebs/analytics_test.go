package openebs

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestOpenEBS_AnalyticsDisabled(t *testing.T) {
	clusterName := util.GenerateClusterName(t)
	kubeconfig := util.SetupKindCluster(t, clusterName, nil)

	kcli := util.CtrlClient(t, kubeconfig)
	hcli := util.HelmClient(t, kubeconfig)

	addon := &openebs.OpenEBS{
		ProxyRegistryDomain: "proxy.replicated.com",
	}
	if err := addon.Install(t.Context(), kcli, hcli, nil, nil); err != nil {
		t.Fatalf("failed to install openebs: %v", err)
	}

	deploy := util.GetDeployment(t, kubeconfig, addon.Namespace(), "openebs-localpv-provisioner")
	assert.Contains(t, deploy.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{Name: "OPENEBS_IO_ENABLE_ANALYTICS", Value: "false"},
		"openebs should not send analytics")
}
