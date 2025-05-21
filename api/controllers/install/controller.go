package install

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/models"
)

type Controller interface {
	Get(ctx context.Context) (*models.Install, error)
	SetConfig(ctx context.Context, config models.InstallationConfig) error
	StartInstall(ctx context.Context) error
}

var _ Controller = &InstallController{}

type InstallController struct {
	installationConfigStore models.InstallationConfigStore
}

func NewInstallController() (*InstallController, error) {
	installationConfigStore, err := models.NewInstallationConfigRuntimeConfigStore()
	if err != nil {
		return nil, fmt.Errorf("new installation config store: %w", err)
	}

	return &InstallController{
		installationConfigStore: installationConfigStore,
	}, nil
}

func (c *InstallController) Get(ctx context.Context) (*models.Install, error) {
	config, err := c.installationConfigStore.Read()
	if err != nil {
		return nil, err
	}

	err = config.SetDefaults()
	if err != nil {
		return nil, fmt.Errorf("set defaults: %w", err)
	}

	err = config.Validate()
	if err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	install := &models.Install{
		Config: *config,
	}

	return install, nil
}

func (c *InstallController) SetConfig(ctx context.Context, config models.InstallationConfig) error {
	err := config.SetDefaults()
	if err != nil {
		return fmt.Errorf("set defaults: %w", err)
	}

	err = config.Validate()
	if err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	err = c.installationConfigStore.Write(config)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func (c *InstallController) StartInstall(ctx context.Context) error {
	// TODO
	return nil
}
