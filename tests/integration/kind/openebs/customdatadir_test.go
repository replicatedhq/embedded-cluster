package openebs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util/kind"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenEBS_CustomDataDir(t *testing.T) {
	util.SetupCtrlLogging(t)

	clusterName := util.GenerateClusterName(t)
	kindConfig := util.NewKindClusterConfig(t, clusterName, nil)

	dataDir := util.TempDirForHostMount(t, "data-dir-*")
	kindConfig.Nodes[0].ExtraMounts = append(kindConfig.Nodes[0].ExtraMounts, kind.Mount{
		HostPath:      dataDir,
		ContainerPath: "/custom",
	})
	kubeconfig := util.SetupKindClusterFromConfig(t, kindConfig)

	kcli := util.CtrlClient(t, kubeconfig)
	mcli := util.MetadataClient(t, kubeconfig)
	hcli := util.HelmClient(t, kubeconfig)

	domains := ecv1beta1.Domains{
		ProxyRegistryDomain: "proxy.replicated.com",
	}

	addon := &openebs.OpenEBS{
		OpenEBSDataDir: "/custom/openebs-local",
	}
	if err := addon.Install(t.Context(), t.Logf, kcli, mcli, hcli, domains, nil); err != nil {
		t.Fatalf("failed to install openebs: %v", err)
	}

	util.WaitForStorageClass(t, kubeconfig, "openebs-hostpath", 30*time.Second)

	// create a Pod and PVC to test that the data dir is mounted
	createPodAndPVC(t, kubeconfig)

	_, err := helpers.Stat(filepath.Join(dataDir, "openebs-local"))
	require.NoError(t, err, "failed to find openebs data dir")
	entries, err := os.ReadDir(dataDir)
	require.NoError(t, err, "failed to read openebs data dir")
	assert.Len(t, entries, 1, "expected pvc dir file in openebs data dir")
}
