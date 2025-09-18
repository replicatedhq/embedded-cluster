package appupgrademanager

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	kotscli "github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	"github.com/replicatedhq/embedded-cluster/pkg-new/constants"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kyaml "sigs.k8s.io/yaml"
)

// Upgrade upgrades the app with the provided config values
func (m *appUpgradeManager) Upgrade(ctx context.Context, configValues kotsv1beta1.ConfigValues) (finalErr error) {
	if err := m.setStatus(types.StateRunning, "Upgrading application"); err != nil {
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

	if err := m.upgrade(ctx, configValues); err != nil {
		return err
	}

	return nil
}

func (m *appUpgradeManager) upgrade(_ context.Context, configValues kotsv1beta1.ConfigValues) error {
	ecDomains := utils.GetDomains(m.releaseData)

	if m.releaseData == nil || m.releaseData.ChannelRelease == nil || m.releaseData.ChannelRelease.AppSlug == "" {
		return fmt.Errorf("release data with app slug is required for upgrade")
	}
	if m.releaseData.ChannelRelease.ChannelID == "" {
		return fmt.Errorf("release data with channel id is required for upgrade")
	}
	if m.releaseData.ChannelRelease.ChannelSequence == 0 {
		return fmt.Errorf("release data with channel sequence is required for upgrade")
	}

	appSlug := m.releaseData.ChannelRelease.AppSlug
	channelID := m.releaseData.ChannelRelease.ChannelID
	channelSequence := m.releaseData.ChannelRelease.ChannelSequence

	configValuesFile, err := m.createConfigValuesFile(configValues)
	if err != nil {
		return fmt.Errorf("creating config values file: %w", err)
	}
	defer os.Remove(configValuesFile)

	deployOpts := kotscli.DeployOptions{
		AppSlug:               appSlug,
		License:               m.license,
		Namespace:             constants.KotsadmNamespace,
		ClusterID:             m.clusterID,
		AirgapBundle:          m.airgapBundle,
		ConfigValuesFile:      configValuesFile,
		ChannelID:             channelID,
		ChannelSequence:       channelSequence,
		ReplicatedAppEndpoint: netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain),
		// Skip running the KOTS app preflights in the Admin Console; they run in the manager experience installer when ENABLE_V3 is enabled
		SkipPreflights: os.Getenv("ENABLE_V3") == "1",
		Stdout:         m.newLogWriter(),
	}

	if m.airgapBundle != "" {
		m.log(deployOpts, "Deploying airgap bundle with license sync, configuration, and deployment")
	} else {
		m.log(deployOpts, "Deploying online update with license sync, configuration, and deployment")
	}

	// Deploy with KOTS
	if m.kotsCLI != nil {
		if err := m.kotsCLI.Deploy(deployOpts); err != nil {
			return fmt.Errorf("kots cli deploy: %w", err)
		}
	} else {
		if err := kotscli.Deploy(deployOpts); err != nil {
			return fmt.Errorf("kots cli deploy: %w", err)
		}
	}

	return nil
}

// createConfigValuesFile creates a temporary file with the config values
func (m *appUpgradeManager) createConfigValuesFile(configValues kotsv1beta1.ConfigValues) (string, error) {
	// Use Kubernetes-specific YAML serialization to properly handle TypeMeta and ObjectMeta
	data, err := kyaml.Marshal(configValues)
	if err != nil {
		return "", fmt.Errorf("marshaling config values: %w", err)
	}

	configValuesFile, err := os.CreateTemp("", "config-values*.yaml")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer configValuesFile.Close()

	if _, err := configValuesFile.Write(data); err != nil {
		_ = os.Remove(configValuesFile.Name())
		return "", fmt.Errorf("write config values to temp file: %w", err)
	}

	return configValuesFile.Name(), nil
}
