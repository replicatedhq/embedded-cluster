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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Upgrade(ctx context.Context, hcli helm.Client, in *ecv1beta1.Installation, meta *ectypes.ReleaseMetadata) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return errors.Wrap(err, "create kube client")
	}

	addons, err := getAddOnsForUpgrade(in, meta)
	if err != nil {
		return errors.Wrap(err, "get addons for upgrade")
	}
	for _, addon := range addons {
		if err := upgradeAddOn(ctx, hcli, kcli, in, addon); err != nil {
			return errors.Wrapf(err, "addon %s", addon.Name())
		}
	}

	return nil
}

func getAddOnsForUpgrade(in *ecv1beta1.Installation, meta *ectypes.ReleaseMetadata) ([]types.AddOn, error) {
	var replicatedAppDomain, proxyRegistryDomain, replicatedRegistryDomain string
	if in.Spec.Config != nil {
		replicatedAppDomain = in.Spec.Config.Domains.ReplicatedAppDomain
		proxyRegistryDomain = in.Spec.Config.Domains.ProxyRegistryDomain
		replicatedRegistryDomain = in.Spec.Config.Domains.ReplicatedRegistryDomain
	}

	fmt.Printf("DEBUG: using proxy domain %s\n", proxyRegistryDomain)

	addOns := []types.AddOn{
		&openebs.OpenEBS{
			ProxyRegistryDomain: proxyRegistryDomain,
		},
	}

	serviceCIDR := ""
	if in.Spec.Network != nil {
		serviceCIDR = in.Spec.Network.ServiceCIDR
	}

	// ECO's embedded (wrong) metadata values do not match the published (correct) metadata values.
	// This is because we re-generate the metadata.yaml file _after_ building the ECO binary / image.
	// We do that because the SHA of the image needs to be included in the metadata.yaml file.
	// HACK: to work around this, override the embedded metadata values with the published ones.
	ecoChartLocation, ecoChartVersion, err := operatorChart(meta)
	if err != nil {
		return nil, errors.Wrap(err, "get operator chart location")
	}
	ecoImageRepo, ecoImageTag, ecoUtilsImage, err := operatorImages(meta.Images, proxyRegistryDomain)
	if err != nil {
		return nil, errors.Wrap(err, "get operator images")
	}
	addOns = append(addOns, &embeddedclusteroperator.EmbeddedClusterOperator{
		IsAirgap:              in.Spec.AirGap,
		Proxy:                 in.Spec.Proxy,
		ChartLocationOverride: ecoChartLocation,
		ChartVersionOverride:  ecoChartVersion,
		BinaryNameOverride:    in.Spec.BinaryName,
		ImageRepoOverride:     ecoImageRepo,
		ImageTagOverride:      ecoImageTag,
		UtilsImageOverride:    ecoUtilsImage,
		ProxyRegistryDomain:   proxyRegistryDomain,
	})

	if in.Spec.AirGap {
		addOns = append(addOns, &registry.Registry{
			ServiceCIDR:         serviceCIDR,
			IsHA:                in.Spec.HighAvailability,
			ProxyRegistryDomain: proxyRegistryDomain,
		})

		if in.Spec.HighAvailability {
			addOns = append(addOns, &seaweedfs.SeaweedFS{
				ServiceCIDR:         serviceCIDR,
				ProxyRegistryDomain: proxyRegistryDomain,
			})
		}
	}

	if in.Spec.LicenseInfo != nil && in.Spec.LicenseInfo.IsDisasterRecoverySupported {
		addOns = append(addOns, &velero.Velero{
			Proxy:               in.Spec.Proxy,
			ProxyRegistryDomain: proxyRegistryDomain,
		})
	}

	addOns = append(addOns, &adminconsole.AdminConsole{
		IsAirgap:                 in.Spec.AirGap,
		IsHA:                     in.Spec.HighAvailability,
		Proxy:                    in.Spec.Proxy,
		ServiceCIDR:              serviceCIDR,
		ReplicatedAppDomain:      replicatedAppDomain,
		ProxyRegistryDomain:      proxyRegistryDomain,
		ReplicatedRegistryDomain: replicatedRegistryDomain,
	})

	return addOns, nil
}

func upgradeAddOn(ctx context.Context, hcli helm.Client, kcli client.Client, in *ecv1beta1.Installation, addon types.AddOn) error {
	// check if we already processed this addon
	if kubeutils.CheckInstallationConditionStatus(in.Status, conditionName(addon)) == metav1.ConditionTrue {
		slog.Info(addon.Name() + " is ready!")
		return nil
	}

	slog.Info("Upgrading addon", "name", addon.Name(), "version", addon.Version())

	// mark as processing
	if err := setCondition(ctx, kcli, in, conditionName(addon), metav1.ConditionFalse, "Upgrading", ""); err != nil {
		return errors.Wrap(err, "failed to set condition status")
	}

	// TODO (@salah): add support for end user overrides
	overrides := addOnOverrides(addon, in.Spec.Config, nil)

	err := addon.Upgrade(ctx, kcli, hcli, overrides)
	if err != nil {
		message := helpers.CleanErrorMessage(err)
		if err := setCondition(ctx, kcli, in, conditionName(addon), metav1.ConditionFalse, "UpgradeFailed", message); err != nil {
			slog.Error("Failed to set condition upgrade failed", "error", err)
		}
		return errors.Wrap(err, "upgrade addon")
	}

	err = setCondition(ctx, kcli, in, conditionName(addon), metav1.ConditionTrue, "Upgraded", "")
	if err != nil {
		return errors.Wrap(err, "set condition upgrade succeeded")
	}

	slog.Info(addon.Name() + " is ready!")
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
