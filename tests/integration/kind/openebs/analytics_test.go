package openebs

import (
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	addonstypes "github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestOpenEBS_AnalyticsDisabled(t *testing.T) {
	util.SetupCtrlLogging(t)

	clusterName := util.GenerateClusterName(t)
	kubeconfig := util.SetupKindCluster(t, clusterName, nil)

	rc := runtimeconfig.New(nil)

	inSpec := ecv1beta1.InstallationSpec{
		Config: &ecv1beta1.ConfigSpec{
			Domains: ecv1beta1.Domains{
				ProxyRegistryDomain: "proxy.replicated.com",
			},
		},
		RuntimeConfig: rc.Get(),
	}

	clients := addonstypes.NewClients(
		util.CtrlClient(t, kubeconfig),
		util.MetadataClient(t, kubeconfig),
		util.HelmClient(t, kubeconfig),
	)

	addon := openebs.New(
		openebs.WithLogFunc(t.Logf),
	)
	if err := addon.Install(t.Context(), clients, nil, inSpec, nil, addonstypes.InstallOptions{}); err != nil {
		t.Fatalf("failed to install openebs: %v", err)
	}

	deploy := util.GetDeployment(t, kubeconfig, addon.Namespace(), "openebs-localpv-provisioner")
	assert.Contains(t, deploy.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{Name: "OPENEBS_IO_ENABLE_ANALYTICS", Value: "false"},
		"openebs should not send analytics")
}
