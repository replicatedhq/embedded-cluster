package install

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/pkg/installation"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
)

type Controller interface {
	Get(ctx context.Context) (*types.Install, error)
	SetConfig(ctx context.Context, config *types.InstallationConfig) error
	SetStatus(ctx context.Context, status *types.InstallationStatus) error
	ReadStatus(ctx context.Context) (*types.InstallationStatus, error)
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

	if err := c.installationManager.SetDefaults(config); err != nil {
		return nil, fmt.Errorf("set defaults: %w", err)
	}

	if err := c.installationManager.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	status, err := c.installationManager.ReadStatus()
	if err != nil {
		return nil, fmt.Errorf("read status: %w", err)
	}

	install := &types.Install{
		Config: *config,
		Status: *status,
	}

	return install, nil
}

func (c *InstallController) SetConfig(ctx context.Context, config *types.InstallationConfig) error {
	if err := c.installationManager.ValidateConfig(config); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	if err := c.computeCIDRs(config); err != nil {
		return fmt.Errorf("compute cidrs: %w", err)
	}

	if err := c.installationManager.WriteConfig(*config); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func (c *InstallController) SetStatus(ctx context.Context, status *types.InstallationStatus) error {
	if err := c.installationManager.WriteStatus(*status); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}

func (c *InstallController) ReadStatus(ctx context.Context) (*types.InstallationStatus, error) {
	return c.installationManager.ReadStatus()
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
