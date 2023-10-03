package metrics

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/helmvm/pkg/addons/adminconsole"
	"github.com/replicatedhq/helmvm/pkg/defaults"
)

// isUpgrade holds globally if we are upgrading a cluster or installing a new one.
// This is used to decide which events to send and is determined only once at the
// beginning of the execution (see init()).
var isUpgrade bool

func init() {
	isUpgrade = defaults.IsUpgrade()
}

// LicenseID returns the embedded license id. If something goes wrong, it returns
// an empty string.
func LicenseID() string {
	var custom adminconsole.AdminConsoleCustomization
	if license, err := custom.License(); err == nil && license != nil {
		return license.Spec.LicenseID
	}
	return ""
}

// ClusterID returns the cluster id. It is read from from a local file (if this is
// a second attempt at installation or an upgrade) or a new one is generated and
// stored locally. TODO: this should be persisted in the cluster as part of a CRD
// managed by our operator, as we don't have an operator yet, we are storing it
// locally only.
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
func ReportInstallationSuceeded(ctx context.Context) {
	Send(ctx, InstallationSucceeded{ClusterID: ClusterID()})
}

// ReportInstallationFailed reports that the installation has failed.
func ReportInstallationFailed(ctx context.Context, err error) {
	Send(ctx, InstallationFailed{ClusterID(), err.Error()})
}

// ReportUpgradeStarted reports that the upgrade has started.
func ReportUpgradeStarted(ctx context.Context) {
	itype := "centralized"
	if defaults.DecentralizedInstall() {
		itype = "decentralized"
	}
	Send(ctx, UpgradeStarted{
		ClusterID:  ClusterID(),
		Version:    defaults.Version,
		Flags:      strings.Join(os.Args[1:], " "),
		BinaryName: defaults.BinaryName(),
		Type:       itype,
		LicenseID:  LicenseID(),
	})
}

// ReportUpgradeSucceeded reports that the upgrade has succeeded.
func ReportUpgradeSuceeded(ctx context.Context) {
	Send(ctx, UpgradeSucceeded{ClusterID: ClusterID()})
}

// ReportUpgradeFailed reports that the upgrade has failed.
func ReportUpgradeFailed(ctx context.Context, err error) {
	Send(ctx, UpgradeFailed{ClusterID(), err.Error()})
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
func ReportJoinFailed(ctx context.Context, clusterID uuid.UUID, err error) {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Warnf("unable to get hostname: %s", err)
		hostname = "unknown"
	}
	Send(ctx, JoinFailed{clusterID, hostname, err.Error()})
}

// ReportApplyStarted decides if we are going to report an InstallationStarted
// or an UpgradeStarted event and calls the appropriate function. If there has
// been provided a bundle directory through the command line it assumes this is
// a disconnected install and returns.
func ReportApplyStarted(c *cli.Context) {
	if c.String("bundle-dir") != "" {
		return
	}
	ctx, cancel := context.WithTimeout(c.Context, 5*time.Second)
	defer cancel()
	if isUpgrade {
		ReportUpgradeStarted(ctx)
		return
	}
	ReportInstallationStarted(ctx)
}

// ReportApplyFinished decides if we are going to report an InstallationSucceeded,
// an InstallationFailed, an UpgradeSucceeded, or an UpgradeFailed event and calls
// the appropriate function.
func ReportApplyFinished(c *cli.Context, err error) {
	if c.String("bundle-dir") != "" {
		return
	}
	ctx, cancel := context.WithTimeout(c.Context, 5*time.Second)
	defer cancel()
	if err != nil {
		if isUpgrade {
			ReportUpgradeFailed(ctx, err)
			return
		}
		ReportInstallationFailed(ctx, err)
		return
	}
	if isUpgrade {
		ReportUpgradeSuceeded(ctx)
		return
	}
	ReportInstallationSuceeded(ctx)
}
