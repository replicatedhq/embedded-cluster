package install

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
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

	if err := c.installationManager.SetConfig(*config); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	if err := c.installationManager.ConfigureHost(ctx); err != nil {
		return fmt.Errorf("configure host: %w", err)
	}

	return nil
}

func (c *InstallController) GetInstallationStatus(ctx context.Context) (*types.Status, error) {
	return c.installationManager.GetStatus()
}
