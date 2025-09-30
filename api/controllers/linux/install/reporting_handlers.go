package install

import (
	"context"
	"errors"
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

func (c *InstallController) reportInstallFailed(ctx context.Context, _, toState statemachine.State) {
	var status types.Status
	var err error

	switch toState {
	case states.StateInstallationConfigurationFailed, states.StateHostConfigurationFailed:
		status, err = c.store.LinuxInstallationStore().GetStatus()
		if err != nil {
			err = fmt.Errorf("get status from installation store: %w", err)
		}
	case states.StateInfrastructureInstallFailed:
		status, err = c.store.LinuxInfraStore().GetStatus()
		if err != nil {
			err = fmt.Errorf("get status from infra store: %w", err)
		}
	case states.StateAppInstallFailed:
		status, err = c.store.AppInstallStore().GetStatus()
		if err != nil {
			err = fmt.Errorf("get status from app install store: %w", err)
		}
	}
	if err != nil {
		c.logger.WithError(err).Error("failed to report failed install")
		return
	}

	c.logger.Info("Reporting metrics event install failed")
	c.metricsReporter.ReportInstallationFailed(ctx, errors.New(status.Description))
}

func (c *InstallController) reportInstallSucceeded(ctx context.Context, _, _ statemachine.State) {
	c.logger.Info("Reporting metrics event install succeeded")
	c.metricsReporter.ReportInstallationSucceeded(ctx)
}

func (c *InstallController) reportHostPreflightsFailed(ctx context.Context, _, _ statemachine.State) {
	output, err := c.store.LinuxPreflightStore().GetOutput()
	if err != nil {
		err = fmt.Errorf("get output from linux preflight store: %w", err)
		c.logger.WithError(err).Error("failed to report host preflights failed")
		return
	}
	c.logger.Info("Reporting metrics event host preflights failed")
	c.metricsReporter.ReportHostPreflightsFailed(ctx, output)
}

func (c *InstallController) reportHostPreflightsBypassed(ctx context.Context, _, _ statemachine.State) {
	output, err := c.store.LinuxPreflightStore().GetOutput()
	if err != nil {
		err = fmt.Errorf("get output from linux preflight store: %w", err)
		c.logger.WithError(err).Error("failed to report host preflights bypassed")
		return
	}
	c.logger.Info("Reporting metrics event host preflights bypassed")
	c.metricsReporter.ReportHostPreflightsBypassed(ctx, output)
}

func (c *InstallController) reportHostPreflightsSucceeded(ctx context.Context, _, _ statemachine.State) {
	c.logger.Info("Reporting metrics event host preflights succeeded")
	c.metricsReporter.ReportHostPreflightsSucceeded(ctx)
}

func (c *InstallController) reportAppPreflightsFailed(ctx context.Context, _, _ statemachine.State) {
	output, err := c.store.AppPreflightStore().GetOutput()
	if err != nil {
		err = fmt.Errorf("get output from app preflight store: %w", err)
		c.logger.WithError(err).Error("failed to report app preflights failed")
		return
	}
	c.logger.Info("Reporting metrics event app preflights failed")
	c.metricsReporter.ReportAppPreflightsFailed(ctx, output)
}

func (c *InstallController) reportAppPreflightsBypassed(ctx context.Context, _, _ statemachine.State) {
	output, err := c.store.AppPreflightStore().GetOutput()
	if err != nil {
		err = fmt.Errorf("get output from app preflight store: %w", err)
		c.logger.WithError(err).Error("failed to report app preflights bypassed")
		return
	}
	c.logger.Info("Reporting metrics event app preflights bypassed")
	c.metricsReporter.ReportAppPreflightsBypassed(ctx, output)
}

func (c *InstallController) reportAppPreflightsSucceeded(ctx context.Context, _, _ statemachine.State) {
	c.logger.Info("Reporting metrics event app preflights succeeded")
	c.metricsReporter.ReportAppPreflightsSucceeded(ctx)
}
