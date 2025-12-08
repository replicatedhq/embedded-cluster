package app

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
)

var (
	ErrAppPreflightChecksFailed = errors.New("app preflight checks failed")
)

type InstallAppOptions struct {
	IgnoreAppPreflights bool
	ProxySpec           *ecv1beta1.ProxySpec
	RegistrySettings    *types.RegistrySettings
	HostCABundlePath    string
}

// InstallApp triggers app installation with proper state transitions and panic handling
func (c *AppController) InstallApp(ctx context.Context, opts InstallAppOptions) (finalErr error) {
	logger := c.logger.WithField("operation", "install-app")

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
		if !opts.IgnoreAppPreflights || !allowIgnoreAppPreflights {
			return types.NewBadRequestError(ErrAppPreflightChecksFailed)
		}

		err = c.stateMachine.Transition(lock, states.StateAppPreflightsFailedBypassed, preflightOutput)
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

	// Extract installable Helm charts from release manager
	installableCharts, err := c.appReleaseManager.ExtractInstallableHelmCharts(ctx, appConfigValues, opts.ProxySpec, opts.RegistrySettings)
	if err != nil {
		return fmt.Errorf("extract installable helm charts: %w", err)
	}

	err = c.stateMachine.Transition(lock, states.StateAppInstalling, nil)
	if err != nil {
		return fmt.Errorf("transition states: %w", err)
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

				if err := c.stateMachine.Transition(lock, states.StateAppInstallFailed, finalErr); err != nil {
					logger.WithError(err).Error("failed to transition states")
				}

				if err := c.setAppInstallStatus(types.StateFailed, finalErr.Error()); err != nil {
					logger.WithError(err).Error("failed to set status to failed")
				}
			}
		}()

		if err := c.setAppInstallStatus(types.StateRunning, "Installing application"); err != nil {
			return fmt.Errorf("set status to running: %w", err)
		}

		// Install the app with installable charts
		err = c.appInstallManager.Install(ctx, installableCharts, appConfigValues, opts.RegistrySettings, opts.HostCABundlePath)
		if err != nil {
			return fmt.Errorf("install app: %w", err)
		}

		if err := c.stateMachine.Transition(lock, states.StateSucceeded, nil); err != nil {
			return fmt.Errorf("transition states: %w", err)
		}

		if err := c.setAppInstallStatus(types.StateSucceeded, "Installation complete"); err != nil {
			return fmt.Errorf("set status to succeeded: %w", err)
		}

		return nil
	}()

	return nil
}

func (c *AppController) GetAppInstallStatus(ctx context.Context) (types.AppInstall, error) {
	return c.store.AppInstallStore().Get()
}

func (c *AppController) setAppInstallStatus(state types.State, description string) error {
	return c.store.AppInstallStore().SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
