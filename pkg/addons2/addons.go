package addons2

import (
	"context"

	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/pkg/errors"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons2/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

type InstallOptions struct {
	ClusterConfig           *k0sconfig.ClusterConfig
	AdminConsolePwd         string
	License                 *kotsv1beta1.License
	LicenseFile             string
	AirgapBundle            string
	Proxy                   *ecv1beta1.ProxySpec
	PrivateCAs              []string
	ConfigValuesFile        string
	NetworkInterface        string
	DisasterRecoveryEnabled bool
}

// this is a temp function that's much more specific than we actually need it to be
// this is going to get us to working installs, and we refactor.
// this is not configurable at all, it's not the way it needs to be in the product
func InstallAddons(ctx context.Context, opts InstallOptions) error {
	addOns := []types.AddOn{
		&openebs.OpenEBS{},
		&adminconsole.AdminConsole{
			Password:         opts.AdminConsolePwd,
			License:          opts.License,
			LicenseFile:      opts.LicenseFile,
			AirgapBundle:     opts.AirgapBundle,
			Proxy:            opts.Proxy,
			PrivateCAs:       opts.PrivateCAs,
			ConfigValuesFile: opts.ConfigValuesFile,
			NetworkInterface: opts.NetworkInterface,
		},
	}

	if opts.DisasterRecoveryEnabled {
		addOns = append(addOns, &velero.Velero{
			Proxy: opts.Proxy,
		})
	}

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return errors.Wrap(err, "create kube client")
	}

	loading := spinner.Start()
	defer loading.Close()

	for _, addon := range addOns {
		loading.Infof("Installing %s addon", addon.Name())

		if err := addon.Prepare(); err != nil {
			return errors.Wrap(err, "prepare addon")
		}
		if err := addon.Install(ctx, kcli, loading); err != nil {
			return errors.Wrap(err, "install addon")
		}
	}

	return nil
}
