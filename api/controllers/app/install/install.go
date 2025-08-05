package install

import (
	"context"
	"fmt"
	"runtime/debug"

	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

// InstallApp triggers app installation with proper state transitions and panic handling
func (c *InstallController) InstallApp(ctx context.Context) (finalErr error) {
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

		defer func() {
			if r := recover(); r != nil {
				finalErr = fmt.Errorf("panic installing app: %v: %s", r, string(debug.Stack()))
			}
			// Handle errors from app installation
			if finalErr != nil {
				c.logger.Error(finalErr)

				if err := c.stateMachine.Transition(lock, states.StateAppInstallFailed); err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}
				return
			}

			// Transition to succeeded state on successful app installation
			if err := c.stateMachine.Transition(lock, states.StateSucceeded); err != nil {
				c.logger.Errorf("failed to transition states: %w", err)
			}
		}()

		// Install the app
		err := c.appInstallManager.Install(ctx, configValues)
		if err != nil {
			return fmt.Errorf("install app: %w", err)
		}

		return nil
	}()

	return nil
}

// TODO: remove this once we have endpoints to trigger app installation and report status
// and the app installation is decoupled from the infra installation
func (c *InstallController) InstallAppNoState(ctx context.Context) error {
	// Get config values for app installation
	configValues, err := c.appConfigManager.GetKotsadmConfigValues()
	if err != nil {
		return fmt.Errorf("get kotsadm config values for app install: %w", err)
	}

	// Install the app using the app install manager
	if err := c.appInstallManager.Install(ctx, configValues); err != nil {
		return fmt.Errorf("install app: %w", err)
	}

	return nil
}

func (c *InstallController) GetAppInstallStatus(ctx context.Context) (types.AppInstall, error) {
	return c.appInstallManager.GetStatus()
}
