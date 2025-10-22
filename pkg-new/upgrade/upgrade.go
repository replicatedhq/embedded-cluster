package upgrade

import (
	"context"
	"fmt"
	"reflect"
	"time"

	apv1b2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/autopilot"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg-new/domains"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	addontypes "github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/config"
	"github.com/replicatedhq/embedded-cluster/pkg/extensions"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/support"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Upgrade upgrades the embedded cluster to the version specified in the installation.
// First the k0s cluster is upgraded, then addon charts are upgraded, and finally the installation is unlocked.
func Upgrade(ctx context.Context, cli client.Client, hcli helm.Client, rc runtimeconfig.RuntimeConfig, in *ecv1beta1.Installation, logger logrus.FieldLogger) error {
	logger.WithField("version", in.Spec.Config.Version).Info("Upgrading Embedded Cluster")

	// Augment the installation with data dirs that may not be present in the previous version.
	// This is important to do ahead of updating the cluster config.
	// We still cannot update the installation object as the CRDs are not updated yet.
	in, err := maybeOverrideInstallationDataDirs(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("override installation data dirs: %w", err)
	}

	// In case the previous version was < 1.15, update the runtime config after we override the
	// installation data dirs from the previous installation.
	rc.Set(in.Spec.RuntimeConfig)

	err = upgradeK0s(ctx, cli, rc, in, logger)
	if err != nil {
		return fmt.Errorf("k0s upgrade: %w", err)
	}

	// We must update the cluster config after we upgrade k0s as it is possible that the schema
	// between versions has changed. One drawback of this is that the sandbox (pause) image does
	// not get updated, and possibly others but I cannot confirm this.
	err = updateClusterConfig(ctx, cli, in, logger)
	if err != nil {
		return fmt.Errorf("cluster config update: %w", err)
	}

	logger.Info("Upgrading addons")
	err = upgradeAddons(ctx, cli, hcli, rc, in, nil, logger)
	if err != nil {
		return fmt.Errorf("upgrade addons: %w", err)
	}

	logger.Info("Upgrading extensions")
	err = upgradeExtensions(ctx, cli, hcli, in, logger)
	if err != nil {
		return fmt.Errorf("upgrade extensions: %w", err)
	}

	err = support.CreateHostSupportBundle(ctx, cli)
	if err != nil {
		logger.WithError(err).Error("Failed to upgrade host support bundle")
	}

	err = kubeutils.SetInstallationState(ctx, cli, in, ecv1beta1.InstallationStateInstalled, "Installed")
	if err != nil {
		return fmt.Errorf("set installation state: %w", err)
	}

	return nil
}

func maybeOverrideInstallationDataDirs(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) (*ecv1beta1.Installation, error) {
	previous, err := kubeutils.GetPreviousInstallation(ctx, cli, in)
	if err != nil {
		return in, fmt.Errorf("get latest installation: %w", err)
	}
	next, _, err := kubeutils.MaybeOverrideInstallationDataDirs(*in, previous)
	if err != nil {
		return in, fmt.Errorf("override installation data dirs: %w", err)
	}
	return &next, nil
}

func upgradeK0s(ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig, in *ecv1beta1.Installation, logger logrus.FieldLogger) error {
	meta, err := release.MetadataFor(ctx, in, cli)
	if err != nil {
		return fmt.Errorf("get release metadata: %w", err)
	}

	// check if the k0s version is the same as the current version
	// if it is, we can skip the upgrade
	desiredVersion := k0sVersionFromMetadata(meta)

	match, err := clusterNodesMatchVersion(ctx, cli, desiredVersion)
	if err != nil {
		return fmt.Errorf("check cluster nodes match version: %w", err)
	}
	if match {
		return nil
	}

	logger.WithField("version", desiredVersion).Info("Upgrading k0s")

	if err := kubeutils.SetInstallationState(ctx, cli, in, ecv1beta1.InstallationStateInstalling, "Upgrading Kubernetes", ""); err != nil {
		return fmt.Errorf("update installation status: %w", err)
	}

	// create an autopilot upgrade plan if one does not yet exist
	if err := createAutopilotPlan(ctx, cli, rc, desiredVersion, in, meta, logger); err != nil {
		return fmt.Errorf("create autpilot upgrade plan: %w", err)
	}

	plan, err := waitForAutopilotPlan(ctx, cli, logger)
	if err != nil {
		return fmt.Errorf("wait for autpilot plan: %w", err)
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
		return upgradeK0s(ctx, cli, rc, in, logger)
	}

	match, err = clusterNodesMatchVersion(ctx, cli, desiredVersion)
	if err != nil {
		return fmt.Errorf("check cluster nodes match version after plan completion: %w", err)
	}
	if !match {
		return fmt.Errorf("cluster nodes did not match version after upgrade")
	}

	// the plan has been completed, so we can move on - kubernetes is now upgraded
	logger.WithField("version", desiredVersion).Info("Upgrade completed successfully")
	if err := cli.Delete(ctx, &plan); err != nil {
		return fmt.Errorf("delete successful upgrade plan: %w", err)
	}

	err = kubeutils.SetInstallationState(ctx, cli, in, ecv1beta1.InstallationStateKubernetesInstalled, "Kubernetes upgraded")
	if err != nil {
		return fmt.Errorf("set installation state: %w", err)
	}

	return nil
}

// updateClusterConfig updates the cluster config with the latest images.
func updateClusterConfig(ctx context.Context, cli client.Client, in *ecv1beta1.Installation, logger logrus.FieldLogger) error {
	var currentCfg k0sv1beta1.ClusterConfig
	err := cli.Get(ctx, client.ObjectKey{Name: "k0s", Namespace: "kube-system"}, &currentCfg)
	if err != nil {
		return fmt.Errorf("get cluster config: %w", err)
	}

	// TODO: This will not work in a non-production environment.
	// The domains in the release are used to supply alternative defaults for staging and the dev environment.
	// The GetDomains function will always fall back to production defaults.
	domains := domains.GetDomains(in.Spec.Config, nil)

	didUpdate := false

	cfg := config.RenderK0sConfig(domains.ProxyRegistryDomain)
	if currentCfg.Spec.Images != nil {
		if !reflect.DeepEqual(*currentCfg.Spec.Images, *cfg.Spec.Images) {
			currentCfg.Spec.Images = cfg.Spec.Images
			didUpdate = true
		}
	}

	if currentCfg.Spec.Network != nil &&
		currentCfg.Spec.Network.NodeLocalLoadBalancing != nil &&
		currentCfg.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy != nil &&
		currentCfg.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image != nil {
		if !reflect.DeepEqual(
			*currentCfg.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image,
			*cfg.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image,
		) {
			currentCfg.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image = cfg.Spec.Network.NodeLocalLoadBalancing.EnvoyProxy.Image
			didUpdate = true
		}
	}

	// Apply unsupported overrides from the installation
	if (in.Spec.Config != nil && in.Spec.Config.UnsupportedOverrides.K0s != "") || in.Spec.EndUserK0sConfigOverrides != "" {
		newCfg := currentCfg.DeepCopy()

		if in.Spec.Config != nil && in.Spec.Config.UnsupportedOverrides.K0s != "" {
			newCfg, err = config.PatchK0sConfig(newCfg, in.Spec.Config.UnsupportedOverrides.K0s, true)
			if err != nil {
				return fmt.Errorf("apply vendor unsupported overrides: %w", err)
			}
		}

		if in.Spec.EndUserK0sConfigOverrides != "" {
			newCfg, err = config.PatchK0sConfig(newCfg, in.Spec.EndUserK0sConfigOverrides, true)
			if err != nil {
				return fmt.Errorf("apply end user unsupported overrides: %w", err)
			}
		}

		// Deduplicate API SANs before comparing configs. K0s cluster config does not allow duplicates.
		if newCfg.Spec.API != nil && len(newCfg.Spec.API.SANs) > 0 {
			newCfg.Spec.API.SANs = helpers.UniqueStringSlice(newCfg.Spec.API.SANs)
		}

		// check if the new config is different from the current config
		if !reflect.DeepEqual(*newCfg, currentCfg) {
			currentCfg = *newCfg
			didUpdate = true
		}
	}

	if !didUpdate {
		return nil
	}

	unstructured, err := helpers.K0sClusterConfigTo129Compat(&currentCfg)
	if err != nil {
		return fmt.Errorf("convert cluster config to 1.29 compat: %w", err)
	}

	err = cli.Update(ctx, unstructured)
	if err != nil {
		return fmt.Errorf("update cluster config: %w", err)
	}
	logger.Info("Updated cluster config with new images")

	return nil
}

func upgradeAddons(ctx context.Context, cli client.Client, hcli helm.Client, rc runtimeconfig.RuntimeConfig, in *ecv1beta1.Installation, progressChan chan addontypes.AddOnProgress, logger logrus.FieldLogger) error {
	err := kubeutils.SetInstallationState(ctx, cli, in, ecv1beta1.InstallationStateAddonsInstalling, "Upgrading addons")
	if err != nil {
		return fmt.Errorf("set installation state: %w", err)
	}

	meta, err := release.MetadataFor(ctx, in, cli)
	if err != nil {
		return fmt.Errorf("get release metadata: %w", err)
	}
	if meta == nil || meta.Images == nil {
		return fmt.Errorf("no images available")
	}

	mcli, err := kubeutils.MetadataClient()
	if err != nil {
		return fmt.Errorf("create metadata client: %w", err)
	}

	// TODO: This will not work in a non-production environment.
	// The domains in the release are used to supply alternative defaults for staging and the dev environment.
	// The GetDomains function will always fall back to production defaults.
	domains := domains.GetDomains(in.Spec.Config, nil)

	addOns := addons.New(
		addons.WithLogFunc(logger.Infof),
		addons.WithKubernetesClient(cli),
		addons.WithMetadataClient(mcli),
		addons.WithHelmClient(hcli),
		addons.WithDomains(domains),
		addons.WithProgressChannel(progressChan),
	)

	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, cli)
	if err != nil {
		return fmt.Errorf("get kotsadm namespace: %w", err)
	}

	opts := addons.UpgradeOptions{
		ClusterID:               in.Spec.ClusterID,
		AdminConsolePort:        rc.AdminConsolePort(),
		IsAirgap:                in.Spec.AirGap,
		IsHA:                    in.Spec.HighAvailability,
		DisasterRecoveryEnabled: in.Spec.LicenseInfo != nil && in.Spec.LicenseInfo.IsDisasterRecoverySupported,
		IsMultiNodeEnabled:      in.Spec.LicenseInfo != nil && in.Spec.LicenseInfo.IsMultiNodeEnabled,
		EmbeddedConfigSpec:      in.Spec.Config,
		EndUserConfigSpec:       nil, // TODO: add support for end user config spec
		ProxySpec:               rc.ProxySpec(),
		HostCABundlePath:        rc.HostCABundlePath(),
		KotsadmNamespace:        kotsadmNamespace,
		DataDir:                 rc.EmbeddedClusterHomeDirectory(),
		K0sDataDir:              rc.EmbeddedClusterK0sSubDir(),
		OpenEBSDataDir:          rc.EmbeddedClusterOpenEBSLocalSubDir(),
		SeaweedFSDataDir:        rc.EmbeddedClusterSeaweedFSSubDir(),
		ServiceCIDR:             rc.ServiceCIDR(),
	}

	if err := addOns.Upgrade(ctx, in, meta, opts); err != nil {
		return fmt.Errorf("upgrade addons: %w", err)
	}

	err = kubeutils.SetInstallationState(ctx, cli, in, ecv1beta1.InstallationStateAddonsInstalled, "Addons upgraded")
	if err != nil {
		return fmt.Errorf("set installation state: %w", err)
	}

	return nil
}

func upgradeExtensions(ctx context.Context, cli client.Client, hcli helm.Client, in *ecv1beta1.Installation, logger logrus.FieldLogger) error {
	err := kubeutils.SetInstallationState(ctx, cli, in, ecv1beta1.InstallationStateAddonsInstalling, "Upgrading extensions")
	if err != nil {
		return fmt.Errorf("set installation state: %w", err)
	}

	previous, err := kubeutils.GetPreviousInstallation(ctx, cli, in)
	if err != nil {
		return fmt.Errorf("get previous installation: %w", err)
	}

	if err := extensions.Upgrade(ctx, cli, hcli, previous, in, logger); err != nil {
		return fmt.Errorf("upgrade extensions: %w", err)
	}

	err = kubeutils.SetInstallationState(ctx, cli, in, ecv1beta1.InstallationStateAddonsInstalled, "Extensions upgraded")
	if err != nil {
		return fmt.Errorf("set installation state: %w", err)
	}

	return nil
}

func createAutopilotPlan(ctx context.Context, cli client.Client, rc runtimeconfig.RuntimeConfig, desiredVersion string, in *ecv1beta1.Installation, meta *ectypes.ReleaseMetadata, logger logrus.FieldLogger) error {
	var plan apv1b2.Plan
	okey := client.ObjectKey{Name: "autopilot"}
	if err := cli.Get(ctx, okey, &plan); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("get upgrade plan: %w", err)
	} else if errors.IsNotFound(err) {
		// if the kubernetes version has changed we create an upgrade command
		logger.WithField("version", desiredVersion).Info("Starting k0s autopilot upgrade plan")

		// there is no autopilot plan in the cluster so we are free to
		// start our own plan. here we link the plan to the installation
		// by its name.
		if err := startAutopilotUpgrade(ctx, cli, rc, in, meta); err != nil {
			return fmt.Errorf("start upgrade: %w", err)
		}
	}
	return nil
}

func waitForAutopilotPlan(ctx context.Context, cli client.Client, logger logrus.FieldLogger) (apv1b2.Plan, error) {
	for {
		var plan apv1b2.Plan
		if err := cli.Get(ctx, client.ObjectKey{Name: "autopilot"}, &plan); err != nil {
			return plan, fmt.Errorf("get upgrade plan: %w", err)
		}
		if autopilot.HasThePlanEnded(plan) {
			return plan, nil
		}
		logger.WithField("plan_id", plan.Spec.ID).Info("An autopilot upgrade is in progress")
		time.Sleep(5 * time.Second)
	}
}
