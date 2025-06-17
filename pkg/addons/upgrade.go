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
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (a *AddOns) Upgrade(ctx context.Context, in *ecv1beta1.Installation, meta *ectypes.ReleaseMetadata) error {
	domains := runtimeconfig.GetDomains(in.Spec.Config)

	addons, err := a.getAddOnsForUpgrade(domains, in, meta)
	if err != nil {
		return errors.Wrap(err, "get addons for upgrade")
	}

	for _, addon := range addons {
		if err := a.upgradeAddOn(ctx, domains, in, addon); err != nil {
			return errors.Wrapf(err, "addon %s", addon.Name())
		}
	}

	return nil
}

func (a *AddOns) getAddOnsForUpgrade(domains ecv1beta1.Domains, in *ecv1beta1.Installation, meta *ectypes.ReleaseMetadata) ([]types.AddOn, error) {
	addOns := []types.AddOn{
		&openebs.OpenEBS{},
	}

	serviceCIDR := a.rc.ServiceCIDR()

	// ECO's embedded (wrong) metadata values do not match the published (correct) metadata values.
	// This is because we re-generate the metadata.yaml file _after_ building the ECO binary / image.
	// We do that because the SHA of the image needs to be included in the metadata.yaml file.
	// HACK: to work around this, override the embedded metadata values with the published ones.
	ecoChartLocation, ecoChartVersion, err := a.operatorChart(meta)
	if err != nil {
		return nil, errors.Wrap(err, "get operator chart location")
	}
	ecoImageRepo, ecoImageTag, ecoUtilsImage, err := a.operatorImages(meta.Images, domains.ProxyRegistryDomain)
	if err != nil {
		return nil, errors.Wrap(err, "get operator images")
	}
	addOns = append(addOns, &embeddedclusteroperator.EmbeddedClusterOperator{
		IsAirgap:              in.Spec.AirGap,
		Proxy:                 a.rc.ProxySpec(),
		ChartLocationOverride: ecoChartLocation,
		ChartVersionOverride:  ecoChartVersion,
		ImageRepoOverride:     ecoImageRepo,
		ImageTagOverride:      ecoImageTag,
		UtilsImageOverride:    ecoUtilsImage,
	})

	if in.Spec.AirGap {
		addOns = append(addOns, &registry.Registry{
			ServiceCIDR: serviceCIDR,
			IsHA:        in.Spec.HighAvailability,
		})

		if in.Spec.HighAvailability {
			addOns = append(addOns, &seaweedfs.SeaweedFS{
				ServiceCIDR: serviceCIDR,
			})
		}
	}

	if in.Spec.LicenseInfo != nil && in.Spec.LicenseInfo.IsDisasterRecoverySupported {
		addOns = append(addOns, &velero.Velero{
			Proxy: a.rc.ProxySpec(),
		})
	}

	addOns = append(addOns, &adminconsole.AdminConsole{
		IsAirgap:           in.Spec.AirGap,
		IsHA:               in.Spec.HighAvailability,
		Proxy:              a.rc.ProxySpec(),
		ServiceCIDR:        serviceCIDR,
		IsMultiNodeEnabled: in.Spec.LicenseInfo != nil && in.Spec.LicenseInfo.IsMultiNodeEnabled,
	})

	return addOns, nil
}

func (a *AddOns) upgradeAddOn(ctx context.Context, domains ecv1beta1.Domains, in *ecv1beta1.Installation, addon types.AddOn) error {
	// check if we already processed this addon
	if kubeutils.CheckInstallationConditionStatus(in.Status, a.conditionName(addon)) == metav1.ConditionTrue {
		slog.Info(addon.Name() + " is ready")
		return nil
	}

	slog.Info("Upgrading addon", "name", addon.Name(), "version", addon.Version())

	// mark as processing
	if err := a.setCondition(ctx, in, a.conditionName(addon), metav1.ConditionFalse, "Upgrading", ""); err != nil {
		return errors.Wrap(err, "failed to set condition status")
	}

	// TODO (@salah): add support for end user overrides
	overrides := a.addOnOverrides(addon, in.Spec.Config, nil)

	err := addon.Upgrade(ctx, a.logf, a.kcli, a.mcli, a.hcli, a.rc, domains, overrides)
	if err != nil {
		message := helpers.CleanErrorMessage(err)
		if err := a.setCondition(ctx, in, a.conditionName(addon), metav1.ConditionFalse, "UpgradeFailed", message); err != nil {
			slog.Error("Failed to set condition upgrade failed", "error", err)
		}
		return errors.Wrap(err, "upgrade addon")
	}

	err = a.setCondition(ctx, in, a.conditionName(addon), metav1.ConditionTrue, "Upgraded", "")
	if err != nil {
		return errors.Wrap(err, "set condition upgrade succeeded")
	}

	slog.Info(addon.Name() + " is ready")
	return nil
}

func (a *AddOns) conditionName(addon types.AddOn) string {
	return fmt.Sprintf("%s-%s", addon.Namespace(), addon.ReleaseName())
}

func (a *AddOns) setCondition(ctx context.Context, in *ecv1beta1.Installation, conditionType string, status metav1.ConditionStatus, reason, message string) error {
	return kubeutils.SetInstallationConditionStatus(ctx, a.kcli, in, metav1.Condition{
		Type:    conditionType,
		Status:  status,
		Reason:  reason,
		Message: message,
	})
}
