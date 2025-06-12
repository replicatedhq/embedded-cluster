package infra

import (
	"context"
	"fmt"
	"runtime/debug"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/api/pkg/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/hostutils"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	ecmetadata "github.com/replicatedhq/embedded-cluster/pkg-new/metadata"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/addons/registry"
	addontypes "github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/extensions"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/support"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const K0sComponentName = "Runtime"

func AlreadyInstalledError() error {
	return fmt.Errorf(
		"\nAn installation is detected on this machine.\nTo install, you must first remove the existing installation.\nYou can do this by running the following command:\n\n  sudo ./%s reset\n",
		runtimeconfig.BinaryName(),
	)
}

func (m *infraManager) Install(ctx context.Context, config *types.InstallationConfig) (finalErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, err := k0s.IsInstalled()
	if err != nil {
		return err
	}
	if installed {
		return AlreadyInstalledError()
	}

	didRun, err := m.installDidRun()
	if err != nil {
		return fmt.Errorf("check if install did run: %w", err)
	}
	if didRun {
		return fmt.Errorf("install can only be run once")
	}

	if config == nil {
		return fmt.Errorf("installation config is required")
	}

	// Build proxy spec
	var proxy *ecv1beta1.ProxySpec
	if config.HTTPProxy != "" || config.HTTPSProxy != "" || config.NoProxy != "" {
		proxy = &ecv1beta1.ProxySpec{
			HTTPProxy:  config.HTTPProxy,
			HTTPSProxy: config.HTTPSProxy,
			NoProxy:    config.NoProxy,
		}
	}

	license, err := helpers.ParseLicense(m.licenseFile)
	if err != nil {
		return fmt.Errorf("parse license: %w", err)
	}

	if err := m.initComponentsList(license); err != nil {
		return fmt.Errorf("init components: %w", err)
	}

	if err := m.setStatus(types.StateRunning, ""); err != nil {
		return fmt.Errorf("set status: %w", err)
	}

	// Run install in background
	go m.install(context.Background(), config, proxy, license)

	return nil
}

func (m *infraManager) initComponentsList(license *kotsv1beta1.License) error {
	components := []types.InfraComponent{{Name: K0sComponentName}}

	addOns := addons.GetAddOnsForInstall(addons.InstallOptions{
		IsAirgap:                m.airgapBundle != "",
		DisasterRecoveryEnabled: license.Spec.IsDisasterRecoverySupported,
	})
	for _, addOn := range addOns {
		components = append(components, types.InfraComponent{Name: addOn.Name()})
	}

	components = append(components, types.InfraComponent{Name: "Additional Components"})

	for _, component := range components {
		if err := m.infraStore.RegisterComponent(component.Name); err != nil {
			return fmt.Errorf("register component: %w", err)
		}
	}
	return nil
}

func (m *infraManager) install(ctx context.Context, config *types.InstallationConfig, proxy *ecv1beta1.ProxySpec, license *kotsv1beta1.License) (finalErr error) {
	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			if err := m.setStatus(types.StateFailed, finalErr.Error()); err != nil {
				m.logger.WithField("error", err).Error("set failed status")
			}
		} else {
			if err := m.setStatus(types.StateSucceeded, "Installation complete"); err != nil {
				m.logger.WithField("error", err).Error("set succeeded status")
			}
		}
	}()

	k0sCfg, err := m.installK0s(ctx, config, proxy)
	if err != nil {
		return fmt.Errorf("install k0s: %w", err)
	}

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}

	mcli, err := kubeutils.MetadataClient()
	if err != nil {
		return fmt.Errorf("create metadata client: %w", err)
	}

	hcli, err := m.getHelmClient()
	if err != nil {
		return fmt.Errorf("create helm client: %w", err)
	}
	defer hcli.Close()

	in, err := m.recordInstallation(ctx, kcli, proxy, license, k0sCfg)
	if err != nil {
		return fmt.Errorf("record installation: %w", err)
	}

	if err := m.installAddOns(ctx, config, proxy, license, kcli, mcli, hcli); err != nil {
		return fmt.Errorf("install addons: %w", err)
	}

	if err := m.installExtensions(ctx, hcli); err != nil {
		return fmt.Errorf("install extensions: %w", err)
	}

	if err := kubeutils.SetInstallationState(ctx, kcli, in, ecv1beta1.InstallationStateInstalled, "Installed"); err != nil {
		return fmt.Errorf("update installation: %w", err)
	}

	if err = support.CreateHostSupportBundle(); err != nil {
		m.logger.Warnf("Unable to create host support bundle: %v", err)
	}

	return nil
}

func (m *infraManager) installK0s(ctx context.Context, config *types.InstallationConfig, proxy *ecv1beta1.ProxySpec) (k0sCfg *k0sv1beta1.ClusterConfig, finalErr error) {
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
				m.logger.WithField("error", err).Error("set failed status")
			}
		} else {
			if err := m.setComponentStatus(componentName, types.StateSucceeded, ""); err != nil {
				m.logger.WithField("error", err).Error("set succeeded status")
			}
		}
	}()

	m.logger.Debug("creating k0s configuration file")
	k0sCfg, err := k0s.WriteK0sConfig(ctx, config.NetworkInterface, m.airgapBundle, config.PodCIDR, config.ServiceCIDR, m.endUserConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("create config file: %w", err)
	}

	m.logger.Debug("creating systemd unit files")
	if err := hostutils.CreateSystemdUnitFiles(ctx, m.logger, m.rc, false, proxy); err != nil {
		return nil, fmt.Errorf("create systemd unit files: %w", err)
	}

	m.logger.Debug("installing k0s")
	if err := k0s.Install(m.rc, config.NetworkInterface); err != nil {
		return nil, fmt.Errorf("install cluster: %w", err)
	}

	m.logger.Debug("waiting for k0s to be ready")
	if err := k0s.WaitForK0s(); err != nil {
		return nil, fmt.Errorf("wait for k0s: %w", err)
	}

	kcli, err := kubeutils.KubeClient()
	if err != nil {
		return nil, fmt.Errorf("create kube client: %w", err)
	}

	m.logger.Debug("waiting for node to be ready")
	if err := m.waitForNode(ctx, kcli); err != nil {
		return nil, fmt.Errorf("wait for node: %w", err)
	}

	m.logger.Debugf("adding insecure registry")
	registryIP, err := registry.GetRegistryClusterIP(config.ServiceCIDR)
	if err != nil {
		return nil, fmt.Errorf("get registry cluster IP: %w", err)
	}
	if err := airgap.AddInsecureRegistry(fmt.Sprintf("%s:5000", registryIP)); err != nil {
		return nil, fmt.Errorf("add insecure registry: %w", err)
	}

	return k0sCfg, nil
}

func (m *infraManager) recordInstallation(ctx context.Context, kcli client.Client, proxy *ecv1beta1.ProxySpec, license *kotsv1beta1.License, k0sCfg *k0sv1beta1.ClusterConfig) (*ecv1beta1.Installation, error) {
	// get the configured custom domains
	ecDomains := utils.GetDomains(m.releaseData)

	// record the installation
	m.logger.Debugf("recording installation")
	in, err := kubeutils.RecordInstallation(ctx, kcli, kubeutils.RecordInstallationOptions{
		IsAirgap:       m.airgapBundle != "",
		Proxy:          proxy,
		K0sConfig:      k0sCfg,
		License:        license,
		ConfigSpec:     m.getECConfigSpec(),
		MetricsBaseURL: netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain),
		RuntimeConfig:  m.rc.Get(),
		EndUserConfig:  m.endUserConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("record installation: %w", err)
	}

	if err := ecmetadata.CreateVersionMetadataConfigmap(ctx, kcli); err != nil {
		return nil, fmt.Errorf("create version metadata configmap: %w", err)
	}

	return in, nil
}

func (m *infraManager) installAddOns(
	ctx context.Context,
	config *types.InstallationConfig,
	proxy *ecv1beta1.ProxySpec,
	license *kotsv1beta1.License,
	kcli client.Client,
	mcli metadata.Interface,
	hcli helm.Client,
) error {
	// get the configured custom domains
	ecDomains := utils.GetDomains(m.releaseData)

	progressChan := make(chan addontypes.AddOnProgress)
	defer close(progressChan)

	go func() {
		for progress := range progressChan {
			if err := m.setComponentStatus(progress.Name, progress.Status.State, progress.Status.Description); err != nil {
				m.logger.Errorf("Failed to update addon status: %v", err)
			}
		}
	}()

	addOns := addons.New(
		addons.WithLogFunc(m.logger.Debugf),
		addons.WithKubernetesClient(kcli),
		addons.WithMetadataClient(mcli),
		addons.WithHelmClient(hcli),
		addons.WithRuntimeConfig(m.rc),
		addons.WithProgressChannel(progressChan),
	)

	m.logger.Debugf("installing addons")
	if err := addOns.Install(ctx, addons.InstallOptions{
		AdminConsolePwd:         m.password,
		License:                 license,
		IsAirgap:                m.airgapBundle != "",
		Proxy:                   proxy,
		TLSCertBytes:            m.tlsConfig.CertBytes,
		TLSKeyBytes:             m.tlsConfig.KeyBytes,
		Hostname:                m.tlsConfig.Hostname,
		ServiceCIDR:             config.ServiceCIDR,
		DisasterRecoveryEnabled: license.Spec.IsDisasterRecoverySupported,
		IsMultiNodeEnabled:      license.Spec.IsEmbeddedClusterMultiNodeEnabled,
		EmbeddedConfigSpec:      m.getECConfigSpec(),
		EndUserConfigSpec:       m.getEndUserConfigSpec(),
		KotsInstaller: func() error {
			opts := kotscli.InstallOptions{
				RuntimeConfig:         m.rc,
				AppSlug:               license.Spec.AppSlug,
				LicenseFile:           m.licenseFile,
				Namespace:             runtimeconfig.KotsadmNamespace,
				AirgapBundle:          m.airgapBundle,
				ConfigValuesFile:      m.configValues,
				ReplicatedAppEndpoint: netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain),
				// TODO (@salah): capture kots install logs
				// Stdout:                stdout,
			}
			return kotscli.Install(opts)
		},
	}); err != nil {
		return err
	}

	return nil
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
				m.logger.WithField("error", err).Error("set failed status")
			}
		} else {
			if err := m.setComponentStatus(componentName, types.StateSucceeded, ""); err != nil {
				m.logger.WithField("error", err).Error("set succeeded status")
			}
		}
	}()

	m.logger.Debugf("installing extensions")
	if err := extensions.Install(ctx, hcli, nil); err != nil {
		return fmt.Errorf("install extensions: %w", err)
	}
	return nil
}
