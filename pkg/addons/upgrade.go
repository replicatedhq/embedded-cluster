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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Upgrade(
	ctx context.Context, logf types.LogFunc, clients types.Clients,
	in *ecv1beta1.Installation, meta *ectypes.ReleaseMetadata,
) error {
	addons, err := getAddOnsForUpgrade(logf, in.Spec, meta)
	if err != nil {
		return errors.Wrap(err, "get addons for upgrade")
	}

	for _, addon := range addons {
		if err := upgradeAddOn(ctx, clients, in, addon); err != nil {
			return errors.Wrapf(err, "addon %s", addon.Name())
		}
	}

	return nil
}

func getAddOnsForUpgrade(logf types.LogFunc, inSpec ecv1beta1.InstallationSpec, meta *ectypes.ReleaseMetadata) ([]types.AddOn, error) {
	domains := runtimeconfig.GetDomains(inSpec.Config)

	addOns := []types.AddOn{
		openebs.New(
			openebs.WithLogFunc(logf),
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
	ecoImageRepo, ecoImageTag, ecoUtilsImage, err := operatorImages(meta.Images, domains.ProxyRegistryDomain)
	if err != nil {
		return nil, errors.Wrap(err, "get operator images")
	}

	ecAddOn := embeddedclusteroperator.New(
		embeddedclusteroperator.WithLogFunc(logf),
	)
	ecAddOn.ChartLocationOverride = ecoChartLocation
	ecAddOn.ChartVersionOverride = ecoChartVersion
	ecAddOn.ImageRepoOverride = ecoImageRepo
	ecAddOn.ImageTagOverride = ecoImageTag
	ecAddOn.UtilsImageOverride = ecoUtilsImage

	addOns = append(addOns, ecAddOn)

	if inSpec.AirGap {
		addOns = append(addOns, registry.New(
			registry.WithLogFunc(logf),
		))

		if inSpec.HighAvailability {
			addOns = append(addOns, seaweedfs.New(
				seaweedfs.WithLogFunc(logf),
			))
		}
	}

	if inSpec.LicenseInfo != nil && inSpec.LicenseInfo.IsDisasterRecoverySupported {
		addOns = append(addOns, velero.New(
			velero.WithLogFunc(logf),
		))
	}

	addOns = append(addOns, adminconsole.New(
		adminconsole.WithLogFunc(logf),
	))

	return addOns, nil
}

func upgradeAddOn(ctx context.Context, clients types.Clients, in *ecv1beta1.Installation, addon types.AddOn) error {
	// check if we already processed this addon
	if kubeutils.CheckInstallationConditionStatus(in.Status, conditionName(addon)) == metav1.ConditionTrue {
		slog.Info("Addon is ready", "name", addon.Name(), "version", addon.Version())
		return nil
	}

	slog.Info("Upgrading addon", "name", addon.Name(), "version", addon.Version())

	// mark as processing
	if err := setCondition(ctx, clients.K8sClient, in, conditionName(addon), metav1.ConditionFalse, "Upgrading", ""); err != nil {
		return errors.Wrap(err, "failed to set condition status")
	}

	overrides := addOnOverrides(addon, in.Spec.Config, nil)

	err := addon.Upgrade(ctx, clients, in.Spec, overrides)
	if err != nil {
		message := helpers.CleanErrorMessage(err)
		if err := setCondition(ctx, clients.K8sClient, in, conditionName(addon), metav1.ConditionFalse, "UpgradeFailed", message); err != nil {
			slog.Error("Failed to set addon condition upgrade failed",
				"name", addon.Name(), "version", addon.Version(), "error", err,
			)
		}
		return errors.Wrap(err, "upgrade addon")
	}

	err = setCondition(ctx, clients.K8sClient, in, conditionName(addon), metav1.ConditionTrue, "Upgraded", "")
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
