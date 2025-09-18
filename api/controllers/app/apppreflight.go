package app

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"

	apppreflightmanager "github.com/replicatedhq/embedded-cluster/api/internal/managers/app/preflight"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
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
		return fmt.Errorf("no app preflight spec found")
	}

	err = c.stateMachine.Transition(lock, states.StateAppPreflightsRunning)
	if err != nil {
		return fmt.Errorf("transition states: %w", err)
	}

	go func() (finalErr error) {
		// Background context is used to avoid canceling the operation if the context is canceled
		ctx := context.Background()

		defer lock.Release()

		defer func() {
			// Clean up binary if requested
			if opts.CleanupBinary {
				_ = os.Remove(opts.PreflightBinaryPath)
			}

			if r := recover(); r != nil {
				finalErr = fmt.Errorf("panic running app preflights: %v: %s", r, string(debug.Stack()))
			}
			// Handle errors from preflight execution
			if finalErr != nil {
				c.logger.Error(finalErr)

				if err := c.stateMachine.Transition(lock, states.StateAppPreflightsExecutionFailed); err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}
				return
			}

			// Get the state from the preflights output
			state := c.getStateFromAppPreflightsOutput(ctx)
			// Transition to the appropriate state based on preflight results
			if err := c.stateMachine.Transition(lock, state); err != nil {
				c.logger.Errorf("failed to transition states: %w", err)
			}
		}()

		// Create RunOptions from the provided options
		runOpts := preflights.RunOptions{
			PreflightBinaryPath: opts.PreflightBinaryPath,
			ProxySpec:           opts.ProxySpec,
			ExtraPaths:          opts.ExtraPaths,
		}

		err := c.appPreflightManager.RunAppPreflights(ctx, apppreflightmanager.RunAppPreflightOptions{
			AppPreflightSpec: appPreflightSpec,
			RunOptions:       runOpts,
		})
		if err != nil {
			return fmt.Errorf("run app preflights: %w", err)
		}

		return nil
	}()

	return nil
}

func (c *AppController) getStateFromAppPreflightsOutput(ctx context.Context) statemachine.State {
	output, err := c.GetAppPreflightOutput(ctx)
	// If there was an error getting the state we assume preflight execution failed
	if err != nil {
		c.logger.WithError(err).Error("error getting app preflight output")
		return states.StateAppPreflightsExecutionFailed
	}
	// If there is no output, we assume preflights succeeded
	if output == nil || !output.HasFail() {
		return states.StateAppPreflightsSucceeded
	}
	return states.StateAppPreflightsFailed
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
