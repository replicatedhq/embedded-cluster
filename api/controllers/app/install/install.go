package install

import (
	"context"
	"fmt"
	"runtime/debug"

	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
)

// InstallApp triggers app installation with proper state transitions and panic handling
func (c *InstallController) InstallApp(ctx context.Context) (finalErr error) {
	lock, err := c.stateMachine.AcquireLock()
	if err != nil {
		return fmt.Errorf("acquire state machine lock for app install: %w", err)
	}

	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
		}
		if finalErr != nil {
			c.logger.Error(finalErr)
			if err := c.stateMachine.Transition(lock, states.StateAppInstallFailed); err != nil {
				c.logger.Errorf("failed to transition to app install failed state: %w", err)
			}
		} else {
			if err := c.stateMachine.Transition(lock, states.StateSucceeded); err != nil {
				c.logger.Errorf("failed to transition to succeeded state: %w", err)
			}
		}
		lock.Release()
	}()

	// Transition to app installing state
	if err := c.stateMachine.Transition(lock, states.StateAppInstalling); err != nil {
		return fmt.Errorf("transition to app installing state: %w", err)
	}

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
