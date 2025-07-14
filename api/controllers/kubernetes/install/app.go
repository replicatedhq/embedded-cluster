package install

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

func (c *InstallController) GetAppConfig(ctx context.Context) (kotsv1beta1.Config, error) {
	if c.appConfigManager == nil {
		return kotsv1beta1.Config{}, errors.New("app config not found")
	}

	return c.appConfigManager.GetConfig()
}

func (c *InstallController) PatchAppConfigValues(ctx context.Context, values map[string]string) (finalErr error) {
	if c.appConfigManager == nil {
		return errors.New("app config not found")
	}

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

func (c *InstallController) GetAppConfigValues(ctx context.Context, maskPasswords bool) (map[string]string, error) {
	if c.appConfigManager == nil {
		return nil, errors.New("app config not found")
	}
	return c.appConfigManager.GetConfigValues(maskPasswords)
}
