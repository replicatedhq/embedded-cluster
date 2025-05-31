package addons

import (
	"context"

	"github.com/pkg/errors"
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
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Install(ctx context.Context, logf types.LogFunc, hcli helm.Client, opts types.InstallOptions) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return errors.Wrap(err, "create kube client")
	}

	mcli, err := kubeutils.MetadataClient()
	if err != nil {
		return errors.Wrap(err, "create metadata client")
	}

	addons := getAddOnsForInstall(logf, kcli, mcli, hcli)
	if opts.IsRestore {
		addons = getAddOnsForRestore(logf, kcli, mcli, hcli)
	}

	for _, addon := range addons {
		loading := spinner.Start()
		loading.Infof("Installing %s", addon.Name())

		overrides := addOnOverrides(addon, opts.EmbeddedConfigSpec, opts.EndUserConfigSpec)

		if err := addon.Install(ctx, loading, opts, overrides); err != nil {
			loading.ErrorClosef("Failed to install %s", addon.Name())
			return errors.Wrapf(err, "install %s", addon.Name())
		}

		loading.Closef("%s is ready", addon.Name())
	}

	return nil
}

func getAddOnsForInstall(logf types.LogFunc, kcli client.Client, mcli metadata.Interface, hcli helm.Client) []types.AddOn {
	addOns := []types.AddOn{
		openebs.New(
			openebs.WithLogFunc(logf),
			openebs.WithClients(kcli, mcli, hcli),
		),
		embeddedclusteroperator.New(
			embeddedclusteroperator.WithLogFunc(logf),
			embeddedclusteroperator.WithClients(kcli, mcli, hcli),
		),
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

	addOns = append(addOns, adminconsole.New(
		adminconsole.WithLogFunc(logf),
		adminconsole.WithClients(kcli, mcli, hcli),
	))

	return addOns
}

func getAddOnsForRestore(logf types.LogFunc, kcli client.Client, mcli metadata.Interface, hcli helm.Client) []types.AddOn {
	domains := runtimeconfig.GetDomains(opts.EmbeddedConfigSpec)

	addOns := []types.AddOn{
		openebs.New(
			openebs.WithLogFunc(logf),
			openebs.WithClients(kcli, mcli, hcli),
		),
		&velero.Velero{
			Proxy:                    opts.Proxy,
			ProxyRegistryDomain:      domains.ProxyRegistryDomain,
			HostCABundlePath:         opts.HostCABundlePath,
			EmbeddedClusterK0sSubDir: runtimeconfig.EmbeddedClusterK0sSubDir(),
		},
	}
	return addOns
}
