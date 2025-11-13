package install

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *InstallController) SetupInfra(ctx context.Context) (finalErr error) {
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

	err = c.stateMachine.Transition(lock, states.StateInfrastructureInstalling, nil)
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
				c.logger.Error(finalErr)

				if err := c.stateMachine.Transition(lock, states.StateInfrastructureInstallFailed, finalErr); err != nil {
					c.logger.WithError(err).Error("failed to transition states")
				}

				if err := c.setInfraStatus(types.StateFailed, finalErr.Error()); err != nil {
					c.logger.WithError(err).Error("failed to set status to failed")
				}
			}
		}()

		if err := c.setInfraStatus(types.StateRunning, "Installing infrastructure"); err != nil {
			return fmt.Errorf("set status to running: %w", err)
		}

		if err := c.infraManager.Install(ctx, c.ki); err != nil {
			return fmt.Errorf("failed to install infrastructure: %w", err)
		}

		if err := c.stateMachine.Transition(lock, states.StateInfrastructureInstalled, nil); err != nil {
			return fmt.Errorf("transition states: %w", err)
		}

		if err := c.setInfraStatus(types.StateSucceeded, "Installation complete"); err != nil {
			return fmt.Errorf("set status to succeeded: %w", err)
		}

		return nil
	}()

	return nil
}

func (c *InstallController) GetInfra(ctx context.Context) (types.Infra, error) {
	return c.store.KubernetesInfraStore().Get()
}

func (c *InstallController) setInfraStatus(state types.State, description string) error {
	return c.store.KubernetesInfraStore().SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
