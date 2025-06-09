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
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

type InstallOptions struct {
	AdminConsolePwd         string
	License                 *kotsv1beta1.License
	IsAirgap                bool
	Proxy                   *ecv1beta1.ProxySpec
	TLSCertBytes            []byte
	TLSKeyBytes             []byte
	Hostname                string
	ServiceCIDR             string
	DisasterRecoveryEnabled bool
	IsMultiNodeEnabled      bool
	EmbeddedConfigSpec      *ecv1beta1.ConfigSpec
	EndUserConfigSpec       *ecv1beta1.ConfigSpec
	KotsInstaller           adminconsole.KotsInstaller
	IsRestore               bool
}

func (a *AddOns) Install(ctx context.Context, opts InstallOptions) error {
	addons := a.getAddOnsForInstall(opts)
	if opts.IsRestore {
		addons = a.getAddOnsForRestore(opts)
	}

	domains := runtimeconfig.GetDomains(opts.EmbeddedConfigSpec)

	for _, addon := range addons {
		loading := spinner.Start()
		loading.Infof("Installing %s", addon.Name())

		overrides := a.addOnOverrides(addon, opts.EmbeddedConfigSpec, opts.EndUserConfigSpec)

		if err := addon.Install(ctx, a.logf, a.kcli, a.mcli, a.hcli, a.rc, domains, overrides, loading); err != nil {
			loading.ErrorClosef("Failed to install %s", addon.Name())
			return errors.Wrapf(err, "install %s", addon.Name())
		}

		loading.Closef("%s is ready", addon.Name())
	}

	return nil
}

func (a *AddOns) getAddOnsForInstall(opts InstallOptions) []types.AddOn {
	addOns := []types.AddOn{
		&openebs.OpenEBS{},
		&embeddedclusteroperator.EmbeddedClusterOperator{
			IsAirgap: opts.IsAirgap,
			Proxy:    opts.Proxy,
		},
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

	adminConsoleAddOn := &adminconsole.AdminConsole{
		IsAirgap:           opts.IsAirgap,
		Proxy:              opts.Proxy,
		ServiceCIDR:        opts.ServiceCIDR,
		Password:           opts.AdminConsolePwd,
		TLSCertBytes:       opts.TLSCertBytes,
		TLSKeyBytes:        opts.TLSKeyBytes,
		Hostname:           opts.Hostname,
		KotsInstaller:      opts.KotsInstaller,
		IsMultiNodeEnabled: opts.IsMultiNodeEnabled,
	}
	addOns = append(addOns, adminConsoleAddOn)

	return addOns
}

func (a *AddOns) getAddOnsForRestore(opts InstallOptions) []types.AddOn {
	addOns := []types.AddOn{
		&openebs.OpenEBS{},
		&velero.Velero{
			Proxy: opts.Proxy,
		},
	}
	return addOns
}
