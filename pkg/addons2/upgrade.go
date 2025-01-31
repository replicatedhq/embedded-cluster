package addons2

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/operator/pkg/k8sutil"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/seaweedfs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Upgrade(ctx context.Context, in *ecv1beta1.Installation, meta *ectypes.ReleaseMetadata) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return errors.Wrap(err, "create kube client")
	}

	airgapChartsPath := ""
	if in.Spec.AirGap {
		airgapChartsPath = runtimeconfig.EmbeddedClusterChartsSubDir()
	}

	hcli, err := helm.NewHelm(helm.HelmOptions{
		K0sVersion: versions.K0sVersion,
		AirgapPath: airgapChartsPath,
	})
	if err != nil {
		return errors.Wrap(err, "create helm client")
	}

	addons, err := getAddOnsForUpgrade(in, meta)
	if err != nil {
		return errors.Wrap(err, "get addons for upgrade")
	}
	for _, addon := range addons {
		if err := upgradeAddOn(ctx, hcli, kcli, in, addon); err != nil {
			return err
		}
	}

	return nil
}

func getAddOnsForUpgrade(in *ecv1beta1.Installation, meta *ectypes.ReleaseMetadata) ([]types.AddOn, error) {
	addOns := []types.AddOn{
		&openebs.OpenEBS{},
	}

	ecoRepo, ecoTag, ecoUtilsImage, err := operatorImages(meta.Images)
	if err != nil {
		return nil, errors.Wrap(err, "get operator images")
	}
	addOns = append(addOns, &embeddedclusteroperator.EmbeddedClusterOperator{
		IsAirgap:           in.Spec.AirGap,
		Proxy:              in.Spec.Proxy,
		BinaryNameOverride: in.Spec.BinaryName,
		ImageRepoOverride:  ecoRepo,
		ImageTagOverride:   ecoTag,
		UtilsImageOverride: ecoUtilsImage,
	})

	if in.Spec.AirGap {
		addOns = append(addOns, &registry.Registry{
			ServiceCIDR: in.Spec.Network.ServiceCIDR,
		})

		if in.Spec.HighAvailability {
			addOns = append(addOns, &seaweedfs.SeaweedFS{
				ServiceCIDR: in.Spec.Network.ServiceCIDR,
			})
		}
	}

	if in.Spec.LicenseInfo.IsDisasterRecoverySupported {
		addOns = append(addOns, &velero.Velero{
			Proxy: in.Spec.Proxy,
		})
	}

	addOns = append(addOns, &adminconsole.AdminConsole{
		IsAirgap:    in.Spec.AirGap,
		IsHA:        in.Spec.HighAvailability,
		Proxy:       in.Spec.Proxy,
		ServiceCIDR: in.Spec.Network.ServiceCIDR,
	})

	return addOns, nil
}

func upgradeAddOn(ctx context.Context, hcli *helm.Helm, kcli client.Client, in *ecv1beta1.Installation, addon types.AddOn) (finalErr error) {
	// check if we already processed this addon
	conditionStatus, err := k8sutil.GetConditionStatus(ctx, kcli, in.Name, conditionName(addon))
	if err != nil {
		return errors.Wrap(err, "get condition status")
	}
	if conditionStatus == metav1.ConditionTrue {
		fmt.Printf("%s is ready\n", addon.Name())
		return nil
	}

	fmt.Printf("Upgrading %s\n", addon.Name())

	// mark as processing
	if err := k8sutil.SetConditionStatus(ctx, kcli, in, metav1.Condition{
		Type:   conditionName(addon),
		Status: metav1.ConditionFalse,
		Reason: "Upgrading",
	}); err != nil {
		return errors.Wrap(err, "set condition status")
	}

	// TODO (@salah): handle panics

	defer func() {
		if finalErr == nil {
			// mark as processed successfully
			if err := k8sutil.SetConditionStatus(ctx, kcli, in, metav1.Condition{
				Type:   conditionName(addon),
				Status: metav1.ConditionTrue,
				Reason: "Upgraded",
			}); err != nil {
				fmt.Printf("failed to set condition status: %v", err)
			}
		} else {
			// mark as failed
			if err := k8sutil.SetConditionStatus(ctx, kcli, in, metav1.Condition{
				Type:    conditionName(addon),
				Status:  metav1.ConditionFalse,
				Reason:  "UpgradeFailed",
				Message: cleanErrorMessage(finalErr),
			}); err != nil {
				fmt.Printf("failed to set condition status: %v", err)
			}
		}
	}()

	// TODO (@salah): add support for end user overrides
	overrides := addOnOverrides(addon, in.Spec.Config, nil)

	if err := addon.Upgrade(ctx, kcli, hcli, overrides); err != nil {
		return errors.Wrap(err, addon.Name())
	}

	fmt.Printf("%s is ready\n", addon.Name())
	return nil
}

func conditionName(addon types.AddOn) string {
	return fmt.Sprintf("%s-%s", addon.Namespace(), addon.ReleaseName())
}
