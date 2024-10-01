package upgrade

import (
	"context"
	"fmt"
	"strings"
	"time"

	apv1b2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0shelm "github.com/k0sproject/k0s/pkg/apis/helm/v1beta1"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/autopilot"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/charts"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/k8sutil"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	operatorChartName   = "embedded-cluster-operator"
	upgradeJobName      = "embedded-cluster-upgrade-%s"
	upgradeJobConfigMap = "upgrade-job-configmap-%s"
)

// Upgrade upgrades the embedded cluster to the version specified in the installation.
// First the k0s cluster is upgraded, then addon charts are upgraded, and finally the installation is unlocked.
func Upgrade(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) error {
	err := k0sUpgrade(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("k0s upgrade: %w", err)
	}

	err = chartUpgrade(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("chart upgrade: %w", err)
	}

	// wait for the operator chart to be ready
	err = waitForOperatorChart(ctx, cli, in.Spec.Config.Version)
	if err != nil {
		return fmt.Errorf("wait for operator chart: %w", err)
	}

	err = unLockInstallation(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("unlock installation: %w", err)
	}

	return nil
}

func k0sUpgrade(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) error {
	meta, err := release.MetadataFor(ctx, in, cli)
	if err != nil {
		return fmt.Errorf("failed to get release metadata: %w", err)
	}

	// check if the k0s version is the same as the current version
	// if it is, we can skip the upgrade
	desiredVersion := k8sutil.K0sVersionFromMetadata(meta)

	match, err := k8sutil.ClusterNodesMatchVersion(ctx, cli, desiredVersion)
	if err != nil {
		return fmt.Errorf("check cluster nodes match version: %w", err)
	}
	if match {
		return nil
	}

	// create an autopilot upgrade plan if one does not yet exist
	var plan apv1b2.Plan
	okey := client.ObjectKey{Name: "autopilot"}
	if err := cli.Get(ctx, okey, &plan); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get upgrade plan: %w", err)
	} else if errors.IsNotFound(err) {
		// if the kubernetes version has changed we create an upgrade command
		fmt.Printf("Starting k0s autopilot upgrade plan to version %s\n", desiredVersion)

		// there is no autopilot plan in the cluster so we are free to
		// start our own plan. here we link the plan to the installation
		// by its name.
		if err := StartAutopilotUpgrade(ctx, cli, in, meta); err != nil {
			return fmt.Errorf("failed to start upgrade: %w", err)
		}
	}

	// restart this function/pod until the plan is complete
	if !autopilot.HasThePlanEnded(plan) {
		return fmt.Errorf("an autopilot upgrade is in progress (%s)", plan.Spec.ID)
	}

	if autopilot.HasPlanFailed(plan) {
		reason := autopilot.ReasonForState(plan)
		return fmt.Errorf("autopilot plan failed: %s", reason)
	}

	// check if this was actually a k0s upgrade plan, or just an image download plan
	isK0sUpgrade := false
	for _, command := range plan.Spec.Commands {
		if command.K0sUpdate != nil {
			isK0sUpgrade = true
			break
		}
	}
	// if this was not a k0s upgrade plan, we can just delete the plan and restart the function to get a k0s upgrade
	if !isK0sUpgrade {
		err = cli.Delete(ctx, &plan)
		if err != nil {
			return fmt.Errorf("delete autopilot plan: %w", err)
		}
		return k0sUpgrade(ctx, cli, in)
	}

	match, err = k8sutil.ClusterNodesMatchVersion(ctx, cli, desiredVersion)
	if err != nil {
		return fmt.Errorf("check cluster nodes match version after plan completion: %w", err)
	}
	if !match {
		return fmt.Errorf("cluster nodes did not match version after upgrade")
	}

	// the plan has been completed, so we can move on - kubernetes is now upgraded
	fmt.Printf("Upgrade to %s completed successfully\n", desiredVersion)
	if err := cli.Delete(ctx, &plan); err != nil {
		return fmt.Errorf("failed to delete successful upgrade plan: %w", err)
	}
	return nil
}

// copied from ReconcileHelmCharts in https://github.com/replicatedhq/embedded-cluster/blob/c6a57a4/operator/controllers/installation_controller.go#L568
func chartUpgrade(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) error {
	meta, err := release.MetadataFor(ctx, in, cli)
	if err != nil {
		return fmt.Errorf("failed to get release metadata: %w", err)
	}

	// fetch the current clusterConfig
	var clusterConfig k0sv1beta1.ClusterConfig
	if err := cli.Get(ctx, client.ObjectKey{Name: "k0s", Namespace: "kube-system"}, &clusterConfig); err != nil {
		return fmt.Errorf("failed to get cluster config: %w", err)
	}

	combinedConfigs, err := charts.K0sHelmExtensionsFromInstallation(ctx, in, meta, &clusterConfig)
	if err != nil {
		return fmt.Errorf("failed to get helm charts from installation: %w", err)
	}

	cfgs := &k0sv1beta1.HelmExtensions{}
	cfgs, err = v1beta1.ConvertTo(*combinedConfigs, cfgs)
	if err != nil {
		return fmt.Errorf("failed to convert chart types: %w", err)
	}

	existingHelm := &k0sv1beta1.HelmExtensions{}
	if clusterConfig.Spec != nil && clusterConfig.Spec.Extensions != nil && clusterConfig.Spec.Extensions.Helm != nil {
		existingHelm = clusterConfig.Spec.Extensions.Helm
	}

	chartDrift, changedCharts, err := charts.DetectChartDrift(cfgs, existingHelm)
	if err != nil {
		return fmt.Errorf("failed to check chart drift: %w", err)
	}

	// detect drift between the cluster config and the installer metadata
	var installedCharts k0shelm.ChartList
	if err := cli.List(ctx, &installedCharts); err != nil {
		return fmt.Errorf("failed to list installed charts: %w", err)
	}
	pendingCharts, chartErrors, err := charts.DetectChartCompletion(existingHelm, installedCharts)
	if err != nil {
		return fmt.Errorf("failed to check chart completion: %w", err)
	}

	// if there is a difference between what we want and what we have
	// we should update the cluster instead of letting chart errors stop deployment permanently
	// otherwise if there are errors we need to abort
	if len(chartErrors) > 0 && !chartDrift {
		chartErrorString := strings.Join(chartErrors, ",")
		chartErrorString = "failed to update helm charts: " + chartErrorString
		fmt.Printf("Chart errors: %s\n", chartErrorString)
		return fmt.Errorf("helm charts have errors and there is no update to be applied")
	}

	// If all addons match their target version + values, things are successful
	// This should not happen on upgrades
	if len(pendingCharts) == 0 && !chartDrift {
		return nil
	}

	if len(pendingCharts) > 0 {
		// If there are pending charts, return an error because we need to wait for some prior installation to complete
		return fmt.Errorf("pending charts: %v", pendingCharts)
	}

	if !chartDrift {
		// if there is no drift, we should not reapply the cluster config
		// This should not happen on upgrades
		return nil
	}

	// Replace the current chart configs with the new chart configs
	clusterConfig.Spec.Extensions.Helm = cfgs
	fmt.Printf("Updating cluster config with new helm charts %v\n", changedCharts)
	//Update the clusterConfig
	if err := cli.Update(ctx, &clusterConfig); err != nil {
		return fmt.Errorf("failed to update cluster config: %w", err)
	}
	return nil

}

func unLockInstallation(ctx context.Context, cli client.Client, in *v1beta1.Installation) error {
	existingInstallation := &v1beta1.Installation{}
	err := cli.Get(ctx, client.ObjectKey{Name: in.Name}, existingInstallation)
	if err != nil {
		return fmt.Errorf("get installation: %w", err)
	}

	existingInstallation.Spec = *in.Spec.DeepCopy() // copy the spec in, in case there were fields added to the spec
	err = cli.Update(ctx, existingInstallation)
	if err != nil {
		return fmt.Errorf("update installation: %w", err)
	}

	// if the installation is locked, we need to unlock it
	if existingInstallation.Status.State == v1beta1.InstallationStateWaiting {
		existingInstallation.Status.State = v1beta1.InstallationStateKubernetesInstalled
		err := cli.Status().Update(ctx, existingInstallation)
		if err != nil {
			return fmt.Errorf("update installation status: %w", err)
		}
	}
	return nil
}

func waitForOperatorChart(ctx context.Context, cli client.Client, version string) error {
	err := wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		ready, err := k8sutil.GetChartHealthVersion(ctx, cli, operatorChartName, version)
		if err != nil {
			return false, fmt.Errorf("get chart health: %w", err)
		}
		return ready, nil
	})
	return err
}
