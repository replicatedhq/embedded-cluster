package addons2

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

type InstallOptions struct {
	AdminConsolePwd         string
	License                 *kotsv1beta1.License
	IsAirgap                bool
	Proxy                   *ecv1beta1.ProxySpec
	PrivateCAs              []string
	ServiceCIDR             string
	DisasterRecoveryEnabled bool
	ReleaseConfig           *ecv1beta1.Config
	EndUserConfig           *ecv1beta1.Config
	KotsInstaller           adminconsole.KotsInstaller
}

func Install(ctx context.Context, opts InstallOptions) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return errors.Wrap(err, "create kube client")
	}

	airgapChartsPath := ""
	if opts.IsAirgap {
		airgapChartsPath = runtimeconfig.EmbeddedClusterChartsSubDir()
	}

	hcli, err := helm.NewHelm(helm.HelmOptions{
		KubeConfig: runtimeconfig.PathToKubeConfig(),
		K0sVersion: versions.K0sVersion,
		AirgapPath: airgapChartsPath,
	})
	if err != nil {
		return errors.Wrap(err, "create helm client")
	}

	for _, addon := range getAddOnsForInstall(opts) {
		loading := spinner.Start()
		loading.Infof("Installing %s", addon.Name())

		overrides := []string{}
		if opts.ReleaseConfig != nil {
			overrides = append(overrides, opts.ReleaseConfig.Spec.OverrideForBuiltIn(addon.ReleaseName()))
		}
		if opts.EndUserConfig != nil {
			overrides = append(overrides, opts.EndUserConfig.Spec.OverrideForBuiltIn(addon.ReleaseName()))
		}

		if err := addon.Install(ctx, kcli, hcli, overrides, loading); err != nil {
			loading.CloseWithError()
			return errors.Wrap(err, "install addon")
		}

		loading.Closef("%s is ready!", addon.Name())
	}

	return nil
}

func getAddOnsForInstall(opts InstallOptions) []types.AddOn {
	addOns := []types.AddOn{
		&openebs.OpenEBS{},
		&embeddedclusteroperator.EmbeddedClusterOperator{},
	}

	if opts.IsAirgap {
		addOns = append(addOns, &registry.Registry{
			ServiceCIDR: opts.ServiceCIDR,
		})
	}

	if opts.DisasterRecoveryEnabled {
		addOns = append(addOns, &velero.Velero{
			Proxy: opts.Proxy,
		})
	}

	addOns = append(addOns, &adminconsole.AdminConsole{
		Password:      opts.AdminConsolePwd,
		IsAirgap:      opts.IsAirgap,
		Proxy:         opts.Proxy,
		PrivateCAs:    opts.PrivateCAs,
		KotsInstaller: opts.KotsInstaller,
	})

	return addOns
}
