package app

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

var (
	ErrAppPreflightChecksFailed = errors.New("app preflight checks failed")
)

// InstallApp triggers app installation with proper state transitions and panic handling
func (c *AppController) InstallApp(ctx context.Context, ignoreAppPreflights bool) (finalErr error) {
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

	// Check if app preflights have failed and if we should ignore them
	if c.stateMachine.CurrentState() == states.StateAppPreflightsFailed {
		// Immediately block installation if there are strict app preflight failures (cannot be bypassed)
		preflightOutput, err := c.appPreflightManager.GetAppPreflightOutput(ctx)
		if err != nil {
			return fmt.Errorf("failed to get app preflight output: %w", err)
		}
		if preflightOutput != nil && preflightOutput.HasStrictFailures() {
			return types.NewBadRequestError(errors.New("installation blocked: strict app preflight checks failed"))
		}

		allowIgnoreAppPreflights := true // TODO: implement if we decide to support a ignore-app-preflights CLI flag for V3
		if !ignoreAppPreflights || !allowIgnoreAppPreflights {
			return types.NewBadRequestError(ErrAppPreflightChecksFailed)
		}

		err = c.stateMachine.Transition(lock, states.StateAppPreflightsFailedBypassed)
		if err != nil {
			return fmt.Errorf("failed to transition states: %w", err)
		}
	}

	if err := c.stateMachine.ValidateTransition(lock, states.StateAppInstalling); err != nil {
		return types.NewConflictError(err)
	}

	// Get config values for app installation
	configValues, err := c.appConfigManager.GetKotsadmConfigValues()
	if err != nil {
		return fmt.Errorf("get kotsadm config values for app install: %w", err)
	}

	err = c.stateMachine.Transition(lock, states.StateAppInstalling)
	if err != nil {
		return fmt.Errorf("transition states: %w", err)
	}

	go func() (finalErr error) {
		// Background context is used to avoid canceling the operation if the context is canceled
		ctx := context.Background()

		defer lock.Release()

		prehookFn := func(status types.Status) error {
			switch status.State {
			case types.StateFailed:
				if err := c.stateMachine.Transition(lock, states.StateAppInstallFailed); err != nil {
					return fmt.Errorf("failed to transition states: %w", err)
				}
			case types.StateSucceeded:
				if err := c.stateMachine.Transition(lock, states.StateSucceeded); err != nil {
					return fmt.Errorf("failed to transition states: %w", err)
				}
			}
			return nil
		}

		// Install the app
		err := c.appInstallManager.Install(ctx, configValues, prehookFn)
		if err != nil {
			return fmt.Errorf("install app: %w", err)
		}

		return nil
	}()

	return nil
}

func (c *AppController) GetAppInstallStatus(ctx context.Context) (types.AppInstall, error) {
	return c.appInstallManager.GetStatus()
}
