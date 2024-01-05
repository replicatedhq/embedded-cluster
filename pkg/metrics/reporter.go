package metrics

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/customization"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

// LicenseID returns the embedded license id. If something goes wrong, it returns
// an empty string.
func LicenseID() string {
	if license, err := customization.GetLicense(); err == nil && license != nil {
		return license.Spec.LicenseID
	}
	return ""
}

// ClusterID returns the cluster id. It is read from from a local file (if this is
// a second attempt at installation or an upgrade) or a new one is generated and
// stored locally.
func ClusterID() uuid.UUID {
	fpath := defaults.PathToConfig(".cluster-id")
	if _, err := os.Stat(fpath); err == nil {
		data, err := os.ReadFile(fpath)
		if err != nil {
			logrus.Warnf("unable to read cluster id from %s: %s", fpath, err)
			return uuid.New()
		}
		id, err := uuid.Parse(string(data))
		if err != nil {
			logrus.Warnf("unable to parse cluster id from %s: %s", fpath, err)
			return uuid.New()
		}
		return id
	}
	id := uuid.New()
	if err := os.WriteFile(fpath, []byte(id.String()), 0644); err != nil {
		logrus.Warnf("unable to write cluster id to %s: %s", fpath, err)
	}
	return id
}

// ReportInstallationStarted reports that the installation has started.
func ReportInstallationStarted(ctx context.Context) {
	Send(ctx, InstallationStarted{
		ClusterID:  ClusterID(),
		Version:    defaults.Version,
		Flags:      strings.Join(os.Args[1:], " "),
		BinaryName: defaults.BinaryName(),
		Type:       "centralized",
		LicenseID:  LicenseID(),
	})
}

// ReportInstallationSucceeded reports that the installation has succeeded.
func ReportInstallationSucceeded(ctx context.Context) {
	Send(ctx, InstallationSucceeded{ClusterID: ClusterID()})
}

// ReportInstallationFailed reports that the installation has failed.
func ReportInstallationFailed(ctx context.Context, err error) {
	Send(ctx, InstallationFailed{ClusterID(), err.Error()})
}

// ReportJoinStarted reports that a join has started.
func ReportJoinStarted(ctx context.Context, clusterID uuid.UUID) {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Warnf("unable to get hostname: %s", err)
		hostname = "unknown"
	}
	Send(ctx, JoinStarted{clusterID, hostname})
}

// ReportJoinSucceeded reports that a join has finished successfully.
func ReportJoinSucceeded(ctx context.Context, clusterID uuid.UUID) {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Warnf("unable to get hostname: %s", err)
		hostname = "unknown"
	}
	Send(ctx, JoinSucceeded{clusterID, hostname})
}

// ReportJoinFailed reports that a join has failed.
func ReportJoinFailed(ctx context.Context, clusterID uuid.UUID, exterr error) {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Warnf("unable to get hostname: %s", err)
		hostname = "unknown"
	}
	Send(ctx, JoinFailed{clusterID, hostname, exterr.Error()})
}

// ReportApplyStarted reports an InstallationStarted event.
func ReportApplyStarted(c *cli.Context) {
	ctx, cancel := context.WithTimeout(c.Context, 5*time.Second)
	defer cancel()
	ReportInstallationStarted(ctx)
}

// ReportApplyFinished reports an InstallationSucceeded or an InstallationFailed.
func ReportApplyFinished(c *cli.Context, err error) {
	ctx, cancel := context.WithTimeout(c.Context, 5*time.Second)
	defer cancel()
	if err != nil {
		ReportInstallationFailed(ctx, err)
		return
	}
	ReportInstallationSucceeded(ctx)
}
