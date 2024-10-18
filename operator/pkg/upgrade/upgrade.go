package upgrade

import (
	"context"
	"fmt"
	"reflect"
	"time"

	apv1b2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	clusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/autopilot"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/charts"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/k8sutil"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/registry"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
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
	fmt.Printf("Upgrading to version %s\n", in.Spec.Config.Version)
	err := clusterConfigUpdate(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("cluster config update: %w", err)
	}

	err = k0sUpgrade(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("k0s upgrade: %w", err)
	}

	fmt.Printf("Updating registry migration status\n")
	err = registryMigrationStatus(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("registry migration status: %w", err)
	}

	fmt.Printf("Upgrading addons\n")
	err = chartUpgrade(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("chart upgrade: %w", err)
	}

	fmt.Printf("Waiting for operator chart to be ready\n")
	// wait for the operator chart to be ready
	err = waitForOperatorChart(ctx, cli, in.Spec.Config.Version)
	if err != nil {
		return fmt.Errorf("wait for operator chart: %w", err)
	}

	fmt.Printf("Re-applying installation\n")
	err = reApplyInstallation(ctx, cli, in)
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

	fmt.Printf("Upgrading k0s to version %s\n", desiredVersion)

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
	isOurK0sUpgrade := false
	for _, command := range plan.Spec.Commands {
		if command.K0sUpdate != nil {
			if command.K0sUpdate.Version == meta.Versions["Kubernetes"] {
				isOurK0sUpgrade = true
				break
			}
		}
	}
	// if this was not a k0s upgrade plan, we can just delete the plan and restart the function to get a k0s upgrade
	if !isOurK0sUpgrade {
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

	err = setInstallationState(ctx, cli, in.Name, v1beta1.InstallationStateKubernetesInstalled, "Kubernetes upgraded")
	if err != nil {
		return fmt.Errorf("set installation state: %w", err)
	}

	return nil
}

// clusterConfigUpdate updates the cluster config with the latest images.
func clusterConfigUpdate(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) error {
	var currentCfg k0sv1beta1.ClusterConfig
	err := cli.Get(ctx, client.ObjectKey{Name: "k0s", Namespace: "kube-system"}, &currentCfg)
	if err != nil {
		return fmt.Errorf("get cluster config: %w", err)
	}

	cfg := config.RenderK0sConfig()
	if currentCfg.Spec.Images != nil {
		if reflect.DeepEqual(*currentCfg.Spec.Images, *cfg.Spec.Images) {
			return nil
		}
	}

	currentCfg.Spec.Images = cfg.Spec.Images

	err = cli.Update(ctx, &currentCfg)
	if err != nil {
		return fmt.Errorf("update cluster config: %w", err)
	}
	fmt.Println("Updated cluster config with new images")

	return nil
}

// registryMigrationStatus ensures that the registry migration complete condition is set orior to
// reconciling the helm charts. Otherwise, the registry chart will revert back to non-ha mode.
func registryMigrationStatus(ctx context.Context, cli client.Client, in *clusterv1beta1.Installation) error {
	if in == nil || !in.Spec.AirGap || !in.Spec.HighAvailability {
		return nil
	}

	_, err := registry.EnsureRegistryMigrationCompleteCondition(ctx, in, cli)
	if err != nil {
		return fmt.Errorf("ensure registry migration complete condition: %w", err)
	}
	return nil
}

func chartUpgrade(ctx context.Context, cli client.Client, original *clusterv1beta1.Installation) error {
	input := original.DeepCopy()
	input.Status.SetState(v1beta1.InstallationStateKubernetesInstalled, "", nil)

	_, err := charts.ReconcileHelmCharts(ctx, cli, input)
	if err != nil {
		return fmt.Errorf("failed to reconcile helm charts: %w", err)
	}

	// check the status and return an error if appropriate
	// 'InstallationStateAddonsInstalling' is the one we expect to be set
	if input.Status.State != v1beta1.InstallationStateAddonsInstalling && input.Status.State != v1beta1.InstallationStateInstalled {
		return fmt.Errorf("got unexpected state %s with message %s reconciling charts", input.Status.State, input.Status.Reason)
	}

	err = setInstallationState(ctx, cli, original.Name, input.Status.State, input.Status.Reason, input.Status.PendingCharts...)
	if err != nil {
		return fmt.Errorf("set installation state: %w", err)
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
