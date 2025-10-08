package addons

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/pkg/errors"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type UpgradeOptions struct {
	ClusterID               string
	AdminConsolePort        int
	IsAirgap                bool
	IsHA                    bool
	DisasterRecoveryEnabled bool
	IsMultiNodeEnabled      bool
	EmbeddedConfigSpec      *ecv1beta1.ConfigSpec
	EndUserConfigSpec       *ecv1beta1.ConfigSpec
	ProxySpec               *ecv1beta1.ProxySpec
	HostCABundlePath        string
	DataDir                 string
	K0sDataDir              string
	OpenEBSDataDir          string
	SeaweedFSDataDir        string
	ServiceCIDR             string
}

func (a *AddOns) Upgrade(ctx context.Context, in *ecv1beta1.Installation, meta *ectypes.ReleaseMetadata, opts UpgradeOptions) error {
	addons, err := a.getAddOnsForUpgrade(meta, opts)
	if err != nil {
		return errors.Wrap(err, "get addons for upgrade")
	}

	for _, addon := range addons {
		a.sendProgress(addon.Name(), apitypes.StateRunning, "Upgrading")

		if err := a.upgradeAddOn(ctx, in, addon); err != nil {
			a.sendProgress(addon.Name(), apitypes.StateFailed, err.Error())
			return errors.Wrapf(err, "addon %s", addon.Name())
		}

		a.sendProgress(addon.Name(), apitypes.StateSucceeded, "Upgraded")
	}

	return nil
}

// Convenience function for getting the names of the addons to upgrade without having to provide the full upgrade options
func GetAddOnsNamesForUpgrade(isAirgap bool, disasterRecoveryEnabled bool, isHA bool) []string {
	addOns := []types.AddOn{
		&openebs.OpenEBS{},
		&embeddedclusteroperator.EmbeddedClusterOperator{},
	}

	if isAirgap {
		addOns = append(addOns, &registry.Registry{})

		if isHA {
			addOns = append(addOns, &seaweedfs.SeaweedFS{})
		}
	}

	if disasterRecoveryEnabled {
		addOns = append(addOns, &velero.Velero{})
	}

	addOns = append(addOns, &adminconsole.AdminConsole{})

	names := []string{}
	for _, addOn := range addOns {
		names = append(names, addOn.Name())
	}
	return names
}

func (a *AddOns) getAddOnsForUpgrade(meta *ectypes.ReleaseMetadata, opts UpgradeOptions) ([]types.AddOn, error) {
	addOns := []types.AddOn{
		&openebs.OpenEBS{
			OpenEBSDataDir: opts.OpenEBSDataDir,
		},
	}

	// ECO's embedded (wrong) metadata values do not match the published (correct) metadata values.
	// This is because we re-generate the metadata.yaml file _after_ building the ECO binary / image.
	// We do that because the SHA of the image needs to be included in the metadata.yaml file.
	// HACK: to work around this, override the embedded metadata values with the published ones.
	ecoChartLocation, ecoChartVersion, err := a.operatorChart(meta)
	if err != nil {
		return nil, errors.Wrap(err, "get operator chart location")
	}
	ecoImageRepo, ecoImageTag, ecoUtilsImage, err := a.operatorImages(meta.Images)
	if err != nil {
		return nil, errors.Wrap(err, "get operator images")
	}
	addOns = append(addOns, &embeddedclusteroperator.EmbeddedClusterOperator{
		ClusterID:        opts.ClusterID,
		IsAirgap:         opts.IsAirgap,
		Proxy:            opts.ProxySpec,
		HostCABundlePath: opts.HostCABundlePath,

		ChartLocationOverride: ecoChartLocation,
		ChartVersionOverride:  ecoChartVersion,
		ImageRepoOverride:     ecoImageRepo,
		ImageTagOverride:      ecoImageTag,
		UtilsImageOverride:    ecoUtilsImage,
	})

	if opts.IsAirgap {
		addOns = append(addOns, &registry.Registry{
			ServiceCIDR: opts.ServiceCIDR,
			IsHA:        opts.IsHA,
		})

		if opts.IsHA {
			addOns = append(addOns, &seaweedfs.SeaweedFS{
				ServiceCIDR:      opts.ServiceCIDR,
				SeaweedFSDataDir: opts.SeaweedFSDataDir,
			})
		}
	}

	if opts.DisasterRecoveryEnabled {
		addOns = append(addOns, &velero.Velero{
			Proxy:            opts.ProxySpec,
			HostCABundlePath: opts.HostCABundlePath,
			K0sDataDir:       opts.K0sDataDir,
		})
	}

	addOns = append(addOns, &adminconsole.AdminConsole{
		ClusterID:          opts.ClusterID,
		IsAirgap:           opts.IsAirgap,
		IsHA:               opts.IsHA,
		Proxy:              opts.ProxySpec,
		ServiceCIDR:        opts.ServiceCIDR,
		IsMultiNodeEnabled: opts.IsMultiNodeEnabled,
		HostCABundlePath:   opts.HostCABundlePath,
		DataDir:            opts.DataDir,
		K0sDataDir:         opts.K0sDataDir,
		AdminConsolePort:   opts.AdminConsolePort,
	})

	return addOns, nil
}

func (a *AddOns) upgradeAddOn(ctx context.Context, in *ecv1beta1.Installation, addon types.AddOn) error {
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

	err := addon.Upgrade(ctx, a.logf, a.kcli, a.mcli, a.hcli, a.domains, overrides)
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
