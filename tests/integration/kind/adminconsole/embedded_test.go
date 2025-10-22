package adminconsole

import (
	"net"
	"testing"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/tlsutils"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util/kind"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestAdminConsole_EmbeddedCluster(t *testing.T) {
	ctx := t.Context()

	util.SetupCtrlLogging(t)

	clusterName := util.GenerateClusterName(t)

	kindConfig := util.NewKindClusterConfig(t, clusterName, nil)

	kindConfig.Nodes[0].ExtraPortMappings = append(kindConfig.Nodes[0].ExtraPortMappings, kind.PortMapping{
		ContainerPort: 30500,
	})

	// data and k0s directories are required for the admin console addon
	ecDataDirMount := kind.Mount{
		HostPath:      util.TempDirForHostMount(t, "data-dir-*"),
		ContainerPath: "/var/lib/embedded-cluster",
	}
	k0sDirMount := kind.Mount{
		HostPath:      util.TempDirForHostMount(t, "k0s-dir-*"),
		ContainerPath: "/var/lib/embedded-cluster/k0s",
	}
	kindConfig.Nodes[0].ExtraMounts = append(kindConfig.Nodes[0].ExtraMounts, ecDataDirMount, k0sDirMount)

	kubeconfig := util.SetupKindClusterFromConfig(t, kindConfig)

	kcli := util.CtrlClient(t, kubeconfig)
	mcli := util.MetadataClient(t, kubeconfig)
	hcli := util.HelmClient(t, kubeconfig)

	rc := runtimeconfig.New(nil)
	rc.SetNetworkSpec(ecv1beta1.NetworkSpec{
		PodCIDR:     "10.85.0.0/12",
		ServiceCIDR: "10.96.0.0/12",
	})

	domains := ecv1beta1.Domains{
		ReplicatedAppDomain:      "replicated.app",
		ProxyRegistryDomain:      "proxy.replicated.com",
		ReplicatedRegistryDomain: "registry.replicated.com",
	}

	t.Logf("%s installing openebs", formattedTime())
	openebsAddon := &openebs.OpenEBS{
		OpenEBSDataDir: rc.EmbeddedClusterOpenEBSLocalSubDir(),
	}
	if err := openebsAddon.Install(ctx, t.Logf, kcli, mcli, hcli, domains, nil); err != nil {
		t.Fatalf("failed to install openebs: %v", err)
	}

	t.Logf("%s waiting for storageclass", formattedTime())
	util.WaitForStorageClass(t, kubeconfig, "openebs-hostpath", 30*time.Second)

	t.Logf("%s generating tls certificate", formattedTime())
	_, certData, keyData, err := tlsutils.GenerateCertificate("localhost", []net.IP{net.ParseIP("127.0.0.1")}, "my-app-namespace")
	if err != nil {
		t.Fatalf("generate tls certificate: %v", err)
	}

	t.Logf("%s installing admin console", formattedTime())
	addon := &adminconsole.AdminConsole{
		IsAirgap:           false,
		IsHA:               false,
		IsMultiNodeEnabled: false,
		Proxy:              rc.ProxySpec(),
		AdminConsolePort:   rc.AdminConsolePort(),

		ClusterID:        "123",
		ServiceCIDR:      "10.96.0.0/12",
		HostCABundlePath: rc.HostCABundlePath(),
		DataDir:          rc.EmbeddedClusterHomeDirectory(),
		K0sDataDir:       rc.EmbeddedClusterK0sSubDir(),

		Password:     "password",
		TLSCertBytes: certData,
		TLSKeyBytes:  keyData,
		Hostname:     "localhost",
	}
	require.NoError(t, addon.Install(ctx, t.Logf, kcli, mcli, hcli, domains, nil))

	t.Logf("%s waiting for admin console to be ready", formattedTime())
	util.WaitForDeployment(t, kubeconfig, "kotsadm", "kotsadm", 1, 30*time.Second)

	deploy := util.GetDeployment(t, kubeconfig, addon.Namespace(), "kotsadm")

	assert.Contains(t, deploy.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{Name: "EMBEDDED_CLUSTER_ID", Value: "123"},
		"admin console should have the EMBEDDED_CLUSTER_ID env var")
}
