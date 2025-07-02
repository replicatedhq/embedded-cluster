package adminconsole

import (
	"net"
	"testing"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/tlsutils"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminConsole_ExistingCluster(t *testing.T) {
	ctx := t.Context()

	util.SetupCtrlLogging(t)

	clusterName := util.GenerateClusterName(t)
	kubeconfig := util.SetupKindCluster(t, clusterName, nil)

	kcli := util.CtrlClient(t, kubeconfig)
	mcli := util.MetadataClient(t, kubeconfig)
	hcli := util.HelmClient(t, kubeconfig)

	ki := kubernetesinstallation.New(nil)

	domains := ecv1beta1.Domains{
		ReplicatedAppDomain:      "replicated.app",
		ProxyRegistryDomain:      "proxy.replicated.com",
		ReplicatedRegistryDomain: "registry.replicated.com",
	}

	t.Logf("%s generating tls certificate", formattedTime())
	_, certData, keyData, err := tlsutils.GenerateCertificate("localhost", []net.IP{net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatalf("generate tls certificate: %v", err)
	}

	t.Logf("%s installing admin console", formattedTime())
	addon := &adminconsole.AdminConsole{
		IsAirgap:           false,
		IsHA:               false,
		IsMultiNodeEnabled: false,
		Proxy:              ki.ProxySpec(),
		AdminConsolePort:   ki.AdminConsolePort(),

		Password:     "password",
		TLSCertBytes: certData,
		TLSKeyBytes:  keyData,
		Hostname:     "localhost",
	}
	require.NoError(t, addon.Install(ctx, t.Logf, kcli, mcli, hcli, domains, nil))

	t.Logf("%s waiting for admin console to be ready", formattedTime())
	util.WaitForDeployment(t, kubeconfig, "kotsadm", "kotsadm", 1, 30*time.Second)

	deploy := util.GetDeployment(t, kubeconfig, addon.Namespace(), "kotsadm")

	// should not have the EMBEDDED_CLUSTER_ID env var
	for _, env := range deploy.Spec.Template.Spec.Containers[0].Env {
		assert.NotEqual(t, "EMBEDDED_CLUSTER_ID", env.Name, "admin console should not have the EMBEDDED_CLUSTER_ID env var")
	}
}
