package infra

import (
	"context"
	"fmt"
	"path"
	"runtime/debug"
	"strings"
	"time"

	"github.com/distribution/reference"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/upgrade"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	addontypes "github.com/replicatedhq/embedded-cluster/pkg/addons/types"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kyaml "sigs.k8s.io/yaml"
)

// Upgrade performs the infrastructure upgrade by orchestrating the upgrade steps
func (m *infraManager) Upgrade(ctx context.Context, rc runtimeconfig.RuntimeConfig, registrySettings *types.RegistrySettings) (finalErr error) {
	// TODO: reporting

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

	if err := m.upgrade(ctx, rc, registrySettings); err != nil {
		return err
	}

	return nil
}

func (m *infraManager) upgrade(ctx context.Context, rc runtimeconfig.RuntimeConfig, registrySettings *types.RegistrySettings) error {
	if m.upgrader == nil {
		// ensure the manager's clients are initialized
		if err := m.setupClients(rc); err != nil {
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

	in, err := m.newInstallationObj(ctx, registrySettings)
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

	if err := m.upgradeK0s(ctx, in, registrySettings); err != nil {
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

func (m *infraManager) newInstallationObj(ctx context.Context, registrySettings *types.RegistrySettings) (*ecv1beta1.Installation, error) {
	current, err := kubeutils.GetLatestInstallation(ctx, m.kcli)
	if err != nil {
		return nil, fmt.Errorf("get current installation: %w", err)
	}

	license := &kotsv1beta1.License{}
	if err := kyaml.Unmarshal(m.license, license); err != nil {
		return nil, fmt.Errorf("parse license: %w", err)
	}

	artifacts, err := m.getECArtifacts(registrySettings)
	if err != nil {
		return nil, fmt.Errorf("get EC artifacts: %w", err)
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
	in.Spec.Artifacts = artifacts
	in.Spec.Config = m.getECConfigSpec()
	in.Spec.LicenseInfo = &ecv1beta1.LicenseInfo{
		IsDisasterRecoverySupported: license.Spec.IsDisasterRecoverySupported,
		IsMultiNodeEnabled:          license.Spec.IsEmbeddedClusterMultiNodeEnabled,
	}

	return in, nil
}

// getECArtifacts returns the path of the different EC artifacts in the registry
// reference from KOTS: https://github.com/replicatedhq/kots/blob/d26ebd2acaccc54313e7f7d5ca3ca580ae1a0bc5/pkg/upstream/fetch.go#L126-L151
func (m *infraManager) getECArtifacts(registrySettings *types.RegistrySettings) (*ecv1beta1.ArtifactsLocation, error) {
	if m.airgapBundle == "" {
		// not an airgap installation
		return nil, nil
	}

	if m.airgapMetadata == nil || m.airgapMetadata.AirgapInfo == nil {
		return nil, fmt.Errorf("airgap metadata is nil")
	}

	airgapInfo := m.airgapMetadata.AirgapInfo

	if airgapInfo.Spec.EmbeddedClusterArtifacts == nil {
		return nil, nil
	}

	opts := ECArtifactOCIPathOptions{
		RegistryHost:      registrySettings.Host,
		RegistryNamespace: registrySettings.Namespace,
		ChannelID:         airgapInfo.Spec.ChannelID,
		UpdateCursor:      airgapInfo.Spec.UpdateCursor,
		VersionLabel:      airgapInfo.Spec.VersionLabel,
	}
	return &ecv1beta1.ArtifactsLocation{
		EmbeddedClusterBinary:   newECOCIArtifactPath(airgapInfo.Spec.EmbeddedClusterArtifacts.BinaryAmd64, opts).String(),
		HelmCharts:              newECOCIArtifactPath(airgapInfo.Spec.EmbeddedClusterArtifacts.Charts, opts).String(),
		Images:                  newECOCIArtifactPath(airgapInfo.Spec.EmbeddedClusterArtifacts.ImagesAmd64, opts).String(),
		EmbeddedClusterMetadata: newECOCIArtifactPath(airgapInfo.Spec.EmbeddedClusterArtifacts.Metadata, opts).String(),
		AdditionalArtifacts: map[string]string{
			"kots":     newECOCIArtifactPath(airgapInfo.Spec.EmbeddedClusterArtifacts.AdditionalArtifacts["kots"], opts).String(),
			"operator": newECOCIArtifactPath(airgapInfo.Spec.EmbeddedClusterArtifacts.AdditionalArtifacts["operator"], opts).String(),
		},
	}, nil
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

func (m *infraManager) upgradeK0s(ctx context.Context, in *ecv1beta1.Installation, registrySettings *types.RegistrySettings) (finalErr error) {
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
	if err := m.distributeArtifacts(ctx, in, registrySettings); err != nil {
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

func (m *infraManager) distributeArtifacts(ctx context.Context, in *ecv1beta1.Installation, registrySettings *types.RegistrySettings) error {
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

	// Determine the local artifact mirror image path
	localArtifactMirrorImage := versions.LocalArtifactMirrorImage

	// For airgap installations, rewrite the LAM image to point to the EC registry
	if registrySettings != nil && registrySettings.HasLocalRegistry {
		destImage, err := destECImage(registrySettings, localArtifactMirrorImage)
		if err != nil {
			return fmt.Errorf("determine LAM image path in EC registry: %w", err)
		}
		localArtifactMirrorImage = destImage
	}

	return m.upgrader.DistributeArtifacts(ctx, in, localArtifactMirrorImage, license.Spec.LicenseID, appSlug, channelID, appVersion)
}

// destECImage returns the location to an EC image in the registry
// reference from KOTS: https://github.com/replicatedhq/kots/blob/d26ebd2acaccc54313e7f7d5ca3ca580ae1a0bc5/pkg/imageutil/image.go#L105
func destECImage(registrySettings *types.RegistrySettings, srcImage string) (string, error) {
	// parsing as a docker reference strips the tag if both a tag and a digest are used
	parsed, err := reference.ParseDockerRef(srcImage)
	if err != nil {
		return "", fmt.Errorf("failed to normalize source image %s: %w", srcImage, err)
	}
	srcImage = parsed.String()

	imageParts := strings.Split(srcImage, "/")
	lastPart := imageParts[len(imageParts)-1]

	return path.Join(registrySettings.Host, registrySettings.Namespace, "embedded-cluster", lastPart), nil
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
