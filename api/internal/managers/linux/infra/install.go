package infra

import (
	"context"
	"fmt"
	"runtime/debug"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ecmetadata "github.com/replicatedhq/embedded-cluster/pkg-new/metadata"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	addontypes "github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/extensions"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/support"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/metadata"
	nodeutil "k8s.io/component-helpers/node/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const K0sComponentName = "Runtime"

func AlreadyInstalledError() error {
	//nolint:staticcheck // ST1005 TODO: use a constant here and print a better error message
	return fmt.Errorf(
		"\nAn installation is detected on this machine.\nTo install, you must first remove the existing installation.\nYou can do this by running the following command:\n\n  sudo ./%s reset\n",
		runtimeconfig.AppSlug(),
	)
}

func (m *infraManager) Install(ctx context.Context, rc runtimeconfig.RuntimeConfig) error {
	installed, err := m.k0scli.IsInstalled()
	if err != nil {
		return fmt.Errorf("check if k0s is installed: %w", err)
	}
	if installed {
		return AlreadyInstalledError()
	}

	if err := m.install(ctx, rc); err != nil {
		return err
	}

	return nil
}

func (m *infraManager) initInstallComponentsList(license *licensewrapper.LicenseWrapper) error {
	if license.IsEmpty() {
		return fmt.Errorf("license is required for component initialization")
	}

	components := []types.InfraComponent{{Name: K0sComponentName}}

	addOnsNames := addons.GetAddOnsNamesForInstall(m.airgapBundle != "", license.IsDisasterRecoverySupported())
	for _, addOnName := range addOnsNames {
		components = append(components, types.InfraComponent{Name: addOnName})
	}

	components = append(components, types.InfraComponent{Name: "Additional Components"})

	for _, component := range components {
		if err := m.infraStore.RegisterComponent(component.Name); err != nil {
			return fmt.Errorf("register component: %w", err)
		}
	}
	return nil
}

func (m *infraManager) install(ctx context.Context, rc runtimeconfig.RuntimeConfig) error {
	license, err := helpers.ParseLicenseFromBytes(m.license)
	if err != nil {
		return fmt.Errorf("parse license: %w", err)
	}

	if err := m.initInstallComponentsList(license); err != nil {
		return fmt.Errorf("init components: %w", err)
	}

	_, err = m.installK0s(ctx, rc)
	if err != nil {
		return fmt.Errorf("install k0s: %w", err)
	}

	in, err := m.recordInstallation(ctx, m.kcli, license, rc)
	if err != nil {
		return fmt.Errorf("record installation: %w", err)
	}

	if err := m.installAddOns(ctx, m.kcli, m.mcli, m.hcli, license, rc); err != nil {
		return fmt.Errorf("install addons: %w", err)
	}

	if err := m.installExtensions(ctx, m.hcli); err != nil {
		return fmt.Errorf("install extensions: %w", err)
	}

	if err := kubeutils.SetInstallationState(ctx, m.kcli, in, ecv1beta1.InstallationStateInstalled, "Installed"); err != nil {
		return fmt.Errorf("update installation: %w", err)
	}

	if err = support.CreateHostSupportBundle(ctx, m.kcli); err != nil {
		m.logger.Warnf("Unable to create host support bundle: %v", err)
	}

	return nil
}

func (m *infraManager) installK0s(ctx context.Context, rc runtimeconfig.RuntimeConfig) (k0sCfg *k0sv1beta1.ClusterConfig, finalErr error) {
	componentName := K0sComponentName

	if err := m.setComponentStatus(componentName, types.StateRunning, "Installing"); err != nil {
		return nil, fmt.Errorf("set extensions status: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("install k0s recovered from panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			if err := m.setComponentStatus(componentName, types.StateFailed, finalErr.Error()); err != nil {
				m.logger.WithError(err).Error("set failed status")
			}
		} else {
			if err := m.setComponentStatus(componentName, types.StateSucceeded, ""); err != nil {
				m.logger.WithError(err).Error("set succeeded status")
			}
		}
	}()

	m.setStatusDesc(fmt.Sprintf("Installing %s", componentName))

	// Detect stable hostname early in installation
	hostname, err := nodeutil.GetHostname("")
	if err != nil {
		return nil, fmt.Errorf("unable to detect hostname: %w", err)
	}

	logFn := m.logFn("k0s")

	logFn("creating k0s configuration file")
	k0sCfg, err = m.k0scli.NewK0sConfig(rc.NetworkInterface(), m.airgapBundle != "", rc.PodCIDR(), rc.ServiceCIDR(), m.endUserConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("new k0s config: %w", err)
	}
	if err := m.k0scli.WriteK0sConfig(ctx, k0sCfg); err != nil {
		return nil, fmt.Errorf("create config file: %w", err)
	}

	logFn("creating systemd unit files")
	if err := m.hostUtils.CreateSystemdUnitFiles(ctx, m.logger, rc, hostname, false); err != nil {
		return nil, fmt.Errorf("create systemd unit files: %w", err)
	}

	logFn("installing k0s")
	if err := m.k0scli.Install(rc, hostname); err != nil {
		return nil, fmt.Errorf("install cluster: %w", err)
	}

	logFn("waiting for k0s to be ready")
	if err := m.k0scli.WaitForK0s(); err != nil {
		return nil, fmt.Errorf("wait for k0s: %w", err)
	}

	// initialize the manager's helm and kube clients
	err = m.setupClients(rc)
	if err != nil {
		return nil, fmt.Errorf("setup clients: %w", err)
	}

	m.setStatusDesc(fmt.Sprintf("Waiting for %s", componentName))

	logFn("waiting for node to be ready")
	if err := m.waitForNode(ctx, m.kcli); err != nil {
		return nil, fmt.Errorf("wait for node: %w", err)
	}

	logFn("adding registry to containerd")
	registryIP, err := registry.GetRegistryClusterIP(rc.ServiceCIDR())
	if err != nil {
		return nil, fmt.Errorf("get registry cluster IP: %w", err)
	}
	if err := m.hostUtils.AddInsecureRegistry(fmt.Sprintf("%s:5000", registryIP)); err != nil {
		return nil, fmt.Errorf("add insecure registry: %w", err)
	}

	return k0sCfg, nil
}

func (m *infraManager) recordInstallation(ctx context.Context, kcli client.Client, license *licensewrapper.LicenseWrapper, rc runtimeconfig.RuntimeConfig) (*ecv1beta1.Installation, error) {
	logFn := m.logFn("metadata")

	// get the configured custom domains
	ecDomains := utils.GetDomains(m.releaseData)

	// extract airgap uncompressed size if airgap info is provided
	var airgapUncompressedSize int64
	if m.airgapMetadata != nil && m.airgapMetadata.AirgapInfo != nil {
		airgapUncompressedSize = m.airgapMetadata.AirgapInfo.Spec.UncompressedSize
	}

	// record the installation
	logFn("recording installation")
	in, err := kubeutils.RecordInstallation(ctx, kcli, kubeutils.RecordInstallationOptions{
		ClusterID:              m.clusterID,
		IsAirgap:               m.airgapBundle != "",
		License:                license,
		ConfigSpec:             m.getECConfigSpec(),
		MetricsBaseURL:         netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain),
		RuntimeConfig:          rc.Get(),
		EndUserConfig:          m.endUserConfig,
		AirgapUncompressedSize: airgapUncompressedSize,
	})
	if err != nil {
		return nil, fmt.Errorf("record installation: %w", err)
	}

	logFn("creating version metadata configmap")
	if err := ecmetadata.CreateVersionMetadataConfigmap(ctx, kcli); err != nil {
		return nil, fmt.Errorf("create version metadata configmap: %w", err)
	}

	return in, nil
}

func (m *infraManager) installAddOns(ctx context.Context, kcli client.Client, mcli metadata.Interface, hcli helm.Client, license *licensewrapper.LicenseWrapper, rc runtimeconfig.RuntimeConfig) error {
	progressChan := make(chan addontypes.AddOnProgress)
	defer close(progressChan)

	go func() {
		for progress := range progressChan {
			// capture progress in debug logs
			m.logger.WithFields(logrus.Fields{"addon": progress.Name, "state": progress.Status.State, "description": progress.Status.Description}).Debugf("addon progress")

			// if in progress, update the overall status to reflect the current component
			if progress.Status.State == types.StateRunning {
				m.setStatusDesc(fmt.Sprintf("%s %s", progress.Status.Description, progress.Name))
			}

			// update the status for the current component
			if err := m.setComponentStatus(progress.Name, progress.Status.State, progress.Status.Description); err != nil {
				m.logger.Errorf("Failed to update addon status: %v", err)
			}
		}
	}()

	logFn := m.logFn("addons")

	addOns := addons.New(
		addons.WithLogFunc(logFn),
		addons.WithKubernetesClient(kcli),
		addons.WithMetadataClient(mcli),
		addons.WithHelmClient(hcli),
		addons.WithDomains(utils.GetDomains(m.releaseData)),
		addons.WithProgressChannel(progressChan),
	)

	opts, err := m.getAddonInstallOpts(ctx, license, rc)
	if err != nil {
		return fmt.Errorf("get addon install options: %w", err)
	}

	logFn("installing addons")
	if err := addOns.Install(ctx, opts); err != nil {
		return err
	}

	return nil
}

func (m *infraManager) getAddonInstallOpts(ctx context.Context, license *licensewrapper.LicenseWrapper, rc runtimeconfig.RuntimeConfig) (addons.InstallOptions, error) {
	if license.IsEmpty() {
		return addons.InstallOptions{}, fmt.Errorf("license is required for addon installation")
	}

	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, m.kcli)
	if err != nil {
		return addons.InstallOptions{}, fmt.Errorf("get kotsadm namespace: %w", err)
	}
	opts := addons.InstallOptions{
		ClusterID:               m.clusterID,
		AdminConsolePwd:         m.password,
		AdminConsolePort:        rc.AdminConsolePort(),
		License:                 license,
		IsAirgap:                m.airgapBundle != "",
		TLSCertBytes:            m.tlsConfig.CertBytes,
		TLSKeyBytes:             m.tlsConfig.KeyBytes,
		Hostname:                m.tlsConfig.Hostname,
		DisasterRecoveryEnabled: license.IsDisasterRecoverySupported(),
		IsMultiNodeEnabled:      license.IsEmbeddedClusterMultiNodeEnabled(),
		EmbeddedConfigSpec:      m.getECConfigSpec(),
		EndUserConfigSpec:       m.getEndUserConfigSpec(),
		ProxySpec:               rc.ProxySpec(),
		HostCABundlePath:        rc.HostCABundlePath(),
		KotsadmNamespace:        kotsadmNamespace,
		DataDir:                 rc.EmbeddedClusterHomeDirectory(),
		K0sDataDir:              rc.EmbeddedClusterK0sSubDir(),
		OpenEBSDataDir:          rc.EmbeddedClusterOpenEBSLocalSubDir(),
		ServiceCIDR:             rc.ServiceCIDR(),
	}

	return opts, nil
}

func (m *infraManager) installExtensions(ctx context.Context, hcli helm.Client) (finalErr error) {
	componentName := "Additional Components"

	if err := m.setComponentStatus(componentName, types.StateRunning, "Installing"); err != nil {
		return fmt.Errorf("set extensions status: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("install extensions recovered from panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			if err := m.setComponentStatus(componentName, types.StateFailed, finalErr.Error()); err != nil {
				m.logger.WithError(err).Error("set failed status")
			}
		} else {
			if err := m.setComponentStatus(componentName, types.StateSucceeded, ""); err != nil {
				m.logger.WithError(err).Error("set succeeded status")
			}
		}
	}()

	m.setStatusDesc(fmt.Sprintf("Installing %s", componentName))

	logFn := m.logFn("extensions")
	logFn("installing extensions")
	if err := extensions.Install(ctx, hcli, nil); err != nil {
		return fmt.Errorf("install extensions: %w", err)
	}
	return nil
}
