package upgrade

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/linux/preflight"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

// RunHostPreflights runs host preflight checks for the upgrade
func (c *UpgradeController) RunHostPreflights(ctx context.Context) (finalErr error) {
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
		EmbeddedClusterConfig: c.endUserConfig,
		IsAirgap:              c.airgapBundle != "",
		AirgapInfo:            airgapInfo,
		EmbeddedAssetsSize:    c.embeddedAssetsSize,
		Mode:                  apitypes.ModeUpgrade,
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

		// Create RunHostPreflightOptions from the prepared spec
		runOpts := preflight.RunHostPreflightOptions{
			HostPreflightSpec: hpf,
		}

		output, err := c.hostPreflightManager.RunHostPreflights(ctx, c.rc, runOpts)
		if err != nil {
			return fmt.Errorf("run host preflights: %w", err)
		}

		if output.HasFail() {
			if err := c.stateMachine.Transition(lock, states.StateHostPreflightsFailed, output); err != nil {
				return fmt.Errorf("transition states: %w", err)
			}

			if err := c.setHostPreflightStatus(types.StateFailed, "Host preflights failed"); err != nil {
				return fmt.Errorf("set status to failed: %w", err)
			}
		} else {
			if err := c.stateMachine.Transition(lock, states.StateHostPreflightsSucceeded, output); err != nil {
				return fmt.Errorf("transition states: %w", err)
			}

			if err := c.setHostPreflightStatus(types.StateSucceeded, "Host preflights succeeded"); err != nil {
				return fmt.Errorf("set status to succeeded: %w", err)
			}
		}

		return nil
	}()

	return nil
}

// GetHostPreflightsStatus returns the current status of host preflights
func (c *UpgradeController) GetHostPreflightsStatus(ctx context.Context) (types.HostPreflights, error) {
	status, err := c.hostPreflightManager.GetHostPreflightStatus(ctx)
	if err != nil {
		return types.HostPreflights{}, err
	}

	output, err := c.hostPreflightManager.GetHostPreflightOutput(ctx)
	if err != nil {
		return types.HostPreflights{}, err
	}

	titles, err := c.hostPreflightManager.GetHostPreflightTitles(ctx)
	if err != nil {
		return types.HostPreflights{}, err
	}

	return types.HostPreflights{
		Status: status,
		Output: output,
		Titles: titles,
	}, nil
}

func (c *UpgradeController) setHostPreflightStatus(state types.State, description string) error {
	return c.store.LinuxPreflightStore().SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
