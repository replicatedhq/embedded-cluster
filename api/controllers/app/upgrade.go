package app

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

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
		defer lock.Release()

		// Background context is used to avoid canceling the operation if the context is canceled
		ctx := context.Background()

		defer func() {
			if r := recover(); r != nil {
				finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
			}
			if finalErr != nil {
				c.logger.Error(finalErr)

				if err := c.stateMachine.Transition(lock, states.StateAppUpgradeFailed); err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}

				if err := c.setAppUpgradeStatus(types.StateFailed, finalErr.Error()); err != nil {
					c.logger.WithError(err).Error("failed to set status to failed")
				}
			}
		}()

		if err := c.setAppUpgradeStatus(types.StateRunning, "Upgrading application"); err != nil {
			return fmt.Errorf("set status to running: %w", err)
		}

		// Upgrade the app
		err := c.appUpgradeManager.Upgrade(ctx, configValues)
		if err != nil {
			return fmt.Errorf("upgrade app: %w", err)
		}

		if err := c.stateMachine.Transition(lock, states.StateSucceeded); err != nil {
			return fmt.Errorf("transition states: %w", err)
		}

		if err := c.setAppUpgradeStatus(types.StateSucceeded, "Upgrade complete"); err != nil {
			return fmt.Errorf("set status to succeeded: %w", err)
		}

		return nil
	}()

	return nil
}

func (c *AppController) GetAppUpgradeStatus(ctx context.Context) (types.AppUpgrade, error) {
	return c.store.AppUpgradeStore().Get()
}

func (c *AppController) setAppUpgradeStatus(state types.State, description string) error {
	return c.store.AppUpgradeStore().SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
