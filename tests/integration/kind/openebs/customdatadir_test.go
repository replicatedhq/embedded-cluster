package openebs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
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

	rc := runtimeconfig.New(nil)
	rc.SetDataDir("/custom")

	opts := types.InstallOptions{
		Domains: ecv1beta1.Domains{
			ProxyRegistryDomain: "proxy.replicated.com",
		},
	}

	kcli := util.CtrlClient(t, kubeconfig)
	mcli := util.MetadataClient(t, kubeconfig)
	hcli := util.HelmClient(t, kubeconfig)

	addon := openebs.New(
		openebs.WithLogFunc(t.Logf),
		openebs.WithClients(kcli, mcli, hcli),
		openebs.WithRuntimeConfig(rc),
	)
	if err := addon.Install(t.Context(), nil, opts, nil); err != nil {
		t.Fatalf("failed to install openebs: %v", err)
	}

	util.WaitForStorageClass(t, kubeconfig, "openebs-hostpath", 30*time.Second)

	// create a Pod and PVC to test that the data dir is mounted
	createPodAndPVC(t, kubeconfig)

	_, err := os.Stat(filepath.Join(dataDir, "openebs-local"))
	require.NoError(t, err, "failed to find %s data dir")
	entries, err := os.ReadDir(dataDir)
	require.NoError(t, err, "failed to read openebs data dir")
	assert.Len(t, entries, 1, "expected pvc dir file in openebs data dir")
}
