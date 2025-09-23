package metrics

import (
	"context"
	"os"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
)

// ReporterInterface defines the interface for reporting various events in the system.
type ReporterInterface interface {
	// ReportInstallationStarted reports that the installation has started
	ReportInstallationStarted(ctx context.Context, licenseID string, appSlug string)

	// ReportInstallationSucceeded reports that the installation has succeeded
	ReportInstallationSucceeded(ctx context.Context)

	// ReportInstallationFailed reports that the installation has failed
	ReportInstallationFailed(ctx context.Context, err error)

	// ReportUpgradeStarted reports that an upgrade has started
	ReportUpgradeStarted(ctx context.Context, licenseID string, appSlug string, targetVersion string, initialVersion string)

	// ReportUpgradeSucceeded reports that an upgrade has succeeded
	ReportUpgradeSucceeded(ctx context.Context, targetVersion string, initialVersion string)

	// ReportUpgradeFailed reports that an upgrade has failed
	ReportUpgradeFailed(ctx context.Context, err error, targetVersion string, initialVersion string)

	// ReportJoinStarted reports that a join has started
	ReportJoinStarted(ctx context.Context)

	// ReportJoinSucceeded reports that a join has finished successfully
	ReportJoinSucceeded(ctx context.Context)

	// ReportJoinFailed reports that a join has failed
	ReportJoinFailed(ctx context.Context, err error)

	// ReportHostPreflightsFailed reports that the host preflights failed
	ReportHostPreflightsFailed(ctx context.Context, output *apitypes.PreflightsOutput)

	// ReportHostPreflightsBypassed reports that the host preflights failed but were bypassed
	ReportHostPreflightsBypassed(ctx context.Context, output *apitypes.PreflightsOutput)

	// ReportHostPreflightsSucceeded reports that the host preflights succeeded
	ReportHostPreflightsSucceeded(ctx context.Context)

	// ReportAppPreflightsFailed reports that the app preflights failed
	ReportAppPreflightsFailed(ctx context.Context, output *apitypes.PreflightsOutput)

	// ReportAppPreflightsBypassed reports that the app preflights failed but were bypassed
	ReportAppPreflightsBypassed(ctx context.Context, output *apitypes.PreflightsOutput)

	// ReportAppPreflightsSucceeded reports that the app preflights succeeded
	ReportAppPreflightsSucceeded(ctx context.Context)

	// ReportSignalAborted reports that a process was terminated by a signal
	ReportSignalAborted(ctx context.Context, signal os.Signal)
}

// Ensure Reporter implements ReporterInterface
var _ ReporterInterface = (*Reporter)(nil)
