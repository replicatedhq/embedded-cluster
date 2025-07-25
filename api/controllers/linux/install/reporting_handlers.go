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
	c.stateMachine.RegisterEventHandler(states.StateSucceeded, c.reportInstallSucceeded)
	c.stateMachine.RegisterEventHandler(states.StateInfrastructureInstallFailed, c.reportInstallFailed)
	c.stateMachine.RegisterEventHandler(states.StateHostConfigurationFailed, c.reportInstallFailed)
	c.stateMachine.RegisterEventHandler(states.StateInstallationConfigurationFailed, c.reportInstallFailed)
	c.stateMachine.RegisterEventHandler(states.StatePreflightsFailed, c.reportPreflightsFailed)
	c.stateMachine.RegisterEventHandler(states.StatePreflightsFailedBypassed, c.reportPreflightsBypassed)
}

func (c *InstallController) reportInstallSucceeded(ctx context.Context, _, _ statemachine.State) {
	c.metricsReporter.ReportInstallationSucceeded(ctx)
}

func (c *InstallController) reportInstallFailed(ctx context.Context, _, toState statemachine.State) {
	var status types.Status
	var err error

	switch toState {
	case states.StateInstallationConfigurationFailed:
		status, err = c.store.LinuxInstallationStore().GetStatus()
		if err != nil {
			err = fmt.Errorf("failed to get status from installation store: %w", err)
		}
	case states.StateHostConfigurationFailed:
		status, err = c.store.LinuxInstallationStore().GetStatus()
		if err != nil {
			err = fmt.Errorf("failed to get status from installation store: %w", err)
		}
	case states.StateInfrastructureInstallFailed:
		status, err = c.store.LinuxInfraStore().GetStatus()
		if err != nil {
			err = fmt.Errorf("failed to get status from infra store: %w", err)
		}
	}
	if err != nil {
		c.logger.WithError(err).Error("failed to report failled install")
		return
	}
	c.metricsReporter.ReportInstallationFailed(ctx, errors.New(status.Description))
}

func (c *InstallController) reportPreflightsFailed(ctx context.Context, _, _ statemachine.State) {
	output, err := c.store.LinuxPreflightStore().GetOutput()
	if err != nil {
		c.logger.WithError(fmt.Errorf("failed to get output from preflight store: %w", err)).Error("failed to report preflights failed")
		return
	}
	c.metricsReporter.ReportPreflightsFailed(ctx, output)
}

func (c *InstallController) reportPreflightsBypassed(ctx context.Context, _, _ statemachine.State) {
	output, err := c.store.LinuxPreflightStore().GetOutput()
	if err != nil {
		c.logger.WithError(fmt.Errorf("failed to get output from preflight store: %w", err)).Error("failed to report preflights bypassed")
		return
	}
	c.metricsReporter.ReportPreflightsBypassed(ctx, output)
}
