package infra

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	addontypes "github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/support"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kyaml "sigs.k8s.io/yaml"
)

func (m *infraManager) Install(ctx context.Context, ki kubernetesinstallation.Installation, configValues kotsv1beta1.ConfigValues) (finalErr error) {
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

	if err := m.install(ctx, ki, configValues); err != nil {
		return err
	}

	return nil
}

func (m *infraManager) initComponentsList(license *kotsv1beta1.License, ki kubernetesinstallation.Installation, configValues kotsv1beta1.ConfigValues) error {
	components := []types.InfraComponent{}

	addOns := addons.GetAddOnsForKubernetesInstall(m.getAddonInstallOpts(license, ki, configValues))
	for _, addOn := range addOns {
		components = append(components, types.InfraComponent{Name: addOn.Name()})
	}

	for _, component := range components {
		if err := m.infraStore.RegisterComponent(component.Name); err != nil {
			return fmt.Errorf("register component: %w", err)
		}
	}
	return nil
}

func (m *infraManager) install(ctx context.Context, ki kubernetesinstallation.Installation, configValues kotsv1beta1.ConfigValues) error {
	license := &kotsv1beta1.License{}
	if err := kyaml.Unmarshal(m.license, license); err != nil {
		return fmt.Errorf("parse license: %w", err)
	}

	if err := m.initComponentsList(license, ki, configValues); err != nil {
		return fmt.Errorf("init components: %w", err)
	}

	_, err := m.recordInstallation(ctx, m.kcli, license, ki)
	if err != nil {
		return fmt.Errorf("record installation: %w", err)
	}

	if err := m.installAddOns(ctx, m.kcli, m.mcli, m.hcli, license, ki, configValues); err != nil {
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

func (m *infraManager) recordInstallation(ctx context.Context, kcli client.Client, license *kotsv1beta1.License, ki kubernetesinstallation.Installation) (*ecv1beta1.Installation, error) {
	// TODO: we may need this later

	return nil, nil
}

func (m *infraManager) installAddOns(ctx context.Context, kcli client.Client, mcli metadata.Interface, hcli helm.Client, license *kotsv1beta1.License, ki kubernetesinstallation.Installation, configValues kotsv1beta1.ConfigValues) error {
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

	opts := m.getAddonInstallOpts(license, ki, configValues)

	logFn("installing addons")
	if err := addOns.InstallKubernetes(ctx, opts); err != nil {
		return err
	}

	return nil
}

func (m *infraManager) getAddonInstallOpts(license *kotsv1beta1.License, ki kubernetesinstallation.Installation, configValues kotsv1beta1.ConfigValues) addons.KubernetesInstallOptions {
	opts := addons.KubernetesInstallOptions{
		AdminConsolePwd:    m.password,
		AdminConsolePort:   ki.AdminConsolePort(),
		License:            license,
		IsAirgap:           m.airgapBundle != "",
		TLSCertBytes:       m.tlsConfig.CertBytes,
		TLSKeyBytes:        m.tlsConfig.KeyBytes,
		Hostname:           m.tlsConfig.Hostname,
		IsMultiNodeEnabled: license.Spec.IsEmbeddedClusterMultiNodeEnabled,
		EmbeddedConfigSpec: m.getECConfigSpec(),
		EndUserConfigSpec:  m.getEndUserConfigSpec(),
		ProxySpec:          ki.ProxySpec(),
	}

	// TODO: no kots app install for now

	return opts
}
