package cli

import (
	"context"
	"os"

	"github.com/google/uuid"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	preflightstypes "github.com/replicatedhq/embedded-cluster/pkg/preflights/types"
	"github.com/spf13/pflag"
)

type InstallReporter struct {
	reporter  *metrics.Reporter
	licenseID string
	appSlug   string
}

func NewInstallReporter(baseURL string, clusterID uuid.UUID, cmd string, args []string, licenseID string, appSlug string) *InstallReporter {
	executionID := uuid.New().String()
	reporter := metrics.NewReporter(executionID, baseURL, clusterID, cmd, args)
	return &InstallReporter{
		licenseID: licenseID,
		appSlug:   appSlug,
		reporter:  reporter,
	}
}

func (r *InstallReporter) ReportInstallationStarted(ctx context.Context) {
	r.reporter.ReportInstallationStarted(ctx, r.licenseID, r.appSlug)
}

func (r *InstallReporter) ReportInstallationSucceeded(ctx context.Context) {
	r.reporter.ReportInstallationSucceeded(ctx)
}

func (r *InstallReporter) ReportInstallationFailed(ctx context.Context, err error) {
	r.reporter.ReportInstallationFailed(ctx, err)
}

func (r *InstallReporter) ReportPreflightsFailed(ctx context.Context, output preflightstypes.Output) {
	r.reporter.ReportPreflightsFailed(ctx, output)
}

func (r *InstallReporter) ReportPreflightsBypassed(ctx context.Context, output preflightstypes.Output) {
	r.reporter.ReportPreflightsBypassed(ctx, output)
}

func (r *InstallReporter) ReportSignalAborted(ctx context.Context, sig os.Signal) {
	r.reporter.ReportSignalAborted(ctx, sig)
}

type JoinReporter struct {
	reporter *metrics.Reporter
}

func NewJoinReporter(baseURL string, clusterID uuid.UUID, cmd string, flags []string) *JoinReporter {
	executionID := uuid.New().String()
	reporter := metrics.NewReporter(executionID, baseURL, clusterID, cmd, flags)
	return &JoinReporter{
		reporter: reporter,
	}
}

func (r *JoinReporter) ReportJoinStarted(ctx context.Context) {
	r.reporter.ReportJoinStarted(ctx)
}

func (r *JoinReporter) ReportJoinSucceeded(ctx context.Context) {
	r.reporter.ReportJoinSucceeded(ctx)
}

func (r *JoinReporter) ReportJoinFailed(ctx context.Context, err error) {
	r.reporter.ReportJoinFailed(ctx, err)
}

func (r *JoinReporter) ReportPreflightsFailed(ctx context.Context, output preflightstypes.Output) {
	r.reporter.ReportPreflightsFailed(ctx, output)
}

func (r *JoinReporter) ReportPreflightsBypassed(ctx context.Context, output preflightstypes.Output) {
	r.reporter.ReportPreflightsBypassed(ctx, output)
}

func (r *JoinReporter) ReportSignalAborted(ctx context.Context, sig os.Signal) {
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
