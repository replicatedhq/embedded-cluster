package addons

import (
	"context"

	"github.com/pkg/errors"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/adminconsole"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/embeddedclusteroperator"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/openebs"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/velero"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

type InstallOptions struct {
	AdminConsolePwd         string
	License                 *kotsv1beta1.License
	IsAirgap                bool
	TLSCertBytes            []byte
	TLSKeyBytes             []byte
	Hostname                string
	DisasterRecoveryEnabled bool
	IsMultiNodeEnabled      bool
	EmbeddedConfigSpec      *ecv1beta1.ConfigSpec
	EndUserConfigSpec       *ecv1beta1.ConfigSpec
	KotsInstaller           adminconsole.KotsInstaller
	IsRestore               bool
}

func (a *AddOns) Install(ctx context.Context, opts InstallOptions) error {
	addons := GetAddOnsForInstall(a.rc, opts)
	if opts.IsRestore {
		addons = GetAddOnsForRestore(a.rc, opts)
	}

	domains := runtimeconfig.GetDomains(opts.EmbeddedConfigSpec)

	for _, addon := range addons {
		a.sendProgress(addon.Name(), apitypes.StateRunning, "Installing")

		overrides := a.addOnOverrides(addon, opts.EmbeddedConfigSpec, opts.EndUserConfigSpec)

		if err := addon.Install(ctx, a.logf, a.kcli, a.mcli, a.hcli, a.rc, domains, overrides); err != nil {
			a.sendProgress(addon.Name(), apitypes.StateFailed, err.Error())
			return errors.Wrapf(err, "install %s", addon.Name())
		}

		a.sendProgress(addon.Name(), apitypes.StateSucceeded, "Installed")
	}

	return nil
}

func GetAddOnsForInstall(rc runtimeconfig.RuntimeConfig, opts InstallOptions) []types.AddOn {
	addOns := []types.AddOn{
		&openebs.OpenEBS{},
		&embeddedclusteroperator.EmbeddedClusterOperator{
			IsAirgap: opts.IsAirgap,
			Proxy:    rc.ProxySpec(),
		},
	}

	if opts.IsAirgap {
		addOns = append(addOns, &registry.Registry{
			ServiceCIDR: rc.ServiceCIDR(),
		})
	}

	if opts.DisasterRecoveryEnabled {
		addOns = append(addOns, &velero.Velero{
			Proxy: rc.ProxySpec(),
		})
	}

	adminConsoleAddOn := &adminconsole.AdminConsole{
		IsAirgap:           opts.IsAirgap,
		Proxy:              rc.ProxySpec(),
		ServiceCIDR:        rc.ServiceCIDR(),
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

func GetAddOnsForRestore(rc runtimeconfig.RuntimeConfig, opts InstallOptions) []types.AddOn {
	addOns := []types.AddOn{
		&openebs.OpenEBS{},
		&velero.Velero{
			Proxy: rc.ProxySpec(),
		},
	}
	return addOns
}
