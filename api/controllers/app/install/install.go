package install

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
)

var (
	ErrAppPreflightChecksFailed = errors.New("app preflight checks failed")
)

// InstallApp triggers app installation with proper state transitions and panic handling
func (c *InstallController) InstallApp(ctx context.Context, ignoreAppPreflights bool, proxySpec *ecv1beta1.ProxySpec) (finalErr error) {
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
		allowIgnoreAppPreflights := true // TODO: implement once we check for strict app preflights
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
	appConfigValues, err := c.GetAppConfigValues(ctx)
	if err != nil {
		return fmt.Errorf("get app config values for app install: %w", err)
	}

	// Get KOTS config values for the KOTS CLI
	kotsConfigValues, err := c.appConfigManager.GetKotsadmConfigValues()
	if err != nil {
		return fmt.Errorf("get kots config values for app install: %w", err)
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
				finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
			}
			if finalErr != nil {
				c.logger.Error(finalErr)

				if err := c.stateMachine.Transition(lock, states.StateAppInstallFailed); err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}
			} else {
				if err := c.stateMachine.Transition(lock, states.StateSucceeded); err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}
			}
		}()

		// Extract installable Helm charts from release manager
		installableCharts, err := c.appReleaseManager.ExtractInstallableHelmCharts(ctx, appConfigValues, proxySpec)
		if err != nil {
			return fmt.Errorf("extract installable helm charts: %w", err)
		}

		// Install the app with installable charts and kots config values
		err = c.appInstallManager.Install(ctx, installableCharts, kotsConfigValues)
		if err != nil {
			return fmt.Errorf("install app: %w", err)
		}

		return nil
	}()

	return nil
}

func (c *InstallController) GetAppInstallStatus(ctx context.Context) (types.AppInstall, error) {
	return c.appInstallManager.GetStatus()
}
