package install

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

func (c *InstallController) GetAppConfig(ctx context.Context) (kotsv1beta1.Config, error) {
	return c.appConfigManager.GetConfig()
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

	err = c.appConfigManager.ValidateConfigValues(values)
	if err != nil {
		return fmt.Errorf("validate app config values: %w", err)
	}

	err = c.appConfigManager.PatchConfigValues(values)
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
	return c.appConfigManager.GetConfigValues(maskPasswords)
}
