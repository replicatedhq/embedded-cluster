package install

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
)

func (c *InstallController) GetInstallationConfig(ctx context.Context) (*types.InstallationConfig, error) {
	config, err := c.installationManager.GetConfig()
	if err != nil {
		return nil, err
	}

	if config == nil {
		return nil, fmt.Errorf("installation config is nil")
	}

	if err := c.installationManager.SetConfigDefaults(config); err != nil {
		return nil, fmt.Errorf("set defaults: %w", err)
	}

	if err := c.installationManager.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	return config, nil
}

func (c *InstallController) ConfigureInstallation(ctx context.Context, config *types.InstallationConfig) error {
	if err := c.installationManager.ValidateConfig(config); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	if err := c.computeCIDRs(config); err != nil {
		return fmt.Errorf("compute cidrs: %w", err)
	}

	if err := c.installationManager.SetConfig(*config); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	// TODO (@team): discuss the distinction between the runtime config and the installation config
	// update the runtime config
	c.rc.SetDataDir(config.DataDirectory)
	c.rc.SetLocalArtifactMirrorPort(config.LocalArtifactMirrorPort)
	c.rc.SetAdminConsolePort(config.AdminConsolePort)

	if err := c.installationManager.ConfigureHost(ctx, config); err != nil {
		return fmt.Errorf("configure: %w", err)
	}

	return nil
}

func (c *InstallController) computeCIDRs(config *types.InstallationConfig) error {
	if config.GlobalCIDR != "" {
		podCIDR, serviceCIDR, err := netutils.SplitNetworkCIDR(config.GlobalCIDR)
		if err != nil {
			return fmt.Errorf("split network cidr: %w", err)
		}
		config.PodCIDR = podCIDR
		config.ServiceCIDR = serviceCIDR
	}

	return nil
}

func (c *InstallController) GetInstallationStatus(ctx context.Context) (*types.Status, error) {
	return c.installationManager.GetStatus()
}
