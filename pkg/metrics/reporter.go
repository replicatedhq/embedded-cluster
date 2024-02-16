package metrics

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

var clusterIDMut sync.Mutex
var clusterID *uuid.UUID

// BaseURL determines the base url to be used when sending metrics over.
func BaseURL(license *kotsv1beta1.License) string {
	if os.Getenv("EMBEDDED_CLUSTER_METRICS_BASEURL") != "" {
		return os.Getenv("EMBEDDED_CLUSTER_METRICS_BASEURL")
	}
	if license != nil && license.Spec.Endpoint != "" {
		return license.Spec.Endpoint
	}
	return "https://replicated.app"
}

// LicenseID returns the license id. If the license is nil, it returns an empty string.
func LicenseID(license *kotsv1beta1.License) string {
	if license != nil {
		return license.Spec.LicenseID
	}
	return ""
}

// License returns the parsed license. If something goes wrong, it returns nil.
func License(c *cli.Context) *kotsv1beta1.License {
	license, _ := helpers.ParseLicense(c.String("license"))
	return license
}

// ClusterID returns the cluster id. This is unique per 'install', but will be stored in the cluster and used by any future 'join' commands.
func ClusterID() uuid.UUID {
	clusterIDMut.Lock()
	defer clusterIDMut.Unlock()
	if clusterID != nil {
		return *clusterID
	}
	id := uuid.New()
	clusterID = &id
	return id
}

// ReportInstallationStarted reports that the installation has started.
func ReportInstallationStarted(ctx context.Context, license *kotsv1beta1.License) {
	Send(ctx, BaseURL(license), InstallationStarted{
		ClusterID:  ClusterID(),
		Version:    defaults.Version,
		Flags:      strings.Join(os.Args[1:], " "),
		BinaryName: defaults.BinaryName(),
		Type:       "centralized",
		LicenseID:  LicenseID(license),
	})
}

// ReportInstallationSucceeded reports that the installation has succeeded.
func ReportInstallationSucceeded(ctx context.Context, license *kotsv1beta1.License) {
	Send(ctx, BaseURL(license), InstallationSucceeded{ClusterID: ClusterID()})
}

// ReportInstallationFailed reports that the installation has failed.
func ReportInstallationFailed(ctx context.Context, license *kotsv1beta1.License, err error) {
	Send(ctx, BaseURL(license), InstallationFailed{ClusterID(), err.Error()})
}

// ReportJoinStarted reports that a join has started.
func ReportJoinStarted(ctx context.Context, baseURL string, clusterID uuid.UUID) {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Warnf("unable to get hostname: %s", err)
		hostname = "unknown"
	}
	Send(ctx, baseURL, JoinStarted{clusterID, hostname})
}

// ReportJoinSucceeded reports that a join has finished successfully.
func ReportJoinSucceeded(ctx context.Context, baseURL string, clusterID uuid.UUID) {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Warnf("unable to get hostname: %s", err)
		hostname = "unknown"
	}
	Send(ctx, baseURL, JoinSucceeded{clusterID, hostname})
}

// ReportJoinFailed reports that a join has failed.
func ReportJoinFailed(ctx context.Context, baseURL string, clusterID uuid.UUID, exterr error) {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Warnf("unable to get hostname: %s", err)
		hostname = "unknown"
	}
	Send(ctx, baseURL, JoinFailed{clusterID, hostname, exterr.Error()})
}

// ReportApplyStarted reports an InstallationStarted event.
func ReportApplyStarted(c *cli.Context) {
	ctx, cancel := context.WithTimeout(c.Context, 5*time.Second)
	defer cancel()
	ReportInstallationStarted(ctx, License(c))
}

// ReportApplyFinished reports an InstallationSucceeded or an InstallationFailed.
func ReportApplyFinished(c *cli.Context, err error) {
	ctx, cancel := context.WithTimeout(c.Context, 5*time.Second)
	defer cancel()
	if err != nil {
		ReportInstallationFailed(ctx, License(c), err)
		return
	}
	ReportInstallationSucceeded(ctx, License(c))
}
