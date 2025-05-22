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
	configStore installation.ConfigStore
}

type InstallControllerOption func(*InstallController)

func WithConfigStore(configStore installation.ConfigStore) InstallControllerOption {
	return func(c *InstallController) {
		c.configStore = configStore
	}
}

func NewInstallController(opts ...InstallControllerOption) (*InstallController, error) {
	controller := &InstallController{}

	for _, opt := range opts {
		opt(controller)
	}

	if controller.configStore == nil {
		controller.configStore = installation.NewConfigMemoryStore()
	}

	return controller, nil
}

func (c *InstallController) Get(ctx context.Context) (*types.Install, error) {
	config, err := c.configStore.Read()
	if err != nil {
		return nil, err
	}

	err = installation.ConfigSetDefaults(config)
	if err != nil {
		return nil, fmt.Errorf("set defaults: %w", err)
	}

	err = installation.ConfigValidate(config)
	if err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	install := &types.Install{
		Config: *config,
	}

	return install, nil
}

func (c *InstallController) SetConfig(ctx context.Context, config *types.InstallationConfig) error {
	err := installation.ConfigValidate(config)
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	err = c.configStore.Write(*config)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}
