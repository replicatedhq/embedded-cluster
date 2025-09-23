package cli

import (
	"context"
	"os"

	"github.com/google/uuid"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/spf13/pflag"
)

type installReporter struct {
	reporter  metrics.ReporterInterface
	licenseID string
	appSlug   string
}

func newInstallReporter(baseURL string, cmd string, args []string, licenseID string, clusterID string, appSlug string) *installReporter {
	executionID := uuid.New().String()
	reporter := metrics.NewReporter(executionID, baseURL, clusterID, cmd, args)
	return &installReporter{
		licenseID: licenseID,
		appSlug:   appSlug,
		reporter:  reporter,
	}
}

func (r *installReporter) ReportInstallationStarted(ctx context.Context) {
	r.reporter.ReportInstallationStarted(ctx, r.licenseID, r.appSlug)
}

func (r *installReporter) ReportInstallationSucceeded(ctx context.Context) {
	r.reporter.ReportInstallationSucceeded(ctx)
}

func (r *installReporter) ReportInstallationFailed(ctx context.Context, err error) {
	r.reporter.ReportInstallationFailed(ctx, err)
}

func (r *installReporter) ReportPreflightsFailed(ctx context.Context, output *apitypes.PreflightsOutput) {
	r.reporter.ReportHostPreflightsFailed(ctx, output)
}

func (r *installReporter) ReportPreflightsBypassed(ctx context.Context, output *apitypes.PreflightsOutput) {
	r.reporter.ReportHostPreflightsBypassed(ctx, output)
}

func (r *installReporter) ReportSignalAborted(ctx context.Context, sig os.Signal) {
	r.reporter.ReportSignalAborted(ctx, sig)
}

type upgradeReporter struct {
	reporter       metrics.ReporterInterface
	licenseID      string
	appSlug        string
	targetVersion  string
	initialVersion string
}

func newUpgradeReporter(baseURL string, cmd string, flags []string, licenseID string, clusterID string, appSlug string, targetVersion string, initialVersion string) *upgradeReporter {
	executionID := uuid.New().String()
	reporter := metrics.NewReporter(executionID, baseURL, clusterID, cmd, flags)
	return &upgradeReporter{
		reporter:       reporter,
		licenseID:      licenseID,
		appSlug:        appSlug,
		targetVersion:  targetVersion,
		initialVersion: initialVersion,
	}
}

func (ur *upgradeReporter) ReportUpgradeStarted(ctx context.Context) {
	ur.reporter.ReportUpgradeStarted(ctx, ur.licenseID, ur.appSlug, ur.targetVersion, ur.initialVersion)
}

func (ur *upgradeReporter) ReportUpgradeSucceeded(ctx context.Context) {
	ur.reporter.ReportUpgradeSucceeded(ctx, ur.targetVersion, ur.initialVersion)
}

func (ur *upgradeReporter) ReportUpgradeFailed(ctx context.Context, err error) {
	ur.reporter.ReportUpgradeFailed(ctx, err, ur.targetVersion, ur.initialVersion)
}

func (ur *upgradeReporter) ReportPreflightsFailed(ctx context.Context, output *apitypes.PreflightsOutput) {
	ur.reporter.ReportHostPreflightsFailed(ctx, output)
}

func (ur *upgradeReporter) ReportPreflightsBypassed(ctx context.Context, output *apitypes.PreflightsOutput) {
	ur.reporter.ReportHostPreflightsBypassed(ctx, output)
}

func (ur *upgradeReporter) ReportSignalAborted(ctx context.Context, sig os.Signal) {
	ur.reporter.ReportSignalAborted(ctx, sig)
}

type joinReporter struct {
	reporter metrics.ReporterInterface
}

func newJoinReporter(baseURL string, clusterID string, cmd string, flags []string) *joinReporter {
	executionID := uuid.New().String()
	reporter := metrics.NewReporter(executionID, baseURL, clusterID, cmd, flags)
	return &joinReporter{
		reporter: reporter,
	}
}

func (r *joinReporter) ReportJoinStarted(ctx context.Context) {
	r.reporter.ReportJoinStarted(ctx)
}

func (r *joinReporter) ReportJoinSucceeded(ctx context.Context) {
	r.reporter.ReportJoinSucceeded(ctx)
}

func (r *joinReporter) ReportJoinFailed(ctx context.Context, err error) {
	r.reporter.ReportJoinFailed(ctx, err)
}

func (r *joinReporter) ReportPreflightsFailed(ctx context.Context, output *apitypes.PreflightsOutput) {
	r.reporter.ReportHostPreflightsFailed(ctx, output)
}

func (r *joinReporter) ReportPreflightsBypassed(ctx context.Context, output *apitypes.PreflightsOutput) {
	r.reporter.ReportHostPreflightsBypassed(ctx, output)
}

func (r *joinReporter) ReportSignalAborted(ctx context.Context, sig os.Signal) {
	r.reporter.ReportSignalAborted(ctx, sig)
}

// flagsToStringSlice converts a pflag.FlagSet's flags into a string slice for metrics reporting.
// It only includes flags that have been explicitly set by the user.
func flagsToStringSlice(flags *pflag.FlagSet) []string {
	var result []string
	flags.VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			if f.Value.Type() == "bool" {
				// For boolean flags, check the actual value
				if f.Value.String() == "true" {
					result = append(result, "--"+f.Name)
				} else {
					result = append(result, "--"+f.Name+"=false")
				}
			} else {
				result = append(result, "--"+f.Name, f.Value.String())
			}
		}
	})
	return result
}
