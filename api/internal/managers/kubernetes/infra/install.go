package infra

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	addontypes "github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/support"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (m *infraManager) Install(ctx context.Context, ki kubernetesinstallation.Installation) (finalErr error) {
	// TODO: check if kots is already installed

	if err := m.setStatus(types.StateRunning, ""); err != nil {
		return fmt.Errorf("set status: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			if err := m.setStatus(types.StateFailed, finalErr.Error()); err != nil {
				m.logger.WithError(err).Error("set failed status")
			}
		} else {
			if err := m.setStatus(types.StateSucceeded, "Installation complete"); err != nil {
				m.logger.WithError(err).Error("set succeeded status")
			}
		}
	}()

	if err := m.install(ctx, ki); err != nil {
		return err
	}

	return nil
}

func (m *infraManager) initInstallComponentsList() error {
	components := []types.InfraComponent{}

	addOnsNames := addons.GetAddOnsNamesForKubernetesInstall()
	for _, addOnName := range addOnsNames {
		components = append(components, types.InfraComponent{Name: addOnName})
	}

	for _, component := range components {
		if err := m.infraStore.RegisterComponent(component.Name); err != nil {
			return fmt.Errorf("register component: %w", err)
		}
	}
	return nil
}

func (m *infraManager) install(ctx context.Context, ki kubernetesinstallation.Installation) error {
	license, err := helpers.ParseLicenseFromBytes(m.license)
	if err != nil {
		return fmt.Errorf("parse license: %w", err)
	}

	if err := m.initInstallComponentsList(); err != nil {
		return fmt.Errorf("init components: %w", err)
	}

	_, err = m.recordInstallation(ctx, m.kcli, license, ki)
	if err != nil {
		return fmt.Errorf("record installation: %w", err)
	}

	if err := m.installAddOns(ctx, m.kcli, m.mcli, m.hcli, license, ki); err != nil {
		return fmt.Errorf("install addons: %w", err)
	}

	// TODO: we may need this later
	// if err := kubeutils.SetInstallationState(ctx, m.kcli, in, ecv1beta1.InstallationStateInstalled, "Installed"); err != nil {
	// 	return fmt.Errorf("update installation: %w", err)
	// }

	if err = support.CreateHostSupportBundle(ctx, m.kcli); err != nil {
		m.logger.Warnf("Unable to create host support bundle: %v", err)
	}

	return nil
}

func (m *infraManager) recordInstallation(ctx context.Context, kcli client.Client, license licensewrapper.LicenseWrapper, ki kubernetesinstallation.Installation) (*ecv1beta1.Installation, error) {
	// TODO: we may need this later

	return nil, nil
}

func (m *infraManager) installAddOns(ctx context.Context, kcli client.Client, mcli metadata.Interface, hcli helm.Client, license licensewrapper.LicenseWrapper, ki kubernetesinstallation.Installation) error {
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

	opts, err := m.getAddonInstallOpts(ctx, license, ki)
	if err != nil {
		return fmt.Errorf("get addon install options: %w", err)
	}

	logFn("installing addons")
	if err := addOns.InstallKubernetes(ctx, opts); err != nil {
		return err
	}

	return nil
}

func (m *infraManager) getAddonInstallOpts(ctx context.Context, license licensewrapper.LicenseWrapper, ki kubernetesinstallation.Installation) (addons.KubernetesInstallOptions, error) {
	// TODO: We should not use the runtimeconfig package for kubernetes target installs. Since runtimeconfig.KotsadmNamespace is
	// target agnostic, we should move it to a package that can be used by both linux/kubernetes targets.
	kotsadmNamespace, err := runtimeconfig.KotsadmNamespace(ctx, m.kcli)
	if err != nil {
		return addons.KubernetesInstallOptions{}, fmt.Errorf("get kotsadm namespace: %w", err)
	}
	opts := addons.KubernetesInstallOptions{
		AdminConsolePwd:    m.password,
		AdminConsolePort:   ki.AdminConsolePort(),
		License:            license,
		IsAirgap:           m.airgapBundle != "",
		TLSCertBytes:       m.tlsConfig.CertBytes,
		TLSKeyBytes:        m.tlsConfig.KeyBytes,
		Hostname:           m.tlsConfig.Hostname,
		IsMultiNodeEnabled: license.IsEmbeddedClusterMultiNodeEnabled(),
		EmbeddedConfigSpec: m.getECConfigSpec(),
		EndUserConfigSpec:  m.getEndUserConfigSpec(),
		KotsadmNamespace:   kotsadmNamespace,
		ProxySpec:          ki.ProxySpec(),
	}

	// TODO: no kots app install for now

	return opts, nil
}
