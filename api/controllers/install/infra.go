package install

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *InstallController) SetupInfra(ctx context.Context) error {
	preflightStatus, err := c.GetHostPreflightStatus(ctx)
	if err != nil {
		return fmt.Errorf("get install host preflight status: %w", err)
	}

	if preflightStatus.State != types.StateFailed && preflightStatus.State != types.StateSucceeded {
		return fmt.Errorf("host preflight checks did not complete")
	}

	if preflightStatus.State == types.StateFailed && c.metricsReporter != nil {
		preflightOutput, err := c.GetHostPreflightOutput(ctx)
		if err != nil {
			return fmt.Errorf("get install host preflight output: %w", err)
		}
		if preflightOutput != nil {
			c.metricsReporter.ReportPreflightsFailed(ctx, preflightOutput)
		}
	}

	// Get current installation config
	config, err := c.installationManager.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to read installation config: %w", err)
	}

	if err := c.infraManager.Install(ctx, config); err != nil {
		return fmt.Errorf("install infra: %w", err)
	}

	return nil
}

func (c *InstallController) GetInfra(ctx context.Context) (*types.Infra, error) {
	return c.infraManager.Get()
}
