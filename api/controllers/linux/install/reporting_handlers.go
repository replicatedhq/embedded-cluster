package install

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	"github.com/replicatedhq/embedded-cluster/api/internal/states"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *InstallController) registerReportingHandlers() {
	c.stateMachine.RegisterEventHandler(states.StateHostConfigurationFailed, c.reportInstallFailed)
	c.stateMachine.RegisterEventHandler(states.StateInstallationConfigurationFailed, c.reportInstallFailed)
	c.stateMachine.RegisterEventHandler(states.StateInfrastructureInstallFailed, c.reportInstallFailed)
	c.stateMachine.RegisterEventHandler(states.StateAppInstallFailed, c.reportInstallFailed)
	c.stateMachine.RegisterEventHandler(states.StateSucceeded, c.reportInstallSucceeded)

	// report preflight failures and bypassed
	c.stateMachine.RegisterEventHandler(states.StateHostPreflightsFailed, c.reportHostPreflightsFailed)
	c.stateMachine.RegisterEventHandler(states.StateHostPreflightsFailedBypassed, c.reportHostPreflightsBypassed)
	c.stateMachine.RegisterEventHandler(states.StateHostPreflightsSucceeded, c.reportHostPreflightsSucceeded)
	c.stateMachine.RegisterEventHandler(states.StateAppPreflightsFailed, c.reportAppPreflightsFailed)
	c.stateMachine.RegisterEventHandler(states.StateAppPreflightsFailedBypassed, c.reportAppPreflightsBypassed)
	c.stateMachine.RegisterEventHandler(states.StateAppPreflightsSucceeded, c.reportAppPreflightsSucceeded)
}

func (c *InstallController) reportInstallFailed(ctx context.Context, _, toState statemachine.State, eventData interface{}) {
	err, ok := eventData.(error)
	if !ok {
		c.logger.
			WithField("dataType", fmt.Sprintf("%T", eventData)).
			Error("failed to report install failed: invalid event data")
		return
	}
	c.logger.Info("Reporting metrics event install failed")
	c.metricsReporter.ReportInstallationFailed(ctx, err)
}

func (c *InstallController) reportInstallSucceeded(ctx context.Context, _, _ statemachine.State, _ interface{}) {
	c.logger.Info("Reporting metrics event install succeeded")
	c.metricsReporter.ReportInstallationSucceeded(ctx)
}

func (c *InstallController) reportHostPreflightsFailed(ctx context.Context, _, _ statemachine.State, eventData interface{}) {
	output, ok := eventData.(*types.PreflightsOutput)
	if !ok {
		c.logger.
			WithField("dataType", fmt.Sprintf("%T", eventData)).
			Error("failed to report host preflights failed: invalid event data")
		return
	}
	c.logger.Info("Reporting metrics event host preflights failed")
	c.metricsReporter.ReportHostPreflightsFailed(ctx, output)
}

func (c *InstallController) reportHostPreflightsBypassed(ctx context.Context, _, _ statemachine.State, eventData interface{}) {
	output, ok := eventData.(*types.PreflightsOutput)
	if !ok {
		c.logger.
			WithField("dataType", fmt.Sprintf("%T", eventData)).
			Error("failed to report host preflights bypassed: invalid event data")
		return
	}
	c.logger.Info("Reporting metrics event host preflights bypassed")
	c.metricsReporter.ReportHostPreflightsBypassed(ctx, output)
}

func (c *InstallController) reportHostPreflightsSucceeded(ctx context.Context, _, _ statemachine.State, _ interface{}) {
	c.logger.Info("Reporting metrics event host preflights succeeded")
	c.metricsReporter.ReportHostPreflightsSucceeded(ctx)
}

func (c *InstallController) reportAppPreflightsFailed(ctx context.Context, _, _ statemachine.State, eventData interface{}) {
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

func (c *InstallController) reportAppPreflightsBypassed(ctx context.Context, _, _ statemachine.State, eventData interface{}) {
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

func (c *InstallController) reportAppPreflightsSucceeded(ctx context.Context, _, _ statemachine.State, _ interface{}) {
	c.logger.Info("Reporting metrics event app preflights succeeded")
	c.metricsReporter.ReportAppPreflightsSucceeded(ctx)
}
