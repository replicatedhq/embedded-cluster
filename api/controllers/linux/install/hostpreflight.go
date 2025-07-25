package install

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/internal/managers/preflight"
	"github.com/replicatedhq/embedded-cluster/api/internal/utils"
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/preflights"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
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

	if err := c.stateMachine.ValidateTransition(lock, StatePreflightsRunning); err != nil {
		return types.NewConflictError(err)
	}

	// Get the configured custom domains
	ecDomains := utils.GetDomains(c.releaseData)

	// Calculate airgap storage space requirement (2x uncompressed size for controller nodes)
	var controllerAirgapStorageSpace string
	if c.airgapMetadata != nil && c.airgapMetadata.AirgapInfo != nil {
		controllerAirgapStorageSpace = preflights.CalculateAirgapStorageSpace(preflights.AirgapStorageSpaceCalcArgs{
			UncompressedSize:   c.airgapMetadata.AirgapInfo.Spec.UncompressedSize,
			EmbeddedAssetsSize: c.embeddedAssetsSize,
			K0sImageSize:       c.airgapMetadata.K0sImageSize,
			IsController:       true,
		})
	}

	// Prepare host preflights
	hpf, err := c.hostPreflightManager.PrepareHostPreflights(ctx, c.rc, preflight.PrepareHostPreflightOptions{
		ReplicatedAppURL:             netutils.MaybeAddHTTPS(ecDomains.ReplicatedAppDomain),
		ProxyRegistryURL:             netutils.MaybeAddHTTPS(ecDomains.ProxyRegistryDomain),
		HostPreflightSpec:            c.releaseData.HostPreflights,
		EmbeddedClusterConfig:        c.releaseData.EmbeddedClusterConfig,
		IsAirgap:                     c.airgapBundle != "",
		IsUI:                         opts.IsUI,
		ControllerAirgapStorageSpace: controllerAirgapStorageSpace,
	})
	if err != nil {
		return fmt.Errorf("failed to prepare host preflights: %w", err)
	}

	err = c.stateMachine.Transition(lock, StatePreflightsRunning)
	if err != nil {
		return fmt.Errorf("failed to transition states: %w", err)
	}

	go func() (finalErr error) {
		// Background context is used to avoid canceling the operation if the context is canceled
		ctx := context.Background()

		defer lock.Release()

		defer func() {
			if r := recover(); r != nil {
				finalErr = fmt.Errorf("panic running host preflights: %v: %s", r, string(debug.Stack()))
			}
			if finalErr != nil {
				c.logger.Error(finalErr)

				if err := c.stateMachine.Transition(lock, StatePreflightsFailed); err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}
			} else {
				if err := c.stateMachine.Transition(lock, StatePreflightsSucceeded); err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}
			}
		}()

		err := c.hostPreflightManager.RunHostPreflights(ctx, c.rc, preflight.RunHostPreflightOptions{
			HostPreflightSpec: hpf,
		})
		if err != nil {
			return fmt.Errorf("failed to run host preflights: %w", err)
		}

		return nil
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
