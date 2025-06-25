package install

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

var (
	ErrPreflightChecksFailed      = errors.New("preflight checks failed")
	ErrPreflightChecksNotComplete = errors.New("preflight checks not complete")
)

func (c *InstallController) SetupInfra(ctx context.Context, ignoreHostPreflights bool) (finalErr error) {
	if c.stateMachine.CurrentState() == StatePreflightsFailed {
		err := c.bypassPreflights(ctx, ignoreHostPreflights)
		if err != nil {
			return fmt.Errorf("bypass preflights: %w", err)
		}
	}

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

	err = c.stateMachine.Transition(lock, StateInfrastructureInstalling)
	if err != nil {
		return types.NewConflictError(err)
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

				if err := c.stateMachine.Transition(lock, StateInfrastructureInstallFailed); err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}
			} else {
				if err := c.stateMachine.Transition(lock, StateSucceeded); err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}
			}
		}()

		if err := c.infraManager.Install(ctx, c.rc); err != nil {
			return fmt.Errorf("failed to install infrastructure: %w", err)
		}

		return nil
	}()

	return nil
}

func (c *InstallController) bypassPreflights(ctx context.Context, ignoreHostPreflights bool) error {
	if !ignoreHostPreflights || !c.allowIgnoreHostPreflights {
		return types.NewBadRequestError(ErrPreflightChecksFailed)
	}

	lock, err := c.stateMachine.AcquireLock()
	if err != nil {
		return types.NewConflictError(err)
	}
	defer lock.Release()

	if err := c.stateMachine.ValidateTransition(lock, StatePreflightsFailedBypassed); err != nil {
		return types.NewConflictError(err)
	}

	// TODO (@ethan): we have already sent the preflight output when we sent the failed event.
	// We should evaluate if we should send it again.
	preflightOutput, err := c.GetHostPreflightOutput(ctx)
	if err != nil {
		return fmt.Errorf("get install host preflight output: %w", err)
	}

	// Report that preflights were bypassed
	if preflightOutput != nil {
		c.metricsReporter.ReportPreflightsBypassed(ctx, preflightOutput)
	}

	err = c.stateMachine.Transition(lock, StatePreflightsFailedBypassed)
	if err != nil {
		return types.NewConflictError(err)
	}

	return nil
}

func (c *InstallController) GetInfra(ctx context.Context) (types.Infra, error) {
	return c.infraManager.Get()
}
