package install

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/preflight"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

func (c *InstallController) RunHostPreflights(ctx context.Context, opts RunHostPreflightsOptions) (finalErr error) {
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

	if err := c.stateMachine.ValidateTransition(lock, states.StateHostPreflightsRunning); err != nil {
		return types.NewConflictError(err)
	}

	// Clear any previous preflight results immediately to prevent serving stale data
	if err := c.hostPreflightManager.ClearHostPreflightResults(ctx); err != nil {
		return fmt.Errorf("clear previous preflight results: %w", err)
	}

	// Get the configured custom domains
	ecDomains := utils.GetDomains(c.releaseData)

	var airgapInfo *kotsv1beta1.Airgap
	if c.airgapMetadata != nil {
		airgapInfo = c.airgapMetadata.AirgapInfo
	}

	// Prepare host preflights
	hpf, err := c.hostPreflightManager.PrepareHostPreflights(ctx, c.rc, preflight.PrepareHostPreflightOptions{
		ReplicatedAppURL:      netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain),
		ProxyRegistryURL:      netutils.MaybeAddHTTPS(ecDomains.ProxyRegistryDomain),
		HostPreflightSpec:     c.releaseData.HostPreflights,
		EmbeddedClusterConfig: c.releaseData.EmbeddedClusterConfig,
		IsAirgap:              c.airgapBundle != "",
		IsUI:                  opts.IsUI,
		AirgapInfo:            airgapInfo,
		EmbeddedAssetsSize:    c.embeddedAssetsSize,
	})
	if err != nil {
		return fmt.Errorf("prepare host preflights: %w", err)
	}

	err = c.stateMachine.Transition(lock, states.StateHostPreflightsRunning)
	if err != nil {
		return fmt.Errorf("transition states: %w", err)
	}

	go func() (finalErr error) {
		// Background context is used to avoid canceling the operation if the context is canceled
		ctx := context.Background()

		defer lock.Release()

		defer func() {
			if r := recover(); r != nil {
				finalErr = fmt.Errorf("panic running host preflights: %v: %s", r, string(debug.Stack()))
			}
			// Handle errors from preflight execution
			if finalErr != nil {
				c.logger.Error(finalErr)

				if err := c.stateMachine.Transition(lock, states.StateHostPreflightsExecutionFailed); err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}
				return
			}

			// Get the state from the preflights output
			state := c.getStateFromPreflightsOutput(ctx)
			// Transition to the appropriate state based on preflight results
			if err := c.stateMachine.Transition(lock, state); err != nil {
				c.logger.Errorf("failed to transition states: %w", err)
			}
		}()

		err := c.hostPreflightManager.RunHostPreflights(ctx, c.rc, preflight.RunHostPreflightOptions{
			HostPreflightSpec: hpf,
		})
		if err != nil {
			return fmt.Errorf("run host preflights: %w", err)
		}

		return nil
	}()

	return nil
}

func (c *InstallController) getStateFromPreflightsOutput(ctx context.Context) statemachine.State {
	status, err := c.GetHostPreflightStatus(ctx)
	if err != nil {
		c.logger.WithError(err).Error("error getting preflight status")
		return states.StateHostPreflightsExecutionFailed
	}
	switch status.State {
	case types.StateSucceeded:
		return states.StateHostPreflightsSucceeded
	case types.StateFailed:
		output, err := c.GetHostPreflightOutput(ctx)
		// If there was an error getting the state we assume preflight execution failed
		if err != nil {
			c.logger.WithError(err).Error("error getting preflight output")
			return states.StateHostPreflightsExecutionFailed
		}
		// If there are failures, we return the failed state
		if output != nil && output.HasFail() {
			return states.StateHostPreflightsFailed
		}
		// Otherwise, we assume preflight execution failed
		return states.StateHostPreflightsExecutionFailed
	default:
		c.logger.Errorf("unexpected preflight status: %s", status.State)
		return states.StateHostPreflightsExecutionFailed
	}
}

func (c *InstallController) GetHostPreflightStatus(ctx context.Context) (types.Status, error) {
	return c.hostPreflightManager.GetHostPreflightStatus(ctx)
}

func (c *InstallController) GetHostPreflightOutput(ctx context.Context) (*types.PreflightsOutput, error) {
	return c.hostPreflightManager.GetHostPreflightOutput(ctx)
}

func (c *InstallController) GetHostPreflightTitles(ctx context.Context) ([]string, error) {
	return c.hostPreflightManager.GetHostPreflightTitles(ctx)
}
