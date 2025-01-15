package migratev2

import (
	"context"
	"fmt"
	"io"

	k0shelmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// cleanupV1 removes control of the Helm Charts from the k0s controller and uninstalls the Embedded
// Cluster operator.
func cleanupV1(ctx context.Context, logf LogFunc, cli client.Client) error {
	logf("Uninstalling operator")
	err := helmUninstallOperator(ctx, logf)
	if err != nil {
		return fmt.Errorf("helm uninstall operator: %w", err)
	}
	logf("Successfully uninstalled operator")

	logf("Deleting Chart custom resources")
	err = forceDeleteChartCRs(ctx, cli)
	if err != nil {
		return fmt.Errorf("delete chart custom resources: %w", err)
	}
	logf("Successfully deleted Chart custom resources")

	logf("Cleaning up v1 ClusterConfig")
	err = cleanupClusterConfig(ctx, cli)
	if err != nil {
		return fmt.Errorf("cleanup cluster config: %w", err)
	}
	logf("Successfully cleaned up v1 ClusterConfig")

	// Do this again to ensure that it was not re-installed by k0s
	logf("Uninstalling operator")
	err = helmUninstallOperator(ctx, logf)
	if err != nil {
		return fmt.Errorf("helm uninstall operator: %w", err)
	}
	logf("Successfully uninstalled operator")

	return nil
}

// forceDeleteChartCRs is necessary because the k0s controller will otherwise uninstall the Helm
// Charts
func forceDeleteChartCRs(ctx context.Context, cli client.Client) error {
	var chartList k0shelmv1beta1.ChartList
	err := cli.List(ctx, &chartList)
	if err != nil {
		return fmt.Errorf("list charts: %w", err)
	}

	for _, chart := range chartList.Items {
		chart.ObjectMeta.Finalizers = []string{}
		err := cli.Update(ctx, &chart)
		if err != nil {
			return fmt.Errorf("update chart: %w", err)
		}

		err = cli.Delete(ctx, &chart,
			client.GracePeriodSeconds(0), client.PropagationPolicy(metav1.DeletePropagationOrphan),
		)
		if err != nil {
			return fmt.Errorf("delete chart: %w", err)
		}
	}

	return nil
}

func cleanupClusterConfig(ctx context.Context, cli client.Client) error {
	var clusterConfig k0sv1beta1.ClusterConfig
	err := cli.Get(ctx, apitypes.NamespacedName{Namespace: "kube-system", Name: "k0s"}, &clusterConfig)
	if err != nil {
		return fmt.Errorf("get cluster config: %w", err)
	}

	if clusterConfig.Spec.Extensions != nil {
		clusterConfig.Spec.Extensions.Helm = &k0sv1beta1.HelmExtensions{}
		err = cli.Update(ctx, &clusterConfig)
		if err != nil {
			return fmt.Errorf("update cluster config: %w", err)
		}
	}

	return nil
}

func helmUninstallOperator(ctx context.Context, logf LogFunc) error {
	helmCLI, err := helm.NewHelm(helm.HelmOptions{
		Writer: io.Discard,
		LogFn:  func(format string, v ...interface{}) { logf(format, v...) },
	})
	if err != nil {
		return fmt.Errorf("create helm client: %w", err)
	}
	defer helmCLI.Close()

	return helmCLI.Uninstall(ctx, helm.UninstallOptions{
		ReleaseName:    "embedded-cluster-operator",
		Namespace:      "embedded-cluster",
		Wait:           true,
		IgnoreNotFound: true,
	})
}
