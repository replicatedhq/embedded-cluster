package install

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/preflight"
	"github.com/replicatedhq/embedded-cluster/api/pkg/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
)

func (c *InstallController) RunHostPreflights(ctx context.Context, opts RunHostPreflightsOptions) error {
	lock, err := c.stateMachine.AcquireLock()
	if err != nil {
		return types.NewConflictError(err)
	}

	if err := c.stateMachine.ValidateTransition(lock, StatePreflightsRunning); err != nil {
		return types.NewConflictError(err)
	}

	// Get the configured custom domains
	ecDomains := utils.GetDomains(c.releaseData)

	// Prepare host preflights
	hpf, err := c.hostPreflightManager.PrepareHostPreflights(ctx, c.rc, preflight.PrepareHostPreflightOptions{
		ReplicatedAppURL:      netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain),
		ProxyRegistryURL:      netutils.MaybeAddHTTPS(ecDomains.ProxyRegistryDomain),
		HostPreflightSpec:     c.releaseData.HostPreflights,
		EmbeddedClusterConfig: c.releaseData.EmbeddedClusterConfig,
		IsAirgap:              c.airgapBundle != "",
		IsUI:                  opts.IsUI,
	})
	if err != nil {
		lock.Release()
		return fmt.Errorf("failed to prepare host preflights: %w", err)
	}

	err = c.stateMachine.Transition(lock, StatePreflightsRunning)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	go func() {
		defer lock.Release()

		defer func() {
			if r := recover(); r != nil {
				c.logger.Errorf("panic running host preflights: %v: %s", r, string(debug.Stack()))

				err := c.stateMachine.Transition(lock, StatePreflightsFailed)
				if err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}
			}
		}()

		// Background context is used to avoid canceling the operation if the context is canceled
		err := c.hostPreflightManager.RunHostPreflights(context.Background(), c.rc, preflight.RunHostPreflightOptions{
			HostPreflightSpec: hpf,
		})

		if err != nil {
			c.logger.Errorf("failed to run host preflights: %w", err)

			err = c.stateMachine.Transition(lock, StatePreflightsFailed)
			if err != nil {
				c.logger.Errorf("failed to transition states: %w", err)
			}
		} else {
			err = c.stateMachine.Transition(lock, StatePreflightsSucceeded)
			if err != nil {
				c.logger.Errorf("failed to transition states: %w", err)
			}
		}
	}()

	return nil
}

func (c *InstallController) GetHostPreflightStatus(ctx context.Context) (types.Status, error) {
	return c.hostPreflightManager.GetHostPreflightStatus(ctx)
}

func (c *InstallController) GetHostPreflightOutput(ctx context.Context) (*types.HostPreflightsOutput, error) {
	return c.hostPreflightManager.GetHostPreflightOutput(ctx)
}

func (c *InstallController) GetHostPreflightTitles(ctx context.Context) ([]string, error) {
	return c.hostPreflightManager.GetHostPreflightTitles(ctx)
}
