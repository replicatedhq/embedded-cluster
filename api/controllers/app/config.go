package app

import (
	"context"
	"fmt"
	"runtime/debug"

	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *AppController) PatchAppConfigValues(ctx context.Context, values types.AppConfigValues) (finalErr error) {
	lock, err := c.stateMachine.AcquireLock()
	if err != nil {
		return types.NewConflictError(err)
	}
	defer lock.Release()

	err = c.stateMachine.ValidateTransition(lock, states.StateApplicationConfiguring, states.StateApplicationConfigured)
	if err != nil {
		return types.NewConflictError(err)
	}

	err = c.stateMachine.Transition(lock, states.StateApplicationConfiguring)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}

		if finalErr != nil {
			if err := c.stateMachine.Transition(lock, states.StateApplicationConfigurationFailed); err != nil {
				c.logger.Errorf("failed to transition states: %w", err)
			}
		}
	}()

	err = c.appConfigManager.ValidateConfigValues(values)
	if err != nil {
		return fmt.Errorf("validate app config values: %w", err)
	}

	err = c.appConfigManager.PatchConfigValues(values)
	if err != nil {
		return fmt.Errorf("patch app config values: %w", err)
	}

	err = c.stateMachine.Transition(lock, states.StateApplicationConfigured)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	return nil
}

func (c *AppController) TemplateAppConfig(ctx context.Context, values types.AppConfigValues, maskPasswords bool) (types.AppConfig, error) {
	return c.appConfigManager.TemplateConfig(values, maskPasswords, true)
}

func (c *AppController) GetAppConfigValues(ctx context.Context) (types.AppConfigValues, error) {
	return c.appConfigManager.GetConfigValues()
}
