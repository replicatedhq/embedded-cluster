package install

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

var (
	ErrPreflightChecksFailed = errors.New("preflight checks failed")
)

func (c *InstallController) SetupInfra(ctx context.Context, ignoreHostPreflights bool) (finalErr error) {
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

	// Check if preflights have failed and if we should ignore them
	if c.stateMachine.CurrentState() == states.StateHostPreflightsFailed {
		if !ignoreHostPreflights || !c.allowIgnoreHostPreflights {
			return types.NewBadRequestError(ErrPreflightChecksFailed)
		}
		err = c.stateMachine.Transition(lock, states.StateHostPreflightsFailedBypassed)
		if err != nil {
			return fmt.Errorf("failed to transition states: %w", err)
		}
	}

	err = c.stateMachine.Transition(lock, states.StateInfrastructureInstalling)
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

				if err := c.stateMachine.Transition(lock, states.StateInfrastructureInstallFailed); err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}

				if err := c.setInfraStatus(types.StateFailed, finalErr.Error()); err != nil {
					c.logger.WithError(err).Error("failed to set status to failed")
				}
			}
		}()

		if err := c.setInfraStatus(types.StateRunning, "Installing infrastructure"); err != nil {
			return fmt.Errorf("set status to running: %w", err)
		}

		if err := c.infraManager.Install(ctx, c.rc); err != nil {
			return fmt.Errorf("failed to install infrastructure: %w", err)
		}

		if err := c.stateMachine.Transition(lock, states.StateInfrastructureInstalled); err != nil {
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
	return c.store.LinuxInfraStore().Get()
}

func (c *InstallController) setInfraStatus(state types.State, description string) error {
	return c.store.LinuxInfraStore().SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
