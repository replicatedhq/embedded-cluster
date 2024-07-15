package k8sutil

import (
	"context"
	"fmt"

	k0sHelmv1beta1 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetChartHealth checks the 'k0s-addon-chart-%s' chart in the 'kube-system' namespace and the specified version
func GetChartHealthVersion(ctx context.Context, cli client.Client, chartName, expectedVersion string) (bool, error) {
	ch := k0sHelmv1beta1.Chart{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: "kube-system", Name: fmt.Sprintf("k0s-addon-chart-%s", chartName)}, &ch)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("get %s chart: %w", chartName, err)
	}

	// check if the chart is deployed and healthy
	// if it is, return true
	if ch.Status.Version == "" {
		return false, nil
	}
	if expectedVersion != "" && ch.Status.Version != expectedVersion {
		return false, nil
	}
	if ch.Spec.Version != ch.Status.Version {
		return false, nil
	}
	if ch.Status.Error != "" {
		return false, nil
	}
	if ch.Status.ValuesHash == "" {
		return false, nil
	}
	if ch.Spec.HashValues() != ch.Status.ValuesHash {
		return false, nil
	}

	return true, nil
}

// GetChartHealth checks the 'k0s-addon-chart-%s' chart in the 'kube-system' namespace
func GetChartHealth(ctx context.Context, cli client.Client, chartName string) (bool, error) {
	return GetChartHealthVersion(ctx, cli, chartName, "")
}
