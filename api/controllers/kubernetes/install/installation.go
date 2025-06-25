package install

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
)

func (c *InstallController) GetInstallationConfig(ctx context.Context) (types.KubernetesInstallationConfig, error) {
	config, err := c.installationManager.GetConfig()
	if err != nil {
		return types.KubernetesInstallationConfig{}, err
	}

	if err := c.installationManager.SetConfigDefaults(&config); err != nil {
		return types.KubernetesInstallationConfig{}, fmt.Errorf("set defaults: %w", err)
	}

	if err := c.installationManager.ValidateConfig(config, c.ki.ManagerPort()); err != nil {
		return types.KubernetesInstallationConfig{}, fmt.Errorf("validate: %w", err)
	}

	return config, nil
}

func (c *InstallController) ConfigureInstallation(ctx context.Context, config types.KubernetesInstallationConfig) error {
	err := c.configureInstallation(ctx, config)
	if err != nil {
		return err
	}

	// TODO NOW: handle status updates

	return nil
}

func (c *InstallController) configureInstallation(ctx context.Context, config types.KubernetesInstallationConfig) error {
	lock, err := c.stateMachine.AcquireLock()
	if err != nil {
		return types.NewConflictError(err)
	}
	defer lock.Release()

	if err := c.stateMachine.ValidateTransition(lock, StateInstallationConfigured); err != nil {
		return types.NewConflictError(err)
	}

	if err := c.installationManager.ValidateConfig(config, c.ki.ManagerPort()); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	if err := c.installationManager.SetConfig(config); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	proxy, err := newconfig.GetKubernetesProxySpec(config.HTTPProxy, config.HTTPSProxy, config.NoProxy)
	if err != nil {
		return fmt.Errorf("get proxy spec: %w", err)
	}

	// update the kubernetes installation
	c.ki.SetAdminConsolePort(config.AdminConsolePort)
	c.ki.SetProxySpec(proxy)

	err = c.stateMachine.Transition(lock, StateInstallationConfigured)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	return nil
}

func (c *InstallController) GetInstallationStatus(ctx context.Context) (types.Status, error) {
	return c.installationManager.GetStatus()
}
