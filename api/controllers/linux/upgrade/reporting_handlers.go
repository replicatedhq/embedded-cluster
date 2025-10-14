package upgrade

import (
	"context"
	"errors"
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

func (c *UpgradeController) reportUpgradeFailed(ctx context.Context, _, toState statemachine.State) {
	var status types.Status
	var err error

	switch toState {
	case states.StateInfrastructureUpgradeFailed:
		status, err = c.store.LinuxInfraStore().GetStatus()
		if err != nil {
			err = fmt.Errorf("get status from infra store: %w", err)
		}
	case states.StateAppUpgradeFailed:
		status, err = c.store.AppUpgradeStore().GetStatus()
		if err != nil {
			err = fmt.Errorf("get status from app upgrade store: %w", err)
		}
	}
	if err != nil {
		c.logger.WithError(err).Error("failed to report failed upgrade")
		return
	}

	c.logger.Info("Reporting metrics event upgrade failed")
	c.metricsReporter.ReportUpgradeFailed(ctx, errors.New(status.Description), c.targetVersion, c.initialVersion)
}

func (c *UpgradeController) reportUpgradeSucceeded(ctx context.Context, _, _ statemachine.State) {
	c.logger.Info("Reporting metrics event upgrade succeeded")
	c.metricsReporter.ReportUpgradeSucceeded(ctx, c.targetVersion, c.initialVersion)
}

func (c *UpgradeController) reportAppPreflightsFailed(ctx context.Context, _, _ statemachine.State) {
	output, err := c.store.AppPreflightStore().GetOutput()
	if err != nil {
		err = fmt.Errorf("get output from app preflight store: %w", err)
		c.logger.WithError(err).Error("failed to report app preflights failed")
		return
	}
	c.logger.Info("Reporting metrics event app preflights failed")
	c.metricsReporter.ReportAppPreflightsFailed(ctx, output)
}

func (c *UpgradeController) reportAppPreflightsBypassed(ctx context.Context, _, _ statemachine.State) {
	output, err := c.store.AppPreflightStore().GetOutput()
	if err != nil {
		err = fmt.Errorf("get output from app preflight store: %w", err)
		c.logger.WithError(err).Error("failed to report app preflights bypassed")
		return
	}
	c.logger.Info("Reporting metrics event app preflights bypassed")
	c.metricsReporter.ReportAppPreflightsBypassed(ctx, output)
}

func (c *UpgradeController) reportAppPreflightsSucceeded(ctx context.Context, _, _ statemachine.State) {
	c.logger.Info("Reporting metrics event app preflights succeeded")
	c.metricsReporter.ReportAppPreflightsSucceeded(ctx)
}
