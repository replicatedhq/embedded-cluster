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
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

type InstallOptions struct {
	AdminConsolePwd         string
	License                 *kotsv1beta1.License
	IsAirgap                bool
	Proxy                   *ecv1beta1.ProxySpec
	HostCABundlePath        string
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

func Install(ctx context.Context, logf types.LogFunc, hcli helm.Client, opts InstallOptions) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return errors.Wrap(err, "create kube client")
	}

	mcli, err := kubeutils.MetadataClient()
	if err != nil {
		return errors.Wrap(err, "create metadata client")
	}

	addons := getAddOnsForInstall(opts)
	if opts.IsRestore {
		addons = getAddOnsForRestore(opts)
	}

	for _, addon := range addons {
		loading := spinner.Start()
		loading.Infof("Installing %s", addon.Name())

		overrides := addOnOverrides(addon, opts.EmbeddedConfigSpec, opts.EndUserConfigSpec)

		if err := addon.Install(ctx, logf, kcli, mcli, hcli, overrides, loading); err != nil {
			loading.ErrorClosef("Failed to install %s", addon.Name())
			return errors.Wrapf(err, "install %s", addon.Name())
		}

		loading.Closef("%s is ready", addon.Name())
	}

	return nil
}

func getAddOnsForInstall(opts InstallOptions) []types.AddOn {
	domains := runtimeconfig.GetDomains(opts.EmbeddedConfigSpec)

	addOns := []types.AddOn{
		&openebs.OpenEBS{
			ProxyRegistryDomain: domains.ProxyRegistryDomain,
		},
		&embeddedclusteroperator.EmbeddedClusterOperator{
			ProxyRegistryDomain: domains.ProxyRegistryDomain,
			IsAirgap:            opts.IsAirgap,
			Proxy:               opts.Proxy,
			HostCABundlePath:    opts.HostCABundlePath,
		},
	}

	if opts.IsAirgap {
		addOns = append(addOns, &registry.Registry{
			ProxyRegistryDomain: domains.ProxyRegistryDomain,
			ServiceCIDR:         opts.ServiceCIDR,
		})
	}

	if opts.DisasterRecoveryEnabled {
		addOns = append(addOns, &velero.Velero{
			ProxyRegistryDomain:      domains.ProxyRegistryDomain,
			Proxy:                    opts.Proxy,
			HostCABundlePath:         opts.HostCABundlePath,
			EmbeddedClusterK0sSubDir: runtimeconfig.EmbeddedClusterK0sSubDir(),
		})
	}

	adminConsoleAddOn := &adminconsole.AdminConsole{
		IsAirgap:                 opts.IsAirgap,
		Proxy:                    opts.Proxy,
		ServiceCIDR:              opts.ServiceCIDR,
		Password:                 opts.AdminConsolePwd,
		TLSCertBytes:             opts.TLSCertBytes,
		TLSKeyBytes:              opts.TLSKeyBytes,
		Hostname:                 opts.Hostname,
		KotsInstaller:            opts.KotsInstaller,
		IsMultiNodeEnabled:       opts.IsMultiNodeEnabled,
		ReplicatedAppDomain:      domains.ReplicatedAppDomain,
		ProxyRegistryDomain:      domains.ProxyRegistryDomain,
		ReplicatedRegistryDomain: domains.ReplicatedRegistryDomain,
		HostCABundlePath:         opts.HostCABundlePath,
	}
	addOns = append(addOns, adminConsoleAddOn)

	return addOns
}

func getAddOnsForRestore(opts InstallOptions) []types.AddOn {
	domains := runtimeconfig.GetDomains(opts.EmbeddedConfigSpec)

	addOns := []types.AddOn{
		&openebs.OpenEBS{
			ProxyRegistryDomain: domains.ProxyRegistryDomain,
		},
		&velero.Velero{
			Proxy:                    opts.Proxy,
			ProxyRegistryDomain:      domains.ProxyRegistryDomain,
			HostCABundlePath:         opts.HostCABundlePath,
			EmbeddedClusterK0sSubDir: runtimeconfig.EmbeddedClusterK0sSubDir(),
		},
	}
	return addOns
}
