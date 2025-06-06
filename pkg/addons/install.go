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
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func InstallOptionsFromInstallationSpec(inSpec ecv1beta1.InstallationSpec) types.InstallOptions {
	serviceCIDR := ""
	if inSpec.Network != nil {
		serviceCIDR = inSpec.Network.ServiceCIDR
	}

	// The following fields are from installation flags:
	// - AdminConsolePassword
	// - TLSCertBytes
	// - TLSKeyBytes
	// - Hostname
	// - EndUserConfigSpec
	// - KotsInstaller

	// TODO (@salah): add support for end user overrides from the installation spec

	return types.InstallOptions{
		ClusterID:                 inSpec.ClusterID,
		IsAirgap:                  inSpec.AirGap,
		IsHA:                      inSpec.HighAvailability,
		Proxy:                     inSpec.Proxy,
		ServiceCIDR:               serviceCIDR,
		IsDisasterRecoveryEnabled: inSpec.LicenseInfo != nil && inSpec.LicenseInfo.IsDisasterRecoverySupported,
		IsMultiNodeEnabled:        inSpec.LicenseInfo != nil && inSpec.LicenseInfo.IsMultiNodeEnabled,
		EmbeddedConfigSpec:        inSpec.Config,
		Domains:                   runtimeconfig.GetDomains(inSpec.Config),
	}
}

func Install(ctx context.Context, logf types.LogFunc, hcli helm.Client, rc runtimeconfig.RuntimeConfig, opts types.InstallOptions) error {
	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return errors.Wrap(err, "create kube client")
	}

	mcli, err := kubeutils.MetadataClient()
	if err != nil {
		return errors.Wrap(err, "create metadata client")
	}

	addons := getAddOnsForInstall(logf, kcli, mcli, hcli, rc, opts)
	if opts.IsRestore {
		addons = getAddOnsForRestore(logf, kcli, mcli, hcli, rc)
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

func getAddOnsForInstall(logf types.LogFunc, kcli client.Client, mcli metadata.Interface, hcli helm.Client, rc runtimeconfig.RuntimeConfig, opts types.InstallOptions) []types.AddOn {
	addOns := []types.AddOn{
		openebs.New(
			openebs.WithLogFunc(logf),
			openebs.WithClients(kcli, mcli, hcli),
			openebs.WithRuntimeConfig(rc),
		),
		embeddedclusteroperator.New(
			embeddedclusteroperator.WithLogFunc(logf),
			embeddedclusteroperator.WithClients(kcli, mcli, hcli),
			embeddedclusteroperator.WithRuntimeConfig(rc),
		),
	}

	if opts.IsAirgap {
		addOns = append(addOns, registry.New(
			registry.WithLogFunc(logf),
			registry.WithClients(kcli, mcli, hcli),
			registry.WithRuntimeConfig(rc),
		))
	}

	if opts.IsDisasterRecoveryEnabled {
		addOns = append(addOns, velero.New(
			velero.WithLogFunc(logf),
			velero.WithClients(kcli, mcli, hcli),
			velero.WithRuntimeConfig(rc),
		))
	}

	addOns = append(addOns, adminconsole.New(
		adminconsole.WithLogFunc(logf),
		adminconsole.WithClients(kcli, mcli, hcli),
		adminconsole.WithRuntimeConfig(rc),
	))

	return addOns
}

func getAddOnsForRestore(logf types.LogFunc, kcli client.Client, mcli metadata.Interface, hcli helm.Client, rc runtimeconfig.RuntimeConfig) []types.AddOn {
	addOns := []types.AddOn{
		openebs.New(
			openebs.WithLogFunc(logf),
			openebs.WithClients(kcli, mcli, hcli),
			openebs.WithRuntimeConfig(rc),
		),
		velero.New(
			velero.WithLogFunc(logf),
			velero.WithClients(kcli, mcli, hcli),
			velero.WithRuntimeConfig(rc),
		),
	}
	return addOns
}
