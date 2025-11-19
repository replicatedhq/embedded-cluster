package upgrade

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *UpgradeController) UpgradeInfra(ctx context.Context) (finalErr error) {
	logger := c.logger.WithField("operation", "upgrade-infra")

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

	err = c.stateMachine.Transition(lock, states.StateInfrastructureUpgrading, nil)
	if err != nil {
		return types.NewConflictError(err)
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
				logger.Error(finalErr)

				if err := c.stateMachine.Transition(lock, states.StateInfrastructureUpgradeFailed, finalErr); err != nil {
					logger.WithError(err).Error("failed to transition states")
				}

				if err := c.setInfraStatus(types.StateFailed, finalErr.Error()); err != nil {
					logger.WithError(err).Error("failed to set status to failed")
				}
			}
		}()

		if err := c.setInfraStatus(types.StateRunning, "Upgrading infrastructure"); err != nil {
			return fmt.Errorf("set status to running: %w", err)
		}

		// Get registry settings for airgap upgrades
		registrySettings, err := c.GetRegistrySettings(ctx, c.rc)
		if err != nil {
			return fmt.Errorf("get registry settings: %w", err)
		}

		if err := c.infraManager.Upgrade(ctx, c.rc, registrySettings); err != nil {
			return fmt.Errorf("upgrade infrastructure: %w", err)
		}

		if err := c.stateMachine.Transition(lock, states.StateInfrastructureUpgraded, nil); err != nil {
			return fmt.Errorf("transition states: %w", err)
		}

		if err := c.setInfraStatus(types.StateSucceeded, "Upgrade complete"); err != nil {
			return fmt.Errorf("set status to succeeded: %w", err)
		}

		return nil
	}()

	return nil
}

func (c *UpgradeController) GetInfra(ctx context.Context) (types.Infra, error) {
	return c.store.LinuxInfraStore().Get()
}

func (c *UpgradeController) setInfraStatus(state types.State, description string) error {
	return c.store.LinuxInfraStore().SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
