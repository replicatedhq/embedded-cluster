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
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
)

type InstallOptions struct {
	AdminConsolePwd    string
	AdminConsolePort   int
	License            licensewrapper.LicenseWrapper
	IsAirgap           bool
	TLSCertBytes       []byte
	TLSKeyBytes        []byte
	Hostname           string
	IsMultiNodeEnabled bool
	EmbeddedConfigSpec *ecv1beta1.ConfigSpec
	EndUserConfigSpec  *ecv1beta1.ConfigSpec
	KotsInstaller      adminconsole.KotsInstaller
	ProxySpec          *ecv1beta1.ProxySpec

	// Linux only options
	ClusterID               string
	DisasterRecoveryEnabled bool
	HostCABundlePath        string
	KotsadmNamespace        string
	DataDir                 string
	K0sDataDir              string
	OpenEBSDataDir          string
	ServiceCIDR             string
}

type KubernetesInstallOptions struct {
	AdminConsolePwd    string
	AdminConsolePort   int
	License            licensewrapper.LicenseWrapper
	IsAirgap           bool
	TLSCertBytes       []byte
	TLSKeyBytes        []byte
	Hostname           string
	IsMultiNodeEnabled bool
	EmbeddedConfigSpec *ecv1beta1.ConfigSpec
	EndUserConfigSpec  *ecv1beta1.ConfigSpec
	KotsInstaller      adminconsole.KotsInstaller
	KotsadmNamespace   string
	ProxySpec          *ecv1beta1.ProxySpec
}

func (a *AddOns) Install(ctx context.Context, opts InstallOptions) error {
	addons := GetAddOnsForInstall(opts)

	for _, addon := range addons {
		a.sendProgress(addon.Name(), apitypes.StateRunning, "Installing")

		overrides := a.addOnOverrides(addon, opts.EmbeddedConfigSpec, opts.EndUserConfigSpec)

		if err := addon.Install(ctx, a.logf, a.kcli, a.mcli, a.hcli, a.domains, overrides); err != nil {
			a.sendProgress(addon.Name(), apitypes.StateFailed, err.Error())
			return errors.Wrapf(err, "install %s", addon.Name())
		}

		a.sendProgress(addon.Name(), apitypes.StateSucceeded, "Installed")
	}

	return nil
}

func (a *AddOns) InstallKubernetes(ctx context.Context, opts KubernetesInstallOptions) error {
	addons := GetAddOnsForKubernetesInstall(opts)

	for _, addon := range addons {
		a.sendProgress(addon.Name(), apitypes.StateRunning, "Installing")

		overrides := a.addOnOverrides(addon, opts.EmbeddedConfigSpec, opts.EndUserConfigSpec)

		if err := addon.Install(ctx, a.logf, a.kcli, a.mcli, a.hcli, a.domains, overrides); err != nil {
			a.sendProgress(addon.Name(), apitypes.StateFailed, err.Error())
			return errors.Wrapf(err, "install %s", addon.Name())
		}

		a.sendProgress(addon.Name(), apitypes.StateSucceeded, "Installed")
	}

	return nil
}

type RestoreOptions struct {
	EmbeddedConfigSpec *ecv1beta1.ConfigSpec
	EndUserConfigSpec  *ecv1beta1.ConfigSpec
	ProxySpec          *ecv1beta1.ProxySpec
	HostCABundlePath   string
	DataDir            string
	OpenEBSDataDir     string
	K0sDataDir         string
}

func (a *AddOns) Restore(ctx context.Context, opts RestoreOptions) error {
	addons := GetAddOnsForRestore(opts)

	for _, addon := range addons {
		a.sendProgress(addon.Name(), apitypes.StateRunning, "Installing")

		overrides := a.addOnOverrides(addon, opts.EmbeddedConfigSpec, opts.EndUserConfigSpec)

		if err := addon.Install(ctx, a.logf, a.kcli, a.mcli, a.hcli, a.domains, overrides); err != nil {
			a.sendProgress(addon.Name(), apitypes.StateFailed, err.Error())
			return errors.Wrapf(err, "install %s", addon.Name())
		}

		a.sendProgress(addon.Name(), apitypes.StateSucceeded, "Installed")
	}

	return nil
}

// Convenience function for getting the names of the addons to install without having to provide the full install options
func GetAddOnsNamesForInstall(isAirgap bool, disasterRecoveryEnabled bool) []string {
	addOns := []types.AddOn{
		&openebs.OpenEBS{},
		&embeddedclusteroperator.EmbeddedClusterOperator{},
	}

	if isAirgap {
		addOns = append(addOns, &registry.Registry{})
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

func GetAddOnsForInstall(opts InstallOptions) []types.AddOn {
	addOns := []types.AddOn{
		&openebs.OpenEBS{
			OpenEBSDataDir: opts.OpenEBSDataDir,
		},
		&embeddedclusteroperator.EmbeddedClusterOperator{
			ClusterID:        opts.ClusterID,
			IsAirgap:         opts.IsAirgap,
			Proxy:            opts.ProxySpec,
			HostCABundlePath: opts.HostCABundlePath,
			KotsadmNamespace: opts.KotsadmNamespace,
		},
	}

	if opts.IsAirgap {
		addOns = append(addOns, &registry.Registry{
			ServiceCIDR: opts.ServiceCIDR,
			IsHA:        false,
		})
	}

	if opts.DisasterRecoveryEnabled {
		addOns = append(addOns, &velero.Velero{
			Proxy:            opts.ProxySpec,
			HostCABundlePath: opts.HostCABundlePath,
			K0sDataDir:       opts.K0sDataDir,
		})
	}

	adminConsoleAddOn := &adminconsole.AdminConsole{
		ClusterID:          opts.ClusterID,
		IsAirgap:           opts.IsAirgap,
		IsHA:               false,
		Proxy:              opts.ProxySpec,
		ServiceCIDR:        opts.ServiceCIDR,
		IsMultiNodeEnabled: opts.IsMultiNodeEnabled,
		HostCABundlePath:   opts.HostCABundlePath,
		DataDir:            opts.DataDir,
		K0sDataDir:         opts.K0sDataDir,
		AdminConsolePort:   opts.AdminConsolePort,

		Password:         opts.AdminConsolePwd,
		TLSCertBytes:     opts.TLSCertBytes,
		TLSKeyBytes:      opts.TLSKeyBytes,
		Hostname:         opts.Hostname,
		KotsInstaller:    opts.KotsInstaller,
		KotsadmNamespace: opts.KotsadmNamespace,
	}
	addOns = append(addOns, adminConsoleAddOn)

	return addOns
}

func GetAddOnsForRestore(opts RestoreOptions) []types.AddOn {
	addOns := []types.AddOn{
		&openebs.OpenEBS{
			OpenEBSDataDir: opts.OpenEBSDataDir,
		},
		&velero.Velero{
			Proxy:            opts.ProxySpec,
			HostCABundlePath: opts.HostCABundlePath,
			K0sDataDir:       opts.K0sDataDir,
		},
	}
	return addOns
}

// Convenience function for getting the names of the addons to install without having to provide the full install options
func GetAddOnsNamesForKubernetesInstall() []string {
	addOns := []types.AddOn{
		&adminconsole.AdminConsole{},
	}

	names := []string{}
	for _, addOn := range addOns {
		names = append(names, addOn.Name())
	}
	return names
}

func GetAddOnsForKubernetesInstall(opts KubernetesInstallOptions) []types.AddOn {
	addOns := []types.AddOn{}

	adminConsoleAddOn := &adminconsole.AdminConsole{
		IsAirgap:           opts.IsAirgap,
		IsHA:               false,
		IsMultiNodeEnabled: opts.IsMultiNodeEnabled,
		Proxy:              opts.ProxySpec,
		AdminConsolePort:   opts.AdminConsolePort,

		Password:         opts.AdminConsolePwd,
		TLSCertBytes:     opts.TLSCertBytes,
		TLSKeyBytes:      opts.TLSKeyBytes,
		Hostname:         opts.Hostname,
		KotsInstaller:    opts.KotsInstaller,
		KotsadmNamespace: opts.KotsadmNamespace,
	}
	addOns = append(addOns, adminConsoleAddOn)

	return addOns
}
