package install

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/internal/states"
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

	err = c.stateMachine.Transition(lock, states.StateInstallationConfiguring, nil)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			c.logger.Error(finalErr)

			if err := c.stateMachine.Transition(lock, states.StateInstallationConfigurationFailed, finalErr); err != nil {
				c.logger.WithError(err).Error("failed to transition states")
			}

			if err = c.setInstallationStatus(types.StateFailed, finalErr.Error()); err != nil {
				c.logger.WithError(err).Error("failed to set status to failed")
			}
		}
	}()

	if err := c.setInstallationStatus(types.StateRunning, "Configuring installation"); err != nil {
		return fmt.Errorf("set status to running: %w", err)
	}

	if err := c.installationManager.ConfigureInstallation(ctx, c.ki, config); err != nil {
		return fmt.Errorf("configure installation: %w", err)
	}

	if err := c.stateMachine.Transition(lock, states.StateInstallationConfigured, nil); err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	if err := c.setInstallationStatus(types.StateSucceeded, "Installation configured"); err != nil {
		return fmt.Errorf("set status to succeeded: %w", err)
	}

	return nil
}

func (c *InstallController) GetInstallationStatus(ctx context.Context) (types.Status, error) {
	return c.store.KubernetesInstallationStore().GetStatus()
}

func (c *InstallController) setInstallationStatus(state types.State, description string) error {
	return c.store.KubernetesInstallationStore().SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
