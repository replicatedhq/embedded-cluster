package app

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	apppreflightmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/preflight"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
)

type RunAppPreflightOptions struct {
	PreflightBinaryPath string
	ProxySpec           *ecv1beta1.ProxySpec
	RegistrySettings    *types.RegistrySettings
	ExtraPaths          []string
	CleanupBinary       bool
}

func (c *AppController) RunAppPreflights(ctx context.Context, opts RunAppPreflightOptions) (finalErr error) {
	logger := c.logger.WithField("operation", "run-app-preflights")

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

	if err := c.stateMachine.ValidateTransition(lock, states.StateAppPreflightsRunning); err != nil {
		return types.NewConflictError(err)
	}

	// Clear any previous preflight results immediately to prevent serving stale data
	if err := c.appPreflightManager.ClearAppPreflightResults(ctx); err != nil {
		return fmt.Errorf("clear previous preflight results: %w", err)
	}

	// Get the app config values
	configValues, err := c.GetAppConfigValues(ctx)
	if err != nil {
		return fmt.Errorf("get app config values: %w", err)
	}

	// Extract app preflight spec from Helm charts
	appPreflightSpec, err := c.appReleaseManager.ExtractAppPreflightSpec(ctx, configValues, opts.ProxySpec, opts.RegistrySettings)
	if err != nil {
		return fmt.Errorf("extract app preflight spec: %w", err)
	}
	if appPreflightSpec == nil {
		// TODO: support for installing without an app preflight spec
		return fmt.Errorf("no app preflight spec found")
	}

	err = c.stateMachine.Transition(lock, states.StateAppPreflightsRunning, nil)
	if err != nil {
		return fmt.Errorf("transition states: %w", err)
	}

	go func() (finalErr error) {
		defer lock.Release()

		// Background context is used to avoid canceling the operation if the context is canceled
		ctx := context.Background()

		defer func() {
			// Clean up binary if requested
			if opts.CleanupBinary {
				_ = os.Remove(opts.PreflightBinaryPath)
			}

			if r := recover(); r != nil {
				finalErr = fmt.Errorf("panic: %v: %s", r, string(debug.Stack()))
			}
			if finalErr != nil {
				logger.Error(finalErr)

				if err := c.stateMachine.Transition(lock, states.StateAppPreflightsExecutionFailed, finalErr); err != nil {
					logger.WithError(err).Error("failed to transition states")
				}

				if err := c.setAppPreflightStatus(types.StateFailed, finalErr.Error()); err != nil {
					logger.WithError(err).Error("failed to set status to failed")
				}
			}
		}()

		if err := c.setAppPreflightStatus(types.StateRunning, "Running app preflights"); err != nil {
			return fmt.Errorf("set status to running: %w", err)
		}

		// Create RunOptions from the provided options
		runOpts := preflights.RunOptions{
			PreflightBinaryPath: opts.PreflightBinaryPath,
			ProxySpec:           opts.ProxySpec,
			ExtraPaths:          opts.ExtraPaths,
		}

		output, err := c.appPreflightManager.RunAppPreflights(ctx, apppreflightmanager.RunAppPreflightOptions{
			AppPreflightSpec: appPreflightSpec,
			RunOptions:       runOpts,
		})
		if err != nil {
			return fmt.Errorf("run app preflights: %w", err)
		}

		if output.HasFail() {
			if err := c.stateMachine.Transition(lock, states.StateAppPreflightsFailed, output); err != nil {
				return fmt.Errorf("transition states: %w", err)
			}

			if err := c.setAppPreflightStatus(types.StateFailed, "App preflights failed"); err != nil {
				return fmt.Errorf("set status to failed: %w", err)
			}
		} else {
			if err := c.stateMachine.Transition(lock, states.StateAppPreflightsSucceeded, output); err != nil {
				return fmt.Errorf("transition states: %w", err)
			}

			if err := c.setAppPreflightStatus(types.StateSucceeded, "App preflights succeeded"); err != nil {
				logger.WithError(err).Error("failed to set status to succeeded")
			}
		}

		return nil
	}()

	return nil
}

func (c *AppController) GetAppPreflightStatus(ctx context.Context) (types.Status, error) {
	return c.appPreflightManager.GetAppPreflightStatus(ctx)
}

func (c *AppController) GetAppPreflightOutput(ctx context.Context) (*types.PreflightsOutput, error) {
	return c.appPreflightManager.GetAppPreflightOutput(ctx)
}

func (c *AppController) GetAppPreflightTitles(ctx context.Context) ([]string, error) {
	return c.appPreflightManager.GetAppPreflightTitles(ctx)
}

func (c *AppController) setAppPreflightStatus(state types.State, description string) error {
	return c.store.AppPreflightStore().SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
