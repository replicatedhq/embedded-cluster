package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
)

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

// Reporter provides methods for reporting various events.
type Reporter struct {
	version      string
	executionID  string
	baseURL      string
	clusterID    string
	hostname     string
	command      string
	commandFlags []string
}

// NewReporter creates a new Reporter with the given parameters.
func NewReporter(executionID string, baseURL string, clusterID string, command string, commandFlags []string) *Reporter {
	return &Reporter{
		version:      versions.Version,
		executionID:  executionID,
		baseURL:      baseURL,
		clusterID:    clusterID,
		hostname:     getHostname(),
		command:      command,
		commandFlags: redactFlags(commandFlags),
	}
}

// newGenericEvent creates a GenericEvent using the Reporter's fields.
func (r *Reporter) newGenericEvent(eventType string, reason string, isExitEvent bool) types.GenericEvent {
	return types.GenericEvent{
		ExecutionID:  r.executionID,
		ClusterID:    r.clusterID,
		Version:      r.version,
		Hostname:     r.hostname,
		EntryCommand: r.command,
		Flags:        strings.Join(r.commandFlags, " "),
		IsExitEvent:  isExitEvent,
		Reason:       reason,
		EventType:    eventType,
	}
}

// ReportInstallationStarted reports that the installation has started.
func (r *Reporter) ReportInstallationStarted(ctx context.Context, licenseID string, appSlug string) {
	rel := release.GetChannelRelease()
	appChannel, appVersion := "", ""
	if rel != nil {
		appChannel = rel.ChannelID
		appVersion = rel.VersionLabel
	}

	Send(ctx, r.baseURL, types.InstallationStarted{
		GenericEvent: r.newGenericEvent(types.EventTypeInstallationStarted, "", false),
		BinaryName:   appSlug,
		LegacyType:   "centralized",
		LicenseID:    licenseID,
		AppChannelID: appChannel,
		AppVersion:   appVersion,
	})
}

// ReportInfraInstallationSucceeded reports that the infrastructure installation has succeeded.
func (r *Reporter) ReportInfraInstallationSucceeded(ctx context.Context) {
	Send(ctx, r.baseURL, types.InstallationSucceeded{
		GenericEvent: r.newGenericEvent(types.EventTypeInfraInstallationSucceeded, "", true),
	})
}

// ReportInfraInstallationFailed reports that the infrastructure installation has failed.
func (r *Reporter) ReportInfraInstallationFailed(ctx context.Context, err error) {
	if errors.As(err, &ErrorNoFail{}) {
		return
	}
	Send(ctx, r.baseURL, types.InstallationFailed{
		GenericEvent: r.newGenericEvent(types.EventTypeInfraInstallationFailed, err.Error(), true),
	})
}

// ReportAppInstallationSucceeded reports that the app installation has succeeded.
func (r *Reporter) ReportAppInstallationSucceeded(ctx context.Context) {
	Send(ctx, r.baseURL, types.InstallationSucceeded{
		GenericEvent: r.newGenericEvent(types.EventTypeAppInstallationSucceeded, "", true),
	})
}

// ReportAppInstallationFailed reports that the app installation has failed.
func (r *Reporter) ReportAppInstallationFailed(ctx context.Context, err error) {
	if errors.As(err, &ErrorNoFail{}) {
		return
	}
	Send(ctx, r.baseURL, types.InstallationFailed{
		GenericEvent: r.newGenericEvent(types.EventTypeAppInstallationFailed, err.Error(), true),
	})
}

// ReportJoinStarted reports that a join has started.
func (r *Reporter) ReportJoinStarted(ctx context.Context) {
	Send(ctx, r.baseURL, types.JoinStarted{
		GenericEvent: r.newGenericEvent(types.EventTypeJoinStarted, "", false),
		NodeName:     getHostname(),
	})
}

// ReportJoinSucceeded reports that a join has finished successfully.
func (r *Reporter) ReportJoinSucceeded(ctx context.Context) {
	Send(ctx, r.baseURL, types.JoinSucceeded{
		GenericEvent: r.newGenericEvent(types.EventTypeJoinSucceeded, "", true),
		NodeName:     getHostname(),
	})
}

// ReportJoinFailed reports that a join has failed.
func (r *Reporter) ReportJoinFailed(ctx context.Context, err error) {
	if errors.As(err, &ErrorNoFail{}) {
		return
	}
	Send(ctx, r.baseURL, types.JoinFailed{
		GenericEvent: r.newGenericEvent(types.EventTypeJoinFailed, err.Error(), true),
		NodeName:     getHostname(),
	})
}

// ReportHostPreflightsFailed reports that the host preflights failed.
func (r *Reporter) ReportHostPreflightsFailed(ctx context.Context, output *apitypes.PreflightsOutput) {
	outputJSON, err := json.Marshal(output)
	if err != nil {
		logrus.Warnf("unable to marshal preflight output: %s", err)
		return
	}

	ev := types.PreflightsFailed{
		GenericEvent:    r.newGenericEvent(types.EventTypeHostPreflightsFailed, "", true),
		NodeName:        getHostname(),
		PreflightOutput: string(outputJSON),
	}
	Send(ctx, r.baseURL, ev)
}

// ReportHostPreflightsBypassed reports that the host preflights failed but were bypassed.
func (r *Reporter) ReportHostPreflightsBypassed(ctx context.Context, output *apitypes.PreflightsOutput) {
	outputJSON, err := json.Marshal(output)
	if err != nil {
		logrus.Warnf("unable to marshal preflight output: %s", err)
		return
	}

	ev := types.PreflightsBypassed{
		GenericEvent:    r.newGenericEvent(types.EventTypeHostPreflightsBypassed, "", false),
		NodeName:        getHostname(),
		PreflightOutput: string(outputJSON),
	}
	Send(ctx, r.baseURL, ev)
}

// ReportHostPreflightsSucceeded reports that the host preflights succeeded.

// ReportHostPreflightsSucceeded reports that the host preflights succeeded.
func (r *Reporter) ReportHostPreflightsSucceeded(ctx context.Context) {
	ev := types.PreflightsSucceeded{
		GenericEvent: r.newGenericEvent(types.EventTypeHostPreflightsSucceeded, "", true),
		NodeName:     getHostname(),
	}
	Send(ctx, r.baseURL, ev)
}

// ReportAppPreflightsFailed reports that the app preflights failed.
func (r *Reporter) ReportAppPreflightsFailed(ctx context.Context, output *apitypes.PreflightsOutput) {
	outputJSON, err := json.Marshal(output)
	if err != nil {
		logrus.Warnf("unable to marshal preflight output: %s", err)
		return
	}

	ev := types.PreflightsFailed{
		GenericEvent:    r.newGenericEvent(types.EventTypeAppPreflightsFailed, "", true),
		NodeName:        getHostname(),
		PreflightOutput: string(outputJSON),
	}
	Send(ctx, r.baseURL, ev)
}

// ReportAppPreflightsBypassed reports that the app preflights failed but were bypassed.
func (r *Reporter) ReportAppPreflightsBypassed(ctx context.Context, output *apitypes.PreflightsOutput) {
	outputJSON, err := json.Marshal(output)
	if err != nil {
		logrus.Warnf("unable to marshal preflight output: %s", err)
		return
	}

	ev := types.PreflightsBypassed{
		GenericEvent:    r.newGenericEvent(types.EventTypeAppPreflightsBypassed, "", false),
		NodeName:        getHostname(),
		PreflightOutput: string(outputJSON),
	}
	Send(ctx, r.baseURL, ev)
}

// ReportAppPreflightsSucceeded reports that the app preflights succeeded.
func (r *Reporter) ReportAppPreflightsSucceeded(ctx context.Context) {
	ev := types.PreflightsSucceeded{
		GenericEvent: r.newGenericEvent(types.EventTypeAppPreflightsSucceeded, "", true),
		NodeName:     getHostname(),
	}
	Send(ctx, r.baseURL, ev)
}

// ReportSignalAborted reports that a process was terminated by a signal.
func (r *Reporter) ReportSignalAborted(ctx context.Context, signal os.Signal) {
	ev := r.newGenericEvent(types.EventTypeSignalAborted, signal.String(), true)
	Send(ctx, r.baseURL, ev)
}

// getHostname returns the hostname or "unknown" if there's an error.
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Warnf("unable to get hostname: %s", err)
		hostname = "unknown"
	}
	return hostname
}
