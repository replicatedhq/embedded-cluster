package upgrade

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *UpgradeController) registerReportingHandlers() {
	c.stateMachine.RegisterEventHandler(states.StateInfrastructureUpgradeFailed, c.reportUpgradeFailed)
	c.stateMachine.RegisterEventHandler(states.StateAppUpgradeFailed, c.reportUpgradeFailed)
	c.stateMachine.RegisterEventHandler(states.StateSucceeded, c.reportUpgradeSucceeded)

	// report preflight failures and bypassed
	c.stateMachine.RegisterEventHandler(states.StateAppPreflightsFailed, c.reportAppPreflightsFailed)
	c.stateMachine.RegisterEventHandler(states.StateAppPreflightsFailedBypassed, c.reportAppPreflightsBypassed)
	c.stateMachine.RegisterEventHandler(states.StateAppPreflightsSucceeded, c.reportAppPreflightsSucceeded)
}

func (c *UpgradeController) reportUpgradeFailed(ctx context.Context, _, toState statemachine.State, eventData interface{}) {
	err, ok := eventData.(error)
	if !ok {
		c.logger.
			WithField("dataType", fmt.Sprintf("%T", eventData)).
			Error("failed to report upgrade failed: invalid event data")
		return
	}
	c.logger.Info("Reporting metrics event upgrade failed")
	c.metricsReporter.ReportUpgradeFailed(ctx, err, c.targetVersion, c.initialVersion)
}

func (c *UpgradeController) reportUpgradeSucceeded(ctx context.Context, _, _ statemachine.State, _ interface{}) {
	c.logger.Info("Reporting metrics event upgrade succeeded")
	c.metricsReporter.ReportUpgradeSucceeded(ctx, c.targetVersion, c.initialVersion)
}

func (c *UpgradeController) reportAppPreflightsFailed(ctx context.Context, _, _ statemachine.State, eventData interface{}) {
	output, ok := eventData.(*types.PreflightsOutput)
	if !ok {
		c.logger.
			WithField("dataType", fmt.Sprintf("%T", eventData)).
			Error("failed to report app preflights failed: invalid event data")
		return
	}
	c.logger.Info("Reporting metrics event app preflights failed")
	c.metricsReporter.ReportAppPreflightsFailed(ctx, output)
}

func (c *UpgradeController) reportAppPreflightsBypassed(ctx context.Context, _, _ statemachine.State, eventData interface{}) {
	output, ok := eventData.(*types.PreflightsOutput)
	if !ok {
		c.logger.
			WithField("dataType", fmt.Sprintf("%T", eventData)).
			Error("failed to report app preflights bypassed: invalid event data")
		return
	}
	c.logger.Info("Reporting metrics event app preflights bypassed")
	c.metricsReporter.ReportAppPreflightsBypassed(ctx, output)
}

func (c *UpgradeController) reportAppPreflightsSucceeded(ctx context.Context, _, _ statemachine.State, _ interface{}) {
	c.logger.Info("Reporting metrics event app preflights succeeded")
	c.metricsReporter.ReportAppPreflightsSucceeded(ctx)
}
