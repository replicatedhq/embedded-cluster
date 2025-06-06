package addons

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Upgrade(ctx context.Context, logf types.LogFunc, hcli helm.Client, rc runtimeconfig.RuntimeConfig, in *ecv1beta1.Installation, meta *ectypes.ReleaseMetadata) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return errors.Wrap(err, "create kube client")
	}

	mcli, err := kubeutils.MetadataClient()
	if err != nil {
		return errors.Wrap(err, "create metadata client")
	}

	opts := getUpgradeOpts(in.Spec)

	addons, err := getAddOnsForUpgrade(logf, hcli, kcli, mcli, rc, meta, opts)
	if err != nil {
		return errors.Wrap(err, "get addons for upgrade")
	}
	for _, addon := range addons {
		if err := upgradeAddOn(ctx, kcli, in, addon, opts); err != nil {
			return errors.Wrapf(err, "addon %s", addon.Name())
		}
	}

	return nil
}

func getUpgradeOpts(inSpec ecv1beta1.InstallationSpec) types.InstallOptions {
	serviceCIDR := ""
	if inSpec.Network != nil {
		serviceCIDR = inSpec.Network.ServiceCIDR
	}

	return types.InstallOptions{
		ClusterID:                 inSpec.ClusterID,
		IsAirgap:                  inSpec.AirGap,
		IsHA:                      inSpec.HighAvailability,
		Proxy:                     inSpec.Proxy,
		ServiceCIDR:               serviceCIDR,
		IsDisasterRecoveryEnabled: inSpec.LicenseInfo != nil && inSpec.LicenseInfo.IsDisasterRecoverySupported,
		IsMultiNodeEnabled:        inSpec.LicenseInfo != nil && inSpec.LicenseInfo.IsMultiNodeEnabled,
		EmbeddedConfigSpec:        inSpec.Config,
		Domains:                   runtimeconfig.GetDomains(inSpec.Config),

		// The following is unset on upgrades
		AdminConsolePassword: "",
		TLSCertBytes:         nil,
		TLSKeyBytes:          nil,
		Hostname:             "",
		// TODO (@salah): add support for end user overrides
		EndUserConfigSpec: nil,
		KotsInstaller:     nil,
	}
}

func getAddOnsForUpgrade(logf types.LogFunc, hcli helm.Client, kcli client.Client, mcli metadata.Interface, rc runtimeconfig.RuntimeConfig, meta *ectypes.ReleaseMetadata, opts types.InstallOptions) ([]types.AddOn, error) {
	addOns := []types.AddOn{
		openebs.New(
			openebs.WithLogFunc(logf),
			openebs.WithClients(kcli, mcli, hcli),
			openebs.WithRuntimeConfig(rc),
		),
	}

	// ECO's embedded (wrong) metadata values do not match the published (correct) metadata values.
	// This is because we re-generate the metadata.yaml file _after_ building the ECO binary / image.
	// We do that because the SHA of the image needs to be included in the metadata.yaml file.
	// HACK: to work around this, override the embedded metadata values with the published ones.
	ecoChartLocation, ecoChartVersion, err := operatorChart(meta)
	if err != nil {
		return nil, errors.Wrap(err, "get operator chart location")
	}
	ecoImageRepo, ecoImageTag, ecoUtilsImage, err := operatorImages(meta.Images, opts.Domains.ProxyRegistryDomain)
	if err != nil {
		return nil, errors.Wrap(err, "get operator images")
	}

	ecAddOn := embeddedclusteroperator.New(
		embeddedclusteroperator.WithLogFunc(logf),
		embeddedclusteroperator.WithClients(kcli, mcli, hcli),
		embeddedclusteroperator.WithRuntimeConfig(rc),
	)
	ecAddOn.ChartLocationOverride = ecoChartLocation
	ecAddOn.ChartVersionOverride = ecoChartVersion
	ecAddOn.ImageRepoOverride = ecoImageRepo
	ecAddOn.ImageTagOverride = ecoImageTag
	ecAddOn.UtilsImageOverride = ecoUtilsImage

	addOns = append(addOns, ecAddOn)

	if opts.IsAirgap {
		addOns = append(addOns, registry.New(
			registry.WithLogFunc(logf),
			registry.WithClients(kcli, mcli, hcli),
			registry.WithRuntimeConfig(rc),
		))

		if opts.IsHA {
			addOns = append(addOns, seaweedfs.New(
				seaweedfs.WithLogFunc(logf),
				seaweedfs.WithClients(kcli, mcli, hcli),
				seaweedfs.WithRuntimeConfig(rc),
			))
		}
	}

	if opts.IsDisasterRecoveryEnabled {
		addOns = append(addOns, velero.New(
			velero.WithLogFunc(logf),
			velero.WithClients(kcli, mcli, hcli),
			velero.WithRuntimeConfig(rc),
		))
	}

	addOns = append(addOns, adminconsole.New(
		adminconsole.WithLogFunc(logf),
		adminconsole.WithClients(kcli, mcli, hcli),
		adminconsole.WithRuntimeConfig(rc),
	))

	return addOns, nil
}

func upgradeAddOn(ctx context.Context, kcli client.Client, in *ecv1beta1.Installation, addon types.AddOn, opts types.InstallOptions) error {
	// check if we already processed this addon
	if kubeutils.CheckInstallationConditionStatus(in.Status, conditionName(addon)) == metav1.ConditionTrue {
		slog.Info("Addon is ready", "name", addon.Name(), "version", addon.Version())
		return nil
	}

	slog.Info("Upgrading addon", "name", addon.Name(), "version", addon.Version())

	// mark as processing
	if err := setCondition(ctx, kcli, in, conditionName(addon), metav1.ConditionFalse, "Upgrading", ""); err != nil {
		return errors.Wrap(err, "failed to set condition status")
	}

	overrides := addOnOverrides(addon, opts.EmbeddedConfigSpec, opts.EndUserConfigSpec)

	err := addon.Upgrade(ctx, opts, overrides)
	if err != nil {
		message := helpers.CleanErrorMessage(err)
		if err := setCondition(ctx, kcli, in, conditionName(addon), metav1.ConditionFalse, "UpgradeFailed", message); err != nil {
			slog.Error("Failed to set addon condition upgrade failed",
				"name", addon.Name(), "version", addon.Version(), "error", err,
			)
		}
		return errors.Wrap(err, "upgrade addon")
	}

	err = setCondition(ctx, kcli, in, conditionName(addon), metav1.ConditionTrue, "Upgraded", "")
	if err != nil {
		return errors.Wrap(err, "set condition upgrade succeeded")
	}

	slog.Info("Addon is ready", "name", addon.Name(), "version", addon.Version())
	return nil
}

func conditionName(addon types.AddOn) string {
	return fmt.Sprintf("%s-%s", addon.Namespace(), addon.ReleaseName())
}

func setCondition(ctx context.Context, kcli client.Client, in *ecv1beta1.Installation, conditionType string, status metav1.ConditionStatus, reason, message string) error {
	return kubeutils.SetInstallationConditionStatus(ctx, kcli, in, metav1.Condition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	})
}
