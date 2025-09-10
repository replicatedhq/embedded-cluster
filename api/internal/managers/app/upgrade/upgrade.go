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

func (m *appUpgradeManager) upgrade(ctx context.Context, configValues kotsv1beta1.ConfigValues) error {
	ecDomains := utils.GetDomains(m.releaseData)

	if m.releaseData == nil || m.releaseData.ChannelRelease == nil || m.releaseData.ChannelRelease.AppSlug == "" {
		return fmt.Errorf("release data with app slug is required for upgrade")
	}
	if m.releaseData.ChannelRelease.VersionLabel == "" {
		return fmt.Errorf("release data with version label is required for upgrade")
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
	versionLabel := m.releaseData.ChannelRelease.VersionLabel

	upstreamUpgradeOpts := kotscli.UpstreamUpgradeOptions{
		AppSlug:               appSlug,
		Namespace:             constants.KotsadmNamespace,
		ClusterID:             m.clusterID,
		AirgapBundle:          m.airgapBundle,
		ReplicatedAppEndpoint: netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain),
		// Skip running the KOTS app preflights in the Admin Console; they run in the manager experience installer when ENABLE_V3 is enabled
		SkipPreflights: os.Getenv("ENABLE_V3") == "1",
		Stdout:         m.newLogWriter(),
	}

	if m.airgapBundle != "" {
		m.log(upstreamUpgradeOpts, "Uploading and processing airgap bundle")
	} else {
		m.log(upstreamUpgradeOpts, "Checking and downloading new versions")
	}

	// Perform the upstream upgrade to fetch new versions
	if m.kotsCLI != nil {
		if err := m.kotsCLI.UpstreamUpgrade(upstreamUpgradeOpts); err != nil {
			return fmt.Errorf("kots cli upgrade: %w", err)
		}
	} else {
		if err := kotscli.UpstreamUpgrade(upstreamUpgradeOpts); err != nil {
			return fmt.Errorf("kots cli upgrade: %w", err)
		}
	}

	// List versions after upgrade to find the target sequence
	getVersionsOpts := kotscli.GetVersionsOptions{
		AppSlug:               appSlug,
		Namespace:             constants.KotsadmNamespace,
		ClusterID:             m.clusterID,
		ReplicatedAppEndpoint: netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain),
	}

	m.log(getVersionsOpts, "Listing versions to find target sequence")

	var versions []kotscli.AppVersionResponse
	var err error
	if m.kotsCLI != nil {
		versions, err = m.kotsCLI.GetVersions(getVersionsOpts)
	} else {
		versions, err = kotscli.GetVersions(getVersionsOpts)
	}
	if err != nil {
		return fmt.Errorf("get application versions: %w", err)
	}

	// Find the kots local sequence of the target version matching channel ID and channel sequence
	// so that we can use it to edit the config
	targetSequence := int64(-1)
	for _, v := range versions {
		if v.ChannelID == channelID && v.ChannelSequence == channelSequence {
			m.log(nil, "Found desired version %s with sequence %d", v.VersionLabel, v.Sequence)
			targetSequence = v.Sequence
			break
		}
	}
	if targetSequence == -1 {
		return fmt.Errorf("could not find target version %s with channel ID %s and channel sequence %d", versionLabel, channelID, channelSequence)
	}

	// Update the config with the new values based off of the target sequence
	setConfigOpts := kotscli.SetConfigOptions{
		AppSlug:               appSlug,
		Namespace:             constants.KotsadmNamespace,
		ClusterID:             m.clusterID,
		Sequence:              targetSequence,
		Deploy:                true,
		ReplicatedAppEndpoint: netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain),
		Stdout:                m.newLogWriter(),
	}

	configValuesFile, err := m.createConfigValuesFile(configValues)
	if err != nil {
		return fmt.Errorf("creating config values file: %w", err)
	}
	setConfigOpts.ConfigValuesFile = configValuesFile

	m.log(setConfigOpts, "Updating config with new values and deploying")

	if m.kotsCLI != nil {
		if err := m.kotsCLI.SetConfig(setConfigOpts); err != nil {
			return fmt.Errorf("kots cli set config: %w", err)
		}
	} else {
		if err := kotscli.SetConfig(setConfigOpts); err != nil {
			return fmt.Errorf("kots cli set config: %w", err)
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
