package install

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/pkg/installation"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

type Controller interface {
	Get(ctx context.Context) (*types.Install, error)
	SetConfig(ctx context.Context, config *types.InstallationConfig) error
}

var _ Controller = &InstallController{}

type InstallController struct {
	configManager installation.ConfigManager
}

type InstallControllerOption func(*InstallController)

func WithConfigManager(configManager installation.ConfigManager) InstallControllerOption {
	return func(c *InstallController) {
		c.configManager = configManager
	}
}

func NewInstallController(opts ...InstallControllerOption) (*InstallController, error) {
	controller := &InstallController{}

	for _, opt := range opts {
		opt(controller)
	}

	if controller.configManager == nil {
		controller.configManager = installation.NewConfigManager(
			installation.WithConfigStore(installation.NewConfigMemoryStore()),
		)
	}

	return controller, nil
}

func (c *InstallController) Get(ctx context.Context) (*types.Install, error) {
	config, err := c.configManager.Read()
	if err != nil {
		return nil, err
	}

	err = c.configManager.SetDefaults(config)
	if err != nil {
		return nil, fmt.Errorf("set defaults: %w", err)
	}

	err = c.configManager.Validate(config)
	if err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	install := &types.Install{
		Config: *config,
	}

	return install, nil
}

func (c *InstallController) SetConfig(ctx context.Context, config *types.InstallationConfig) error {
	err := c.configManager.Validate(config)
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	err = c.configManager.Write(*config)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}
