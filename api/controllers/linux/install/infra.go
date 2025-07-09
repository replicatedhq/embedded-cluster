package install

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

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
	if c.stateMachine.CurrentState() == StatePreflightsFailed {
		if !ignoreHostPreflights || !c.allowIgnoreHostPreflights {
			return types.NewBadRequestError(ErrPreflightChecksFailed)
		}
		err = c.stateMachine.Transition(lock, StatePreflightsFailedBypassed)
		if err != nil {
			return fmt.Errorf("failed to transition states: %w", err)
		}
	}

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

		if err := c.updateInfraManagerWithLatestConfigValues(); err != nil {
			return fmt.Errorf("updating infra manager with latest config values: %w", err)
		}

		if err := c.infraManager.Install(ctx, c.rc); err != nil {
			return fmt.Errorf("failed to install infrastructure: %w", err)
		}

		return nil
	}()

	return nil
}

// updateInfraManagerWithLatestConfigValues updates the infraManager with the latest config values from the memory store.
// This ensures that any config values set via SetAppConfigValues are properly passed to the infra manager.
func (c *InstallController) updateInfraManagerWithLatestConfigValues() error {
	// Get the latest config values directly from the store
	memoryStoreConfigValues := make(map[string]string)
	if c.store != nil && c.store.AppConfigStore() != nil {
		if configValues, err := c.store.AppConfigStore().GetConfigValues(); err != nil {
			c.logger.WithError(err).Warn("reading config values from store")
		} else {
			memoryStoreConfigValues = configValues
		}
	}

	// Update the existing infraManager with the latest config values
	c.infraManager.UpdateConfigValues(memoryStoreConfigValues)

	return nil
}

func (c *InstallController) GetInfra(ctx context.Context) (types.Infra, error) {
	return c.infraManager.Get()
}
