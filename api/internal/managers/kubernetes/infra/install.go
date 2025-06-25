package infra

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	addontypes "github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/metadata"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (m *infraManager) Install(ctx context.Context, ki kubernetesinstallation.Installation) (finalErr error) {
	if err := m.setStatus(types.StateRunning, ""); err != nil {
		return fmt.Errorf("set status: %w", err)
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

	if err := m.install(ctx, ki); err != nil {
		return err
	}

	return nil
}

func (m *infraManager) initComponentsList(ki kubernetesinstallation.Installation) error {
	components := []types.KubernetesInfraComponent{}

	addOns := addons.GetAddOnsForKubernetesInstall(addons.KubernetesInstallOptions{})
	for _, addOn := range addOns {
		components = append(components, types.KubernetesInfraComponent{Name: addOn.Name()})
	}

	for _, component := range components {
		if err := m.infraStore.RegisterComponent(component.Name); err != nil {
			return fmt.Errorf("register component: %w", err)
		}
	}
	return nil
}

func (m *infraManager) install(ctx context.Context, ki kubernetesinstallation.Installation) error {
	if err := m.initComponentsList(ki); err != nil {
		return fmt.Errorf("init components: %w", err)
	}

	kcli, err := m.kubeClient()
	if err != nil {
		return fmt.Errorf("create kube client: %w", err)
	}

	mcli, err := m.metadataClient()
	if err != nil {
		return fmt.Errorf("create metadata client: %w", err)
	}

	hcli, err := m.helmClient(ki)
	if err != nil {
		return fmt.Errorf("create helm client: %w", err)
	}
	defer hcli.Close()

	if err := m.installAddOns(ctx, kcli, mcli, hcli, ki); err != nil {
		return fmt.Errorf("install addons: %w", err)
	}

	if err := m.recordInstallation(ctx, kcli, ki); err != nil {
		return fmt.Errorf("record installation: %w", err)
	}

	return nil
}

func (m *infraManager) recordInstallation(ctx context.Context, kcli client.Client, ki kubernetesinstallation.Installation) error {
	logFn := m.logFn("metadata")

	ki.SetStatus(ecv1beta1.KubernetesInstallationStatus{
		State: ecv1beta1.KubernetesInstallationStateInstalled,
	})

	// record the installation
	logFn("recording installation")
	if err := kubeutils.RecordKubernetesInstallation(ctx, kcli, ki); err != nil {
		return fmt.Errorf("record installation: %w", err)
	}

	return nil
}

func (m *infraManager) installAddOns(ctx context.Context, kcli client.Client, mcli metadata.Interface, hcli helm.Client, ki kubernetesinstallation.Installation) error {
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
		addons.WithProgressChannel(progressChan),
	)

	opts := addons.KubernetesInstallOptions{
		AdminConsolePwd:    m.password,
		AdminConsolePort:   ki.AdminConsolePort(),
		ProxySpec:          ki.ProxySpec(),
		EmbeddedConfigSpec: m.getECConfigSpec(),
		EndUserConfigSpec:  m.getEndUserConfigSpec(),
		IsAirgap:           false,
	}

	logFn("installing addons")
	if err := addOns.InstallKubernetes(ctx, opts); err != nil {
		return err
	}

	return nil
}
