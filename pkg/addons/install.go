package addons

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Install(
	ctx context.Context, logf types.LogFunc,
	kcli client.Client, mcli metadata.Interface, hcli helm.Client,
	inSpec ecv1beta1.InstallationSpec, installOpts types.InstallOptions,
) error {
	addons := getAddOnsForInstall(logf, inSpec)
	if installOpts.IsRestore {
		addons = getAddOnsForRestore(logf)
	}

	clients := types.Clients{
		K8sClient:      kcli,
		MetadataClient: mcli,
		HelmClient:     hcli,
	}

	for _, addon := range addons {
		loading := spinner.Start()
		loading.Infof("Installing %s", addon.Name())

		overrides := addOnOverrides(addon, inSpec.Config, installOpts.EndUserConfigSpec)

		if err := addon.Install(ctx, clients, loading, inSpec, overrides, installOpts); err != nil {
			loading.ErrorClosef("Failed to install %s", addon.Name())
			return errors.Wrapf(err, "install %s", addon.Name())
		}

		loading.Closef("%s is ready", addon.Name())
	}

	return nil
}

func getAddOnsForInstall(logf types.LogFunc, inSpec ecv1beta1.InstallationSpec) []types.AddOn {
	addOns := []types.AddOn{
		openebs.New(
			openebs.WithLogFunc(logf),
		),
		embeddedclusteroperator.New(
			embeddedclusteroperator.WithLogFunc(logf),
		),
	}

	if inSpec.AirGap {
		addOns = append(addOns, registry.New(
			registry.WithLogFunc(logf),
		))
	}

	if inSpec.LicenseInfo != nil && inSpec.LicenseInfo.IsDisasterRecoverySupported {
		addOns = append(addOns, velero.New(
			velero.WithLogFunc(logf),
		))
	}

	addOns = append(addOns, adminconsole.New(
		adminconsole.WithLogFunc(logf),
	))

	return addOns
}

func getAddOnsForRestore(logf types.LogFunc) []types.AddOn {
	addOns := []types.AddOn{
		openebs.New(
			openebs.WithLogFunc(logf),
		),
		velero.New(
			velero.WithLogFunc(logf),
		),
	}
	return addOns
}
