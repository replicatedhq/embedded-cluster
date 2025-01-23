package addons2

import (
	"context"

	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

type InstallOptions struct {
	AdminConsolePwd         string
	License                 *kotsv1beta1.License
	LicenseFile             string
	AirgapBundle            string
	Proxy                   *ecv1beta1.ProxySpec
	PrivateCAs              []string
	ConfigValuesFile        string
	ServiceCIDR             string
	DisasterRecoveryEnabled bool
}

func Install(ctx context.Context, opts InstallOptions) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return errors.Wrap(err, "create kube client")
	}

	for _, addon := range getAddOns(opts) {
		loading := spinner.Start()
		loading.Infof("Installing %s", addon.Name())

		if err := addon.Install(ctx, kcli, loading); err != nil {
			loading.CloseWithError()
			return errors.Wrap(err, "install addon")
		}

		loading.Closef("%s is ready!", addon.Name())
	}

	return nil
}

func getAddOns(opts InstallOptions) []types.AddOn {
	addOns := []types.AddOn{
		&openebs.OpenEBS{
			IsAirgap: opts.AirgapBundle != "",
		},
	}

	if opts.AirgapBundle != "" {
		addOns = append(addOns, &registry.Registry{
			ServiceCIDR: opts.ServiceCIDR,
		})
	}

	if opts.DisasterRecoveryEnabled {
		addOns = append(addOns, &velero.Velero{
			Proxy:    opts.Proxy,
			IsAirgap: opts.AirgapBundle != "",
		})
	}

	addOns = append(addOns, &adminconsole.AdminConsole{
		Password:         opts.AdminConsolePwd,
		License:          opts.License,
		LicenseFile:      opts.LicenseFile,
		AirgapBundle:     opts.AirgapBundle,
		Proxy:            opts.Proxy,
		PrivateCAs:       opts.PrivateCAs,
		ConfigValuesFile: opts.ConfigValuesFile,
	})

	return addOns
}
