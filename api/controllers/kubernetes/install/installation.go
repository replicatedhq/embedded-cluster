package install

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/types"
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

func (c *InstallController) ConfigureInstallation(ctx context.Context, config types.KubernetesInstallationConfig) (finalErr error) {
	lock, err := c.stateMachine.AcquireLock()
	if err != nil {
		return types.NewConflictError(err)
	}
	defer lock.Release()

	if err := c.stateMachine.ValidateTransition(lock, StateInstallationConfigured); err != nil {
		return types.NewConflictError(err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			c.logger.Error(finalErr)

			if err := c.stateMachine.Transition(lock, StateInstallationConfigurationFailed); err != nil {
				c.logger.Errorf("failed to transition states: %w", err)
			}
		} else {
			if err := c.stateMachine.Transition(lock, StateInstallationConfigured); err != nil {
				c.logger.Errorf("failed to transition states: %w", err)
			}
		}
	}()

	if err := c.installationManager.ConfigureInstallation(ctx, c.ki, config); err != nil {
		return err
	}

	return nil
}

func (c *InstallController) GetInstallationStatus(ctx context.Context) (types.Status, error) {
	return c.installationManager.GetStatus()
}
