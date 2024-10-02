package charts

import (
	"context"
	"fmt"
	"strings"

	v1beta3 "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	v1beta2 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	"sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconcileHelmCharts reconciles the helm charts from the Installation metadata with the clusterconfig object.
func ReconcileHelmCharts(ctx context.Context, cli client.Client, in *v1beta1.Installation) error {
	if in.Spec.Config == nil || in.Spec.Config.Version == "" {
		if in.Status.State == v1beta1.InstallationStateKubernetesInstalled {
			in.Status.SetState(v1beta1.InstallationStateInstalled, "Installed", nil)
		}
		return nil
	}

	log := controllerruntime.LoggerFrom(ctx)
	// skip if the installer has already failed or if the k0s upgrade is still in progress
	if in.Status.State == v1beta1.InstallationStateFailed ||
		!in.Status.GetKubernetesInstalled() {
		log.Info("Skipping helm chart reconciliation", "state", in.Status.State)
		return nil
	}

	meta, err := release.MetadataFor(ctx, in, cli)
	if err != nil {
		in.Status.SetState(v1beta1.InstallationStateHelmChartUpdateFailure, err.Error(), nil)
		return nil
	}

	// skip if the new release has no addon configs - this should not happen in production
	if len(meta.Configs.Charts) == 0 {
		log.Info("Addons", "configcheck", "no addons")
		if in.Status.State == v1beta1.InstallationStateKubernetesInstalled {
			in.Status.SetState(v1beta1.InstallationStateInstalled, "Installed", nil)
		}
		return nil
	}

	// fetch the current clusterConfig
	var clusterConfig v1beta2.ClusterConfig
	if err := cli.Get(ctx, client.ObjectKey{Name: "k0s", Namespace: "kube-system"}, &clusterConfig); err != nil {
		return fmt.Errorf("failed to get cluster config: %w", err)
	}

	combinedConfigs, err := K0sHelmExtensionsFromInstallation(ctx, in, meta, &clusterConfig)
	if err != nil {
		return fmt.Errorf("failed to get helm charts from installation: %w", err)
	}

	cfgs := &v1beta2.HelmExtensions{}
	cfgs, err = v1beta1.ConvertTo(*combinedConfigs, cfgs)
	if err != nil {
		return fmt.Errorf("failed to convert chart types: %w", err)
	}

	existingHelm := &v1beta2.HelmExtensions{}
	if clusterConfig.Spec != nil && clusterConfig.Spec.Extensions != nil && clusterConfig.Spec.Extensions.Helm != nil {
		existingHelm = clusterConfig.Spec.Extensions.Helm
	}

	chartDrift, changedCharts, err := DetectChartDrift(cfgs, existingHelm)
	if err != nil {
		return fmt.Errorf("failed to check chart drift: %w", err)
	}

	// detect drift between the cluster config and the installer metadata
	var installedCharts v1beta3.ChartList
	if err := cli.List(ctx, &installedCharts); err != nil {
		return fmt.Errorf("failed to list installed charts: %w", err)
	}
	pendingCharts, chartErrors, err := DetectChartCompletion(existingHelm, installedCharts)
	if err != nil {
		return fmt.Errorf("failed to check chart completion: %w", err)
	}

	// If any chart has errors, update installer state and return
	// if there is a difference between what we want and what we have
	// we should update the cluster instead of letting chart errors stop deployment permanently
	// otherwise if there are errors we need to abort
	if len(chartErrors) > 0 && !chartDrift {
		chartErrorString := strings.Join(chartErrors, ",")
		chartErrorString = "failed to update helm charts: " + chartErrorString
		log.Info("Chart errors", "errors", chartErrorString)
		if len(chartErrorString) > 1024 {
			chartErrorString = chartErrorString[:1024]
		}
		in.Status.SetState(v1beta1.InstallationStateHelmChartUpdateFailure, chartErrorString, nil)
		return nil
	}

	// If all addons match their target version + values, mark installation as complete
	if len(pendingCharts) == 0 && !chartDrift {
		in.Status.SetState(v1beta1.InstallationStateInstalled, "Addons upgraded", nil)
		return nil
	}

	if len(pendingCharts) > 0 {
		// If there are pending charts, mark the installation as pending with a message about the pending charts
		in.Status.SetState(v1beta1.InstallationStatePendingChartCreation, fmt.Sprintf("Pending charts: %v", pendingCharts), pendingCharts)
		return nil
	}

	if in.Status.State == v1beta1.InstallationStateAddonsInstalling {
		// after the first time we apply new helm charts, this will be set to InstallationStateAddonsInstalling
		// and we will not re-apply the charts to the k0s cluster config while waiting for those changes to propagate
		return nil
	}

	if !chartDrift {
		// if there is no drift, we should not reapply the cluster config
		// however, the charts have not been applied yet, so we should not mark the installation as complete
		// this should not happen on upgrades
		return nil
	}

	// Replace the current chart configs with the new chart configs
	clusterConfig.Spec.Extensions.Helm = cfgs
	in.Status.SetState(v1beta1.InstallationStateAddonsInstalling, "Installing addons", nil)
	log.Info("Updating cluster config with new helm charts", "updated charts", changedCharts)
	//Update the clusterConfig
	if err := cli.Update(ctx, &clusterConfig); err != nil {
		return fmt.Errorf("failed to update cluster config: %w", err)
	}
	return nil
}
