package install

import (
	"context"
	"errors"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

var ErrPreflightChecksFailed = errors.New("preflight checks failed")

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
		// Get preflight output for reporting
		preflightOutput, err := c.GetHostPreflightOutput(ctx)
		if err != nil {
			return false, fmt.Errorf("get install host preflight output: %w", err)
		}

		// Check if we can proceed despite failures
		if !ignorePreflightFailures || !c.allowIgnoreHostPreflights {
			return false, ErrPreflightChecksFailed
		}

		// We're proceeding despite failures - report bypass
		if c.metricsReporter != nil && preflightOutput != nil {
			c.metricsReporter.ReportPreflightsBypassed(ctx, preflightOutput)
		}
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
