package migratev2

import (
	"context"
	"fmt"
	"io"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// cleanupV1 removes control of the Helm Charts from the k0s controller and uninstalls the Embedded
// Cluster operator.
func cleanupV1(ctx context.Context, logf LogFunc, cli client.Client) error {
	logf("Deleting Installation CRs")
	err := deleteInstallationCRs(ctx, cli)
	if err != nil {
		return fmt.Errorf("delete installation crs: %w", err)
	}
	logf("Successfully deleted Installation CRs")

	logf("Cleaning up v1 ClusterConfig")
	err = cleanupClusterConfig(ctx, cli)
	if err != nil {
		return fmt.Errorf("cleanup cluster config: %w", err)
	}
	logf("Successfully cleaned up v1 ClusterConfig")

	logf("Uninstalling operator")
	err = helmUninstallOperator(ctx)
	if err != nil {
		return fmt.Errorf("helm uninstall operator: %w", err)
	}
	logf("Successfully uninstalled operator")

	return nil
}

func cleanupClusterConfig(ctx context.Context, cli client.Client) error {
	var clusterConfig k0sv1beta1.ClusterConfig
	err := cli.Get(ctx, apitypes.NamespacedName{Namespace: "kube-system", Name: "k0s"}, &clusterConfig)
	if err != nil {
		return fmt.Errorf("get cluster config: %w", err)
	}

	if clusterConfig.Spec.Extensions != nil {
		clusterConfig.Spec.Extensions.Helm = nil
		err = cli.Update(ctx, &clusterConfig)
		if err != nil {
			return fmt.Errorf("update cluster config: %w", err)
		}
	}

	return nil
}

func helmUninstallOperator(ctx context.Context) error {
	helmCLI, err := helm.NewHelm(helm.HelmOptions{Writer: io.Discard})
	if err != nil {
		return fmt.Errorf("create helm client: %w", err)
	}
	defer helmCLI.Close()

	return helmCLI.Uninstall(ctx, helm.UninstallOptions{
		ReleaseName: "embedded-cluster-operator",
		Namespace:   ecNamespace,
		Wait:        true,
	})
}
