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
	if c.releaseData == nil || c.releaseData.AppConfig == nil {
		return kotsv1beta1.Config{}, errors.New("app config not found")
	}

	return c.appConfigManager.GetConfig(*c.releaseData.AppConfig)
}

func (c *InstallController) SetAppConfigValues(ctx context.Context, values map[string]string) (finalErr error) {
	if c.releaseData == nil || c.releaseData.AppConfig == nil {
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

	err = c.appConfigManager.SetConfigValues(*c.releaseData.AppConfig, values)
	if err != nil {
		return fmt.Errorf("set app config values: %w", err)
	}

	err = c.stateMachine.Transition(lock, StateApplicationConfigured)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	return nil
}

func (c *InstallController) GetAppConfigValues(ctx context.Context) (map[string]string, error) {
	return c.appConfigManager.GetConfigValues()
}
