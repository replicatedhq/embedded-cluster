package install

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/preflight"
	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

func (c *InstallController) RunHostPreflights(ctx context.Context, opts RunHostPreflightsOptions) (finalErr error) {
	logger := c.logger.WithField("operation", "run-host-preflights")

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

	err = c.stateMachine.Transition(lock, states.StateHostPreflightsRunning, nil)
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

				if err := c.stateMachine.Transition(lock, states.StateHostPreflightsExecutionFailed, finalErr); err != nil {
					logger.WithError(err).Error("failed to transition states")
				}

				if err := c.setHostPreflightStatus(types.StateFailed, finalErr.Error()); err != nil {
					logger.WithError(err).Error("failed to set status to failed")
				}
			}
		}()

		if err := c.setHostPreflightStatus(types.StateRunning, "Running host preflights"); err != nil {
			return fmt.Errorf("set status to running: %w", err)
		}

		// Create RunHostPreflightOptions from the provided options
		runOpts := preflight.RunHostPreflightOptions{
			HostPreflightSpec: hpf,
		}

		output, err := c.hostPreflightManager.RunHostPreflights(ctx, c.rc, runOpts)
		if err != nil {
			return fmt.Errorf("run host preflights: %w", err)
		}

		// Transition state machine based on whether there are failures
		// This is used internally for workflow state tracking
		var smState statemachine.State
		if output.HasFail() {
			smState = states.StateHostPreflightsFailed
		} else {
			smState = states.StateHostPreflightsSucceeded
		}

		if err := c.stateMachine.Transition(lock, smState, output); err != nil {
			return fmt.Errorf("transition states: %w", err)
		}

		// Always set status to StateSucceeded to indicate execution completed successfully
		// The output will contain failures/warnings for the caller to check
		var statusDescription string
		if output.HasFail() {
			statusDescription = "Host preflights completed with failures"
		} else {
			statusDescription = "Host preflights succeeded"
		}

		if err := c.setHostPreflightStatus(types.StateSucceeded, statusDescription); err != nil {
			return fmt.Errorf("set status to succeeded: %w", err)
		}

		return nil
	}()

	return nil
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

func (c *InstallController) setHostPreflightStatus(state types.State, description string) error {
	return c.store.LinuxPreflightStore().SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
