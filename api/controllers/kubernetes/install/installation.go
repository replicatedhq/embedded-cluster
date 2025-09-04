package install

import (
	"context"
	"fmt"
	"runtime/debug"

	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *InstallController) GetInstallationConfig(ctx context.Context) (types.KubernetesInstallationConfigResponse, error) {
	// Get stored config (user values only)
	values, err := c.installationManager.GetConfigValues()
	if err != nil {
		return types.KubernetesInstallationConfigResponse{}, fmt.Errorf("get config values: %w", err)
	}

	// Get defaults separately
	defaults, err := c.installationManager.GetDefaults()
	if err != nil {
		return types.KubernetesInstallationConfigResponse{}, fmt.Errorf("get defaults: %w", err)
	}

	// Get the final "resolved" config with the user values and defaults applied
	config, err := c.installationManager.GetConfig()
	if err != nil {
		return types.KubernetesInstallationConfigResponse{}, fmt.Errorf("get config: %w", err)
	}

	return types.KubernetesInstallationConfigResponse{
		Values:   values,
		Defaults: defaults,
		Resolved: config,
	}, nil
}

func (c *InstallController) ConfigureInstallation(ctx context.Context, config types.KubernetesInstallationConfig) (finalErr error) {
	lock, err := c.stateMachine.AcquireLock()
	if err != nil {
		return types.NewConflictError(err)
	}
	defer lock.Release()

	if err := c.stateMachine.ValidateTransition(lock, states.StateInstallationConfiguring, states.StateInstallationConfigured); err != nil {
		return types.NewConflictError(err)
	}

	err = c.stateMachine.Transition(lock, states.StateInstallationConfiguring)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			c.logger.Error(finalErr)

			if err := c.stateMachine.Transition(lock, states.StateInstallationConfigurationFailed); err != nil {
				c.logger.Errorf("failed to transition states: %w", err)
			}
		} else {
			if err := c.stateMachine.Transition(lock, states.StateInstallationConfigured); err != nil {
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
