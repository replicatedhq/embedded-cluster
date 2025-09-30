package app

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

// UpgradeApp triggers app upgrade with proper state transitions and panic handling
func (c *AppController) UpgradeApp(ctx context.Context, ignoreAppPreflights bool) (finalErr error) {
	lock, err := c.stateMachine.AcquireLock()
	if err != nil {
		return types.NewConflictError(err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			lock.Release()
		}
	}()

	if err := c.stateMachine.ValidateTransition(lock, states.StateAppUpgrading); err != nil {
		return types.NewConflictError(err)
	}

	// Get config values for app upgrade
	configValues, err := c.appConfigManager.GetKotsadmConfigValues()
	if err != nil {
		return fmt.Errorf("get kotsadm config values for app upgrade: %w", err)
	}

	err = c.stateMachine.Transition(lock, states.StateAppUpgrading)
	if err != nil {
		return fmt.Errorf("transition states: %w", err)
	}

	go func() (finalErr error) {
		// Background context is used to avoid canceling the operation if the context is canceled
		ctx := context.Background()

		defer lock.Release()

		defer func() {
			if r := recover(); r != nil {
				finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
			}
			if finalErr != nil {
				c.logger.Error(finalErr)

				if err := c.stateMachine.Transition(lock, states.StateAppUpgradeFailed); err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}
			} else {
				if err := c.stateMachine.Transition(lock, states.StateSucceeded); err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}
			}
		}()

		// Upgrade the app
		err := c.appUpgradeManager.Upgrade(ctx, configValues)
		if err != nil {
			return fmt.Errorf("upgrade app: %w", err)
		}

		return nil
	}()

	return nil
}

func (c *AppController) GetAppUpgradeStatus(ctx context.Context) (types.AppUpgrade, error) {
	return c.appUpgradeManager.GetStatus()
}
