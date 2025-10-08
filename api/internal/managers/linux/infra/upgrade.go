package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/upgrade"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	addontypes "github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kyaml "sigs.k8s.io/yaml"
)

// RequiresUpgrade returns true if the embedded cluster is in a state that requires an infrastructure upgrade.
// This is determined by checking that:
// - The current embedded cluster config (as part of the Installation object) already exists in the cluster.
// - The new embedded cluster configuration differs from the current embedded cluster configuration.
func (m *infraManager) RequiresUpgrade(ctx context.Context) (bool, error) {
	current, err := kubeutils.GetLatestInstallation(ctx, m.kcli)
	if err != nil {
		return false, fmt.Errorf("get current installation: %w", err)
	}

	curcfg := current.Spec.Config
	newcfg := m.getECConfigSpec()

	serializedCur, err := json.Marshal(curcfg)
	if err != nil {
		return false, fmt.Errorf("marshal current embedded cluster config: %w", err)
	}
	serializedNew, err := json.Marshal(newcfg)
	if err != nil {
		return false, fmt.Errorf("marshal new embedded cluster config: %w", err)
	}
	return !bytes.Equal(serializedCur, serializedNew), nil
}

// Upgrade performs the infrastructure upgrade by orchestrating the upgrade steps
func (m *infraManager) Upgrade(ctx context.Context, rc runtimeconfig.RuntimeConfig) (finalErr error) {
	// TODO NOW: reporting

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
			if err := m.setStatus(types.StateSucceeded, "Upgrade complete"); err != nil {
				m.logger.WithError(err).Error("set succeeded status")
			}
		}
	}()

	if err := m.upgrade(ctx, rc); err != nil {
		return err
	}

	return nil
}

func (m *infraManager) upgrade(ctx context.Context, rc runtimeconfig.RuntimeConfig) error {
	if m.upgrader == nil {
		// initialize the manager's helm and kube clients
		if err := m.setupClients(rc.PathToKubeConfig(), rc.EmbeddedClusterChartsSubDir()); err != nil {
			return fmt.Errorf("setup clients: %w", err)
		}

		// initialize the upgrader
		m.upgrader = upgrade.NewInfraUpgrader(
			upgrade.WithKubeClient(m.kcli),
			upgrade.WithHelmClient(m.hcli),
			upgrade.WithRuntimeConfig(rc),
			upgrade.WithLogger(m.logger),
		)
	}

	in, err := m.newInstallationObj(ctx)
	if err != nil {
		return fmt.Errorf("new installation: %w", err)
	}

	m.logger.WithField("version", in.Spec.Config.Version).Info("Starting infrastructure upgrade")

	if err := m.initUpgradeComponentsList(in); err != nil {
		return fmt.Errorf("init components: %w", err)
	}

	if err := m.recordUpgrade(ctx, in); err != nil {
		return fmt.Errorf("record upgrade: %w", err)
	}

	if err := m.upgradeK0s(ctx, in); err != nil {
		return fmt.Errorf("upgrade k0s: %w", err)
	}

	if err := m.upgradeAddOns(ctx, in); err != nil {
		return fmt.Errorf("upgrade addons: %w", err)
	}

	if err := m.upgradeExtensions(ctx, in); err != nil {
		return fmt.Errorf("upgrade extensions: %w", err)
	}

	if err := kubeutils.SetInstallationState(ctx, m.kcli, in, ecv1beta1.InstallationStateInstalled, "Upgraded"); err != nil {
		return fmt.Errorf("update installation: %w", err)
	}

	if err := m.upgrader.CreateHostSupportBundle(ctx); err != nil {
		m.logger.Warnf("Unable to create host support bundle: %v", err)
	}

	return nil
}

func (m *infraManager) newInstallationObj(ctx context.Context) (*ecv1beta1.Installation, error) {
	current, err := kubeutils.GetLatestInstallation(ctx, m.kcli)
	if err != nil {
		return nil, fmt.Errorf("get current installation: %w", err)
	}

	license := &kotsv1beta1.License{}
	if err := kyaml.Unmarshal(m.license, license); err != nil {
		return nil, fmt.Errorf("parse license: %w", err)
	}

	in := &ecv1beta1.Installation{
		TypeMeta: metav1.TypeMeta{
			APIVersion: ecv1beta1.GroupVersion.String(),
			Kind:       "Installation",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: time.Now().Format("20060102150405"),
			Labels: map[string]string{
				"replicated.com/disaster-recovery": "ec-install",
			},
		},
		Spec: current.Spec,
	}
	// TODO: configure when we support airgap
	// in.Spec.Artifacts = artifacts
	in.Spec.Config = m.getECConfigSpec()
	in.Spec.LicenseInfo = &ecv1beta1.LicenseInfo{
		IsDisasterRecoverySupported: license.Spec.IsDisasterRecoverySupported,
		IsMultiNodeEnabled:          license.Spec.IsEmbeddedClusterMultiNodeEnabled,
	}

	return in, nil
}

func (m *infraManager) initUpgradeComponentsList(in *ecv1beta1.Installation) error {
	components := []types.InfraComponent{{Name: K0sComponentName}}

	addOnsNames := addons.GetAddOnsNamesForUpgrade(in.Spec.AirGap, in.Spec.LicenseInfo.IsDisasterRecoverySupported, in.Spec.HighAvailability)
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

func (m *infraManager) recordUpgrade(ctx context.Context, in *ecv1beta1.Installation) error {
	logFn := m.logFn("metadata")

	logFn("creating installation object")
	if err := m.upgrader.CreateInstallation(ctx, in); err != nil {
		return fmt.Errorf("create installation: %w", err)
	}

	logFn("copying version metadata to cluster")
	if err := m.upgrader.CopyVersionMetadataToCluster(ctx, in); err != nil {
		return fmt.Errorf("copy version metadata: %w", err)
	}

	return nil
}

func (m *infraManager) upgradeK0s(ctx context.Context, in *ecv1beta1.Installation) (finalErr error) {
	componentName := K0sComponentName

	if err := m.setComponentStatus(componentName, types.StateRunning, "Upgrading"); err != nil {
		return fmt.Errorf("set component status: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("upgrade k0s recovered from panic: %v: %s", r, string(debug.Stack()))
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

	m.setStatusDesc(fmt.Sprintf("Upgrading %s", componentName))

	logFn := m.logFn("k0s")

	logFn("distributing artifacts")
	if err := m.distributeArtifacts(ctx, in); err != nil {
		return fmt.Errorf("distribute artifacts: %w", err)
	}

	logFn("upgrading k0s")
	if err := m.upgrader.UpgradeK0s(ctx, in); err != nil {
		return fmt.Errorf("k0s upgrade: %w", err)
	}

	logFn("updating cluster config")
	if err := m.upgrader.UpdateClusterConfig(ctx, in); err != nil {
		return fmt.Errorf("cluster config update: %w", err)
	}

	return nil
}

func (m *infraManager) distributeArtifacts(ctx context.Context, in *ecv1beta1.Installation) error {
	license := &kotsv1beta1.License{}
	if err := kyaml.Unmarshal(m.license, license); err != nil {
		return fmt.Errorf("parse license: %w", err)
	}

	if m.releaseData == nil || m.releaseData.ChannelRelease == nil {
		return fmt.Errorf("release data with channel release is required for distribute artifacts")
	}

	appSlug := m.releaseData.ChannelRelease.AppSlug
	channelID := m.releaseData.ChannelRelease.ChannelID
	appVersion := m.releaseData.ChannelRelease.VersionLabel

	// TODO: configure when we support airgap
	localArtifactMirrorImage := ""

	return m.upgrader.DistributeArtifacts(ctx, in, localArtifactMirrorImage, license.Spec.LicenseID, appSlug, channelID, appVersion)
}

func (m *infraManager) upgradeAddOns(ctx context.Context, in *ecv1beta1.Installation) error {
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

	m.logFn("addons")("upgrading addons")

	if err := m.upgrader.UpgradeAddons(ctx, in, progressChan); err != nil {
		return err
	}

	return nil
}

func (m *infraManager) upgradeExtensions(ctx context.Context, in *ecv1beta1.Installation) (finalErr error) {
	componentName := "Additional Components"

	if err := m.setComponentStatus(componentName, types.StateRunning, "Upgrading"); err != nil {
		return fmt.Errorf("set extensions status: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("upgrade extensions recovered from panic: %v: %s", r, string(debug.Stack()))
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

	m.setStatusDesc(fmt.Sprintf("Upgrading %s", componentName))

	logFn := m.logFn("extensions")
	logFn("upgrading extensions")
	if err := m.upgrader.UpgradeExtensions(ctx, in); err != nil {
		return fmt.Errorf("upgrade extensions: %w", err)
	}
	return nil
}
