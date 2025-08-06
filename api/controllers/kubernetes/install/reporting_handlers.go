package install

import (
	"context"
	"errors"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
	states "github.com/replicatedhq/embedded-cluster/api/internal/states/install"
	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *InstallController) registerReportingHandlers() {
	c.stateMachine.RegisterEventHandler(states.StateInstallationConfigurationFailed, c.reportInfrastructureInstallFailed)
	c.stateMachine.RegisterEventHandler(states.StateInfrastructureInstallFailed, c.reportInfrastructureInstallFailed)
	c.stateMachine.RegisterEventHandler(states.StateInfrastructureInstallSucceeded, c.reportInfraInstallSucceeded)

	c.stateMachine.RegisterEventHandler(states.StateAppInstallFailed, c.reportAppInstallFailed)
	c.stateMachine.RegisterEventHandler(states.StateSucceeded, c.reportAppInstallSucceeded)

	// report preflight failures and bypassed
	c.stateMachine.RegisterEventHandler(states.StateAppPreflightsFailed, c.reportAppPreflightsFailed)
	c.stateMachine.RegisterEventHandler(states.StateAppPreflightsFailedBypassed, c.reportAppPreflightsBypassed)
	c.stateMachine.RegisterEventHandler(states.StateAppPreflightsSucceeded, c.reportAppPreflightsSucceeded)
}

func (c *InstallController) reportInfraInstallSucceeded(ctx context.Context, _, _ statemachine.State) {
	c.logger.Debug("reporting metrics event infrastructure install succeeded")
	c.metricsReporter.ReportInfraInstallationSucceeded(ctx)
}

func (c *InstallController) reportInfrastructureInstallFailed(ctx context.Context, _, toState statemachine.State) {
	var status types.Status
	var err error

	switch toState {
	case states.StateInstallationConfigurationFailed:
		status, err = c.store.KubernetesInstallationStore().GetStatus()
		if err != nil {
			err = fmt.Errorf("get status from installation store: %w", err)
		}
	case states.StateInfrastructureInstallFailed:
		status, err = c.store.KubernetesInfraStore().GetStatus()
		if err != nil {
			err = fmt.Errorf("get status from infra store: %w", err)
		}
	}
	if err != nil {
		c.logger.WithError(err).Error("failed to report failed infrastructure install")
		return
	}

	c.logger.Debug("reporting metrics event infrastructure install failed")
	c.metricsReporter.ReportInfraInstallationFailed(ctx, errors.New(status.Description))
}

func (c *InstallController) reportAppInstallSucceeded(ctx context.Context, _, _ statemachine.State) {
	c.logger.Debug("reporting metrics event app install succeeded")
	c.metricsReporter.ReportAppInstallationSucceeded(ctx)
}

func (c *InstallController) reportAppInstallFailed(ctx context.Context, _, _ statemachine.State) {
	status, err := c.store.AppInstallStore().GetStatus()
	if err != nil {
		c.logger.WithError(fmt.Errorf("get status from app installation store: %w", err)).Error("failed to report failed app install")
		return
	}
	c.logger.Debug("reporting metrics event app install failed")
	c.metricsReporter.ReportAppInstallationFailed(ctx, errors.New(status.Description))
}

func (c *InstallController) reportAppPreflightsFailed(ctx context.Context, _, _ statemachine.State) {
	output, err := c.store.AppPreflightStore().GetOutput()
	if err != nil {
		c.logger.WithError(fmt.Errorf("get output from app preflight store: %w", err)).Error("failed to report app preflights failed")
		return
	}
	c.logger.Debug("reporting metrics event app preflights failed")
	c.metricsReporter.ReportAppPreflightsFailed(ctx, output)
}

func (c *InstallController) reportAppPreflightsBypassed(ctx context.Context, _, _ statemachine.State) {
	output, err := c.store.AppPreflightStore().GetOutput()
	if err != nil {
		c.logger.WithError(fmt.Errorf("get output from app preflight store: %w", err)).Error("failed to report app preflights bypassed")
		return
	}
	c.logger.Debug("reporting metrics event app preflights bypassed")
	c.metricsReporter.ReportAppPreflightsBypassed(ctx, output)
}

func (c *InstallController) reportAppPreflightsSucceeded(ctx context.Context, _, _ statemachine.State) {
	c.logger.Debug("reporting metrics event app preflights succeeded")
	c.metricsReporter.ReportAppPreflightsSucceeded(ctx)
}
