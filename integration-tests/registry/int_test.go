package registry

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/integration-tests/registry/static"
	"github.com/replicatedhq/embedded-cluster/integration-tests/util"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
)

func TestRegistryChart(t *testing.T) {
	clusterName := "int-test-registry"

	// cleanup
	util.DeleteKindCluster(t, clusterName)

	kubeconfig := util.CreateKindCluster(t, clusterName, nil)
	// t.Cleanup(func() { util.DeleteKindCluster(t, clusterName) })

	os.Setenv("KUBECONFIG", kubeconfig)

	cli, err := kubeutils.KubeClient()
	if err != nil {
		t.Fatalf("failed to create kube client: %s", err)
	}

	installOpenEBS(t)

	charts, _, err := registry.GenerateHelmConfig(registry.HelmConfigOptions{
		Namespace: "registry",
		IsHA:      false,
		ServiceIP: "10.96.0.12",
	})
	if err != nil {
		t.Fatalf("failed to generate helm config: %s", err)
	}

	chart := charts[0]
	namespace := chart.TargetNS

	helmValuesFile := util.WriteHelmValuesFile(t, "registry", chart.Values)

	util.HelmInstall(t, namespace, chart.Name, chart.Version, chart.ChartName, helmValuesFile)

	t.Log("creating registry auth secret")
	err = registry.CreateRegistryAuthSecret(context.Background(), cli, namespace, "password")
	if err != nil {
		t.Fatalf("failed to create registry secret: %s", err)
	}
	t.Log("registry auth secret created")

	t.Log("creating registry tls secret")
	err = registry.CreateRegistryTLSSecret(context.Background(), cli, namespace)
	if err != nil {
		t.Fatalf("failed to create registry secret: %s", err)
	}
	t.Log("registry tls secret created")

	util.WaitForDeployment(t, "registry", "registry", 1, 30*time.Second)
}

func installOpenEBS(t *testing.T) {
	values, err := static.FS.ReadFile("openebs-values.yaml")
	if err != nil {
		t.Fatalf("failed to read openebs values: %s", err)
	}
	helmValuesFile := util.WriteHelmValuesFile(t, "openebs", string(values))

	util.AddHelmRepo(t, "openebs", "https://openebs.github.io/openebs")
	util.HelmInstall(t, "openebs", "openebs", openebs.Metadata.Version, "openebs/openebs", helmValuesFile)
	util.WaitForDeployment(t, "openebs", "openebs-localpv-provisioner", 1, 30*time.Second)
}
