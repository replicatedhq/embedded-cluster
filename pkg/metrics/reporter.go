package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics/types"
	preflightstypes "github.com/replicatedhq/embedded-cluster/pkg/preflights/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

var clusterIDMut sync.Mutex
var clusterID *uuid.UUID

// ErrorNoFail is an error that is excluded from metrics failures.
type ErrorNoFail struct {
	Err error
}

func NewErrorNoFail(err error) ErrorNoFail {
	return ErrorNoFail{
		Err: err,
	}
}

func (e ErrorNoFail) Error() string {
	return e.Err.Error()
}

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
func License(licenseFlag string) *kotsv1beta1.License {
	license, _ := helpers.ParseLicense(licenseFlag)
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

func SetClusterID(id uuid.UUID) {
	clusterIDMut.Lock()
	defer clusterIDMut.Unlock()
	clusterID = &id
}

// ReportInstallationStarted reports that the installation has started.
func ReportInstallationStarted(ctx context.Context, license *kotsv1beta1.License, clusterID uuid.UUID) {
	rel, _ := release.GetChannelRelease()
	appChannel, appVersion := "", ""
	if rel != nil {
		appChannel = rel.ChannelID
		appVersion = rel.VersionLabel
	}

	Send(ctx, BaseURL(license), types.InstallationStarted{
		ClusterID:    clusterID,
		Version:      versions.Version,
		Flags:        strings.Join(redactFlags(os.Args[1:]), " "),
		BinaryName:   runtimeconfig.BinaryName(),
		Type:         "centralized",
		LicenseID:    LicenseID(license),
		AppChannelID: appChannel,
		AppVersion:   appVersion,
	})
}

// ReportInstallationSucceeded reports that the installation has succeeded.
func ReportInstallationSucceeded(ctx context.Context, license *kotsv1beta1.License, clusterID uuid.UUID) {
	Send(ctx, BaseURL(license), types.InstallationSucceeded{ClusterID: clusterID, Version: versions.Version})
}

// ReportInstallationFailed reports that the installation has failed.
func ReportInstallationFailed(ctx context.Context, license *kotsv1beta1.License, clusterID uuid.UUID, err error) {
	if errors.As(err, &ErrorNoFail{}) {
		return
	}
	Send(ctx, BaseURL(license), types.InstallationFailed{
		ClusterID: clusterID,
		Version:   versions.Version,
		Reason:    err.Error(),
	})
}

// ReportJoinStarted reports that a join has started.
func ReportJoinStarted(ctx context.Context, baseURL string, clusterID uuid.UUID) {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Warnf("unable to get hostname: %s", err)
		hostname = "unknown"
	}
	Send(ctx, baseURL, types.JoinStarted{
		ClusterID: clusterID,
		Version:   versions.Version,
		NodeName:  hostname,
	})
}

// ReportJoinSucceeded reports that a join has finished successfully.
func ReportJoinSucceeded(ctx context.Context, baseURL string, clusterID uuid.UUID) {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Warnf("unable to get hostname: %s", err)
		hostname = "unknown"
	}
	Send(ctx, baseURL, types.JoinSucceeded{
		ClusterID: clusterID,
		Version:   versions.Version,
		NodeName:  hostname,
	})
}

// ReportJoinFailed reports that a join has failed.
func ReportJoinFailed(ctx context.Context, baseURL string, clusterID uuid.UUID, err error) {
	if errors.As(err, &ErrorNoFail{}) {
		return
	}
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	Send(ctx, baseURL, types.JoinFailed{
		ClusterID: clusterID,
		Version:   versions.Version,
		NodeName:  hostname,
		Reason:    err.Error(),
	})
}

// ReportApplyStarted reports an InstallationStarted event.
func ReportApplyStarted(ctx context.Context, licenseFlag string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ReportInstallationStarted(ctx, License(licenseFlag), ClusterID())
}

// ReportApplyFinished reports an InstallationSucceeded or an InstallationFailed.
func ReportApplyFinished(ctx context.Context, licenseFlag string, license *kotsv1beta1.License, err error) {
	if license == nil && licenseFlag != "" {
		license = License(licenseFlag)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err != nil {
		ReportInstallationFailed(ctx, license, ClusterID(), err)
		return
	}
	ReportInstallationSucceeded(ctx, license, ClusterID())
}

// ReportPreflightsFailed reports that the preflights failed but were bypassed.
func ReportPreflightsFailed(ctx context.Context, baseURL string, clusterID uuid.UUID, output preflightstypes.Output, bypassed bool, entryCommand string) {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Warnf("unable to get hostname: %s", err)
		hostname = "unknown"
	}

	eventType := "PreflightsFailed"
	if bypassed {
		eventType = "PreflightsBypassed"
	}

	outputJSON, err := json.Marshal(output)
	if err != nil {
		logrus.Warnf("unable to marshal preflight output: %s", err)
		return
	}

	ev := types.PreflightsFailed{
		ClusterID:       clusterID,
		Version:         versions.Version,
		NodeName:        hostname,
		PreflightOutput: string(outputJSON),
		EventType:       eventType,
		EntryCommand:    entryCommand,
	}
	go Send(ctx, baseURL, ev)
}
