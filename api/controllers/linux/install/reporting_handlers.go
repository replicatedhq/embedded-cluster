package install

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/statemachine"
)

func (c *InstallController) RegisterReportingHandlers() {
	c.stateMachine.RegisterEventHandler(StateSucceeded, c.reportInstallSucceeded)
	c.stateMachine.RegisterEventHandler(StateFailed, c.reportInstallFailed)
	c.stateMachine.RegisterEventHandler(StatePreflightsFailed, c.reportPreflightsFailed)
	c.stateMachine.RegisterEventHandler(StatePreflightsFailedBypassed, c.reportPreflightsBypassed)
}

func (c *InstallController) reportInstallSucceeded(ctx context.Context, _, _ statemachine.State) {
	c.metricsReporter.ReportInstallationSucceeded(ctx)
}

func (c *InstallController) reportInstallFailed(ctx context.Context, from, _ statemachine.State) {
	if from == StateInfrastructureInstalling {
		c.metricsReporter.ReportInstallationFailed(ctx, fmt.Errorf(c.install.Steps.Infra.Status.Description))
	}
}

func (c *InstallController) reportPreflightsFailed(ctx context.Context, _, _ statemachine.State) {
	c.metricsReporter.ReportPreflightsFailed(ctx, c.install.Steps.HostPreflight.Output)
}

func (c *InstallController) reportPreflightsBypassed(ctx context.Context, _, _ statemachine.State) {
	c.metricsReporter.ReportPreflightsFailed(ctx, c.install.Steps.HostPreflight.Output)
}
