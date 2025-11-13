package install

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *InstallController) ProcessAirgap(ctx context.Context) (finalErr error) {
	logger := c.logger.WithField("operation", "process-airgap")

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

	err = c.stateMachine.Transition(lock, states.StateAirgapProcessing, nil)
	if err != nil {
		return types.NewConflictError(err)
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

				if err := c.stateMachine.Transition(lock, states.StateAirgapProcessingFailed, finalErr); err != nil {
					logger.WithError(err).Error("failed to transition states")
				}

				if err := c.setAirgapStatus(types.StateFailed, finalErr.Error()); err != nil {
					logger.WithError(err).Error("failed to set status to failed")
				}
			}
		}()

		if err := c.setAirgapStatus(types.StateRunning, "Processing airgap bundle"); err != nil {
			return fmt.Errorf("set status to running: %w", err)
		}

		// Calculate registry settings for the airgap processing
		registrySettings, err := c.CalculateRegistrySettings(ctx)
		if err != nil {
			return fmt.Errorf("calculate registry settings: %w", err)
		}

		if err := c.airgapManager.Process(ctx, registrySettings); err != nil {
			return fmt.Errorf("process airgap: %w", err)
		}

		if err := c.stateMachine.Transition(lock, states.StateAirgapProcessed, nil); err != nil {
			return fmt.Errorf("transition states: %w", err)
		}

		if err := c.setAirgapStatus(types.StateSucceeded, "Airgap bundle processed"); err != nil {
			logger.WithError(err).Error("failed to set status to succeeded")
		}

		return nil
	}()

	return nil
}

func (c *InstallController) GetAirgapStatus(ctx context.Context) (types.Airgap, error) {
	return c.store.AirgapStore().Get()
}

func (c *InstallController) setAirgapStatus(state types.State, description string) error {
	return c.store.AirgapStore().SetStatus(types.Status{
		State:       state,
		Description: description,
		LastUpdated: time.Now(),
	})
}
