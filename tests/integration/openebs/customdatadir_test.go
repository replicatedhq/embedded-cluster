package openebs

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util"
	"github.com/replicatedhq/embedded-cluster/tests/integration/util/kind"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenEBS_CustomDataDir(t *testing.T) {
	t.Parallel()

	clusterName := util.GenerateClusterName(t)
	kindConfig := util.NewKindClusterConfig(t, clusterName, nil)

	dataDir := util.TempDirForHostMount(t, "data-dir-*")
	kindConfig.Nodes[0].ExtraMounts = append(kindConfig.Nodes[0].ExtraMounts, kind.Mount{
		HostPath:      dataDir,
		ContainerPath: "/custom",
	})
	kubeconfig := util.SetupKindClusterFromConfig(t, kindConfig)

	addon := openebs.OpenEBS{}
	provider := defaults.NewProvider("/custom")
	charts, _, err := addon.GenerateHelmConfig(provider, nil, false)
	require.NoError(t, err, "failed to generate helm config")

	chart := charts[0]
	namespace := chart.TargetNS

	helmValuesFile := util.WriteHelmValuesFile(t, chart.Values)

	util.HelmInstall(t, kubeconfig, namespace, chart.Name, chart.Version, chart.ChartName, helmValuesFile)

	util.WaitForStorageClass(t, kubeconfig, "openebs-hostpath", 30*time.Second)

	// create a Pod and PVC to test that the data dir is mounted
	createPodAndPVC(t, kubeconfig)

	_, err = os.Stat(filepath.Join(dataDir, "openebs-local"))
	require.NoError(t, err, "failed to find %s data dir")
	entries, err := os.ReadDir(dataDir)
	require.NoError(t, err, "failed to read openebs data dir")
	assert.Len(t, entries, 1, "expected pvc dir file in openebs data dir")
}
