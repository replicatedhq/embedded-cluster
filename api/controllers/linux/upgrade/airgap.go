package upgrade

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *UpgradeController) ProcessAirgap(ctx context.Context) (finalErr error) {
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

	err = c.stateMachine.Transition(lock, states.StateAirgapProcessing)
	if err != nil {
		return types.NewConflictError(err)
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

				if err := c.stateMachine.Transition(lock, states.StateAirgapProcessingFailed); err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}
			} else {
				if err := c.stateMachine.Transition(lock, states.StateAirgapProcessed); err != nil {
					c.logger.Errorf("failed to transition states: %w", err)
				}
			}
		}()

		// Get registry settings from the cluster for upgrades
		registrySettings, err := c.GetRegistrySettings(ctx, c.rc)
		if err != nil {
			return fmt.Errorf("failed to get registry settings: %w", err)
		}

		if err := c.airgapManager.Process(ctx, registrySettings); err != nil {
			return fmt.Errorf("failed to process airgap: %w", err)
		}

		return nil
	}()

	return nil
}

func (c *UpgradeController) GetAirgapStatus(ctx context.Context) (types.Airgap, error) {
	return c.airgapManager.GetStatus()
}
