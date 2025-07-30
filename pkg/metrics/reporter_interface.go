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

	// ReportJoinStarted reports that a join has started
	ReportJoinStarted(ctx context.Context)

	// ReportJoinSucceeded reports that a join has finished successfully
	ReportJoinSucceeded(ctx context.Context)

	// ReportJoinFailed reports that a join has failed
	ReportJoinFailed(ctx context.Context, err error)

	// ReportPreflightsFailed reports that the preflights failed
	ReportPreflightsFailed(ctx context.Context, output *apitypes.PreflightsOutput)

	// ReportPreflightsBypassed reports that the preflights failed but were bypassed
	ReportPreflightsBypassed(ctx context.Context, output *apitypes.PreflightsOutput)

	// ReportSignalAborted reports that a process was terminated by a signal
	ReportSignalAborted(ctx context.Context, signal os.Signal)
}

// Ensure Reporter implements ReporterInterface
var _ ReporterInterface = (*Reporter)(nil)
