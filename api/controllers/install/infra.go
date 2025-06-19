package install

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *InstallController) SetupInfra(ctx context.Context, ignorePreflightFailures bool) (preflightsWereIgnored bool, err error) {
	// Check preflight status and requirements
	preflightStatus, err := c.GetHostPreflightStatus(ctx)
	if err != nil {
		return false, fmt.Errorf("get install host preflight status: %w", err)
	}

	if preflightStatus.State != types.StateFailed && preflightStatus.State != types.StateSucceeded {
		return false, fmt.Errorf("host preflight checks did not complete")
	}

	preflightsWereIgnored = false

	// Handle failed preflights
	if preflightStatus.State == types.StateFailed {
		// Report metrics for failed preflights
		if c.metricsReporter != nil {
			preflightOutput, err := c.GetHostPreflightOutput(ctx)
			if err != nil {
				return false, fmt.Errorf("get install host preflight output: %w", err)
			}
			if preflightOutput != nil {
				c.metricsReporter.ReportPreflightsFailed(ctx, preflightOutput)
			}
		}

		// Check if we can proceed despite failures
		if !ignorePreflightFailures || !c.allowIgnoreHostPreflights {
			return false, fmt.Errorf("Preflight checks failed")
		}

		// We're proceeding despite failures
		preflightsWereIgnored = true
	}

	// Install infrastructure using main's approach
	if err := c.infraManager.Install(ctx, c.rc); err != nil {
		return false, fmt.Errorf("install infra: %w", err)
	}

	return preflightsWereIgnored, nil
}

func (c *InstallController) GetInfra(ctx context.Context) (types.Infra, error) {
	return c.infraManager.Get()
}
