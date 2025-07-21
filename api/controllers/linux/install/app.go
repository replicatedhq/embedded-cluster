package install

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *InstallController) GetAppConfig(ctx context.Context) (types.AppConfig, error) {
	return c.appConfigManager.GetConfig(c.rc)
}

func (c *InstallController) PatchAppConfigValues(ctx context.Context, values types.AppConfigValues) (finalErr error) {
	lock, err := c.stateMachine.AcquireLock()
	if err != nil {
		return types.NewConflictError(err)
	}
	defer lock.Release()

	err = c.stateMachine.ValidateTransition(lock, StateApplicationConfiguring, StateApplicationConfigured)
	if err != nil {
		return types.NewConflictError(err)
	}

	err = c.stateMachine.Transition(lock, StateApplicationConfiguring)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}

		if finalErr != nil {
			if err := c.stateMachine.Transition(lock, StateApplicationConfigurationFailed); err != nil {
				c.logger.Errorf("failed to transition states: %w", err)
			}
		}
	}()

	err = c.appConfigManager.ValidateConfigValues(values, c.rc)
	if err != nil {
		return fmt.Errorf("validate app config values: %w", err)
	}

	err = c.appConfigManager.PatchConfigValues(values, c.rc)
	if err != nil {
		return fmt.Errorf("patch app config values: %w", err)
	}

	err = c.stateMachine.Transition(lock, StateApplicationConfigured)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	return nil
}

func (c *InstallController) GetAppConfigValues(ctx context.Context, maskPasswords bool) (types.AppConfigValues, error) {
	return c.appConfigManager.GetConfigValues(maskPasswords, c.rc)
}

func (c *InstallController) TemplateAppConfig(ctx context.Context, values types.AppConfigValues) (types.AppConfig, error) {
	return c.appConfigManager.TemplateConfig(values, c.rc)
}
