package cli

import (
	"context"
	"os"

	"github.com/google/uuid"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	preflightstypes "github.com/replicatedhq/embedded-cluster/pkg/preflights/types"
)

type InstallReporter struct {
	baseURL   string
	licenseID string
	clusterID uuid.UUID
	cmd       string
}

func NewInstallReporter(baseURL string, licenseID string, clusterID uuid.UUID, cmd string) *InstallReporter {
	return &InstallReporter{
		baseURL:   baseURL,
		licenseID: licenseID,
		clusterID: clusterID,
		cmd:       cmd,
	}
}

func (r *InstallReporter) ReportInstallationStarted(ctx context.Context) {
	metrics.ReportInstallationStarted(ctx, r.baseURL, r.licenseID, r.clusterID)
}

func (r *InstallReporter) ReportInstallationSucceeded(ctx context.Context) {
	metrics.ReportInstallationSucceeded(ctx, r.baseURL, r.clusterID)
}

func (r *InstallReporter) ReportInstallationFailed(ctx context.Context, err error) {
	metrics.ReportInstallationFailed(ctx, r.baseURL, r.clusterID, err)
}

func (r *InstallReporter) ReportPreflightsFailed(ctx context.Context, output preflightstypes.Output, bypassed bool) {
	metrics.ReportPreflightsFailed(ctx, r.baseURL, r.clusterID, output, bypassed, r.cmd)
}

func (r *InstallReporter) ReportSignalAborted(ctx context.Context, signal os.Signal) {
	metrics.ReportSignalAborted(ctx, r.baseURL, r.clusterID, signal, r.cmd)
}

type JoinReporter struct {
	baseURL   string
	clusterID uuid.UUID
	cmd       string
}

func NewJoinReporter(baseURL string, clusterID uuid.UUID, cmd string) *JoinReporter {
	return &JoinReporter{
		baseURL:   baseURL,
		clusterID: clusterID,
		cmd:       cmd,
	}
}

func (r *JoinReporter) ReportJoinStarted(ctx context.Context) {
	metrics.ReportJoinStarted(ctx, r.baseURL, r.clusterID)
}

func (r *JoinReporter) ReportJoinSucceeded(ctx context.Context) {
	metrics.ReportJoinSucceeded(ctx, r.baseURL, r.clusterID)
}

func (r *JoinReporter) ReportJoinFailed(ctx context.Context, err error) {
	metrics.ReportJoinFailed(ctx, r.baseURL, r.clusterID, err)
}

func (r *JoinReporter) ReportPreflightsFailed(ctx context.Context, output preflightstypes.Output, bypassed bool) {
	metrics.ReportPreflightsFailed(ctx, r.baseURL, r.clusterID, output, bypassed, r.cmd)
}

func (r *JoinReporter) ReportSignalAborted(ctx context.Context, signal os.Signal) {
	metrics.ReportSignalAborted(ctx, r.baseURL, r.clusterID, signal, r.cmd)
}
