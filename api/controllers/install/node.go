package install

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *InstallController) SetupNode(ctx context.Context) error {
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

	// TODO: implement node setup

	return nil
}
