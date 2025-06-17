package install

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *InstallController) SetupInfra(ctx context.Context) error {
	if c.stateMachine.CurrentState() == StatePreflightsFailed {
		err := c.bypassPreflights(ctx)
		if err != nil {
			return fmt.Errorf("bypass preflights: %w", err)
		}
	}

	lock, err := c.stateMachine.AcquireLock()
	if err != nil {
		return types.NewConflictError(err)
	}

	err = c.stateMachine.Transition(lock, StateInfrastructureInstalling)
	if err != nil {
		lock.Release()
		return types.NewConflictError(err)
	}

	go func() {
		defer lock.Release()

		defer func() {
			if r := recover(); r != nil {
				c.logger.Errorf("panic installing infrastructure: %v: %s", r, string(debug.Stack()))

				err := c.stateMachine.Transition(lock, StateFailed)
				if err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}
			}
		}()

		// Background context is used to avoid canceling the operation if the context is canceled
		err := c.infraManager.Install(context.Background(), c.rc)

		if err != nil {
			c.logger.Errorf("failed to install infrastructure: %w", err)

			err := c.stateMachine.Transition(lock, StateFailed)
			if err != nil {
				c.logger.Errorf("failed to transition states: %w", err)
			}
		} else {
			err = c.stateMachine.Transition(lock, StateSucceeded)
			if err != nil {
				c.logger.Errorf("failed to transition states: %w", err)
			}
		}
	}()

	return nil
}

func (c *InstallController) bypassPreflights(ctx context.Context) error {
	lock, err := c.stateMachine.AcquireLock()
	if err != nil {
		return types.NewConflictError(err)
	}
	defer lock.Release()

	// TODO (@ethan): this feels awkward
	preflightOutput, err := c.GetHostPreflightOutput(ctx)
	if err != nil {
		return fmt.Errorf("get install host preflight output: %w", err)
	}
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
