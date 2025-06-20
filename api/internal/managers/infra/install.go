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
	"github.com/sirupsen/logrus"
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

func (m *infraManager) Install(ctx context.Context, rc runtimeconfig.RuntimeConfig) (finalErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, err := m.k0scli.IsInstalled()
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

	license, err := helpers.ParseLicense(m.licenseFile)
	if err != nil {
		return fmt.Errorf("parse license: %w", err)
	}

	if err := m.initComponentsList(license, rc); err != nil {
		return fmt.Errorf("init components: %w", err)
	}

	if err := m.setStatus(types.StateRunning, ""); err != nil {
		return fmt.Errorf("set status: %w", err)
	}

	// Background context is used to avoid canceling the operation if the context is canceled
	go m.install(context.Background(), license, rc)

	return nil
}

func (m *infraManager) initComponentsList(license *kotsv1beta1.License, rc runtimeconfig.RuntimeConfig) error {
	components := []types.InfraComponent{{Name: K0sComponentName}}

	addOns := addons.GetAddOnsForInstall(rc, addons.InstallOptions{
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

func (m *infraManager) install(ctx context.Context, license *kotsv1beta1.License, rc runtimeconfig.RuntimeConfig) (finalErr error) {
	// extract airgap info if airgap bundle is provided
	var airgapInfo *kotsv1beta1.Airgap
	if m.airgapBundle != "" {
		var err error
		airgapInfo, err = airgap.AirgapInfoFromPath(m.airgapBundle)
		if err != nil {
			return fmt.Errorf("failed to get airgap info: %w", err)
		}
	}

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

	_, err := m.installK0s(ctx, rc)
	if err != nil {
		return fmt.Errorf("install k0s: %w", err)
	}

	kcli, err := m.kubeClient()
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}

	mcli, err := m.metadataClient()
	if err != nil {
		return fmt.Errorf("create metadata client: %w", err)
	}

	hcli, err := m.helmClient(rc)
	if err != nil {
		return fmt.Errorf("create helm client: %w", err)
	}
	defer hcli.Close()

	in, err := m.recordInstallation(ctx, kcli, license, rc, airgapInfo)
	if err != nil {
		return fmt.Errorf("record installation: %w", err)
	}

	if err := m.installAddOns(ctx, license, kcli, mcli, hcli, rc); err != nil {
		return fmt.Errorf("install addons: %w", err)
	}

	if err := m.installExtensions(ctx, hcli); err != nil {
		return fmt.Errorf("install extensions: %w", err)
	}

	if err := kubeutils.SetInstallationState(ctx, kcli, in, ecv1beta1.InstallationStateInstalled, "Installed"); err != nil {
		return fmt.Errorf("update installation: %w", err)
	}

	if err = support.CreateHostSupportBundle(ctx, kcli); err != nil {
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
				m.logger.WithField("error", err).Error("set failed status")
			}
		} else {
			if err := m.setComponentStatus(componentName, types.StateSucceeded, ""); err != nil {
				m.logger.WithField("error", err).Error("set succeeded status")
			}
		}
	}()

	m.setStatusDesc(fmt.Sprintf("Installing %s", componentName))

	logFn := m.logFn("k0s")

	logFn("creating k0s configuration file")
	k0sCfg, err := m.k0scli.WriteK0sConfig(ctx, rc.NetworkInterface(), m.airgapBundle, rc.PodCIDR(), rc.ServiceCIDR(), m.endUserConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("create config file: %w", err)
	}

	logFn("creating systemd unit files")
	if err := m.hostUtils.CreateSystemdUnitFiles(ctx, m.logger, rc, false); err != nil {
		return nil, fmt.Errorf("create systemd unit files: %w", err)
	}

	logFn("installing k0s")
	if err := m.k0scli.Install(rc); err != nil {
		return nil, fmt.Errorf("install cluster: %w", err)
	}

	logFn("waiting for k0s to be ready")
	if err := m.k0scli.WaitForK0s(); err != nil {
		return nil, fmt.Errorf("wait for k0s: %w", err)
	}

	kcli, err := m.kubeClient()
	if err != nil {
		return nil, fmt.Errorf("create kube client: %w", err)
	}

	m.setStatusDesc(fmt.Sprintf("Waiting for %s", componentName))

	logFn("waiting for node to be ready")
	if err := m.waitForNode(ctx, kcli); err != nil {
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

func (m *infraManager) recordInstallation(ctx context.Context, kcli client.Client, license *kotsv1beta1.License, rc runtimeconfig.RuntimeConfig, airgapInfo *kotsv1beta1.Airgap) (*ecv1beta1.Installation, error) {
	logFn := m.logFn("metadata")

	// get the configured custom domains
	ecDomains := utils.GetDomains(m.releaseData)

	// extract airgap uncompressed size if airgap info is provided
	var airgapUncompressedSize int64
	if airgapInfo != nil {
		airgapUncompressedSize = airgapInfo.Spec.UncompressedSize
	}

	// record the installation
	logFn("recording installation")
	in, err := kubeutils.RecordInstallation(ctx, kcli, kubeutils.RecordInstallationOptions{
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

func (m *infraManager) installAddOns(ctx context.Context, license *kotsv1beta1.License, kcli client.Client, mcli metadata.Interface, hcli helm.Client, rc runtimeconfig.RuntimeConfig) error {
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
		addons.WithRuntimeConfig(rc),
		addons.WithProgressChannel(progressChan),
	)

	opts := m.getAddonInstallOpts(license, rc)

	logFn("installing addons")
	if err := addOns.Install(ctx, opts); err != nil {
		return err
	}

	return nil
}

func (m *infraManager) getAddonInstallOpts(license *kotsv1beta1.License, rc runtimeconfig.RuntimeConfig) addons.InstallOptions {
	ecDomains := utils.GetDomains(m.releaseData)

	opts := addons.InstallOptions{
		AdminConsolePwd:         m.password,
		License:                 license,
		IsAirgap:                m.airgapBundle != "",
		TLSCertBytes:            m.tlsConfig.CertBytes,
		TLSKeyBytes:             m.tlsConfig.KeyBytes,
		Hostname:                m.tlsConfig.Hostname,
		DisasterRecoveryEnabled: license.Spec.IsDisasterRecoverySupported,
		IsMultiNodeEnabled:      license.Spec.IsEmbeddedClusterMultiNodeEnabled,
		EmbeddedConfigSpec:      m.getECConfigSpec(),
		EndUserConfigSpec:       m.getEndUserConfigSpec(),
	}

	if m.kotsInstaller != nil { // used for testing
		opts.KotsInstaller = m.kotsInstaller
	} else {
		opts.KotsInstaller = func() error {
			opts := kotscli.InstallOptions{
				RuntimeConfig:         rc,
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
		}
	}

	return opts
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

	m.setStatusDesc(fmt.Sprintf("Installing %s", componentName))

	logFn := m.logFn("extensions")
	logFn("installing extensions")
	if err := extensions.Install(ctx, hcli, nil); err != nil {
		return fmt.Errorf("install extensions: %w", err)
	}
	return nil
}
