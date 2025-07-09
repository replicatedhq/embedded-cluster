package install

import (
	"context"
	"errors"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

func (c *InstallController) GetAppConfig(ctx context.Context) (kotsv1beta1.Config, error) {
	if c.releaseData == nil || c.releaseData.AppConfig == nil {
		return kotsv1beta1.Config{}, errors.New("app config not found")
	}

	values, err := c.appConfigManager.GetConfigValues()
	if err != nil {
		return kotsv1beta1.Config{}, fmt.Errorf("get app config values: %w", err)
	}

	appConfig, err := c.appConfigManager.ApplyValuesToConfig(*c.releaseData.AppConfig, values)
	if err != nil {
		return kotsv1beta1.Config{}, fmt.Errorf("apply values to config: %w", err)
	}

	return appConfig, nil
}

func (c *InstallController) SetAppConfigValues(ctx context.Context, values map[string]string) (finalErr error) {
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
		if finalErr != nil {
			if err := c.stateMachine.Transition(lock, StateApplicationConfigurationFailed); err != nil {
				c.logger.Errorf("failed to transition states: %w", err)
			}
		}
	}()

	err = c.appConfigManager.SetConfigValues(ctx, values)
	if err != nil {
		return fmt.Errorf("set app config values: %w", err)
	}

	err = c.stateMachine.Transition(lock, StateApplicationConfigured)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	return nil
}
