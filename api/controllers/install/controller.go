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
	SetStatus(ctx context.Context, status *types.InstallationStatus) error
}

var _ Controller = &InstallController{}

type InstallController struct {
	installationManager installation.InstallationManager
}

type InstallControllerOption func(*InstallController)

func WithInstallationManager(installationManager installation.InstallationManager) InstallControllerOption {
	return func(c *InstallController) {
		c.installationManager = installationManager
	}
}

func NewInstallController(opts ...InstallControllerOption) (*InstallController, error) {
	controller := &InstallController{}

	for _, opt := range opts {
		opt(controller)
	}

	if controller.installationManager == nil {
		controller.installationManager = installation.NewInstallationManager()
	}

	return controller, nil
}

func (c *InstallController) Get(ctx context.Context) (*types.Install, error) {
	config, err := c.installationManager.ReadConfig()
	if err != nil {
		return nil, err
	}

	err = c.installationManager.SetDefaults(config)
	if err != nil {
		return nil, fmt.Errorf("set defaults: %w", err)
	}

	err = c.installationManager.ValidateConfig(config)
	if err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	install := &types.Install{
		Config: *config,
	}

	return install, nil
}

func (c *InstallController) SetConfig(ctx context.Context, config *types.InstallationConfig) error {
	err := c.installationManager.ValidateConfig(config)
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	err = c.installationManager.WriteConfig(*config)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func (c *InstallController) SetStatus(ctx context.Context, status *types.InstallationStatus) error {
	err := c.installationManager.WriteStatus(*status)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}
