package metrics

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/google/uuid"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics/types"
	preflightstypes "github.com/replicatedhq/embedded-cluster/pkg/preflights/types"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/versions"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/sirupsen/logrus"
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

// Reporter provides methods for reporting various events.
type Reporter struct {
	executionID  string
	baseURL      string
	clusterID    uuid.UUID
	command      string
	commandFlags []string
	didExit      bool
}

// NewReporter creates a new Reporter with the given parameters.
func NewReporter(executionID string, baseURL string, clusterID uuid.UUID, command string, commandFlags []string) *Reporter {
	return &Reporter{
		executionID:  executionID,
		baseURL:      baseURL,
		clusterID:    clusterID,
		command:      command,
		commandFlags: redactFlags(commandFlags),
	}
}

// newBaseEvent creates a BaseEvent using the Reporter's fields.
func (r *Reporter) newBaseEvent(isExitEvent bool) types.BaseEvent {
	return types.NewBaseEvent(r.executionID, r.clusterID, versions.Version, r.command, r.commandFlags, isExitEvent)
}

// ReportInstallationStarted reports that the installation has started.
func (r *Reporter) ReportInstallationStarted(ctx context.Context, licenseID string) {
	rel := release.GetChannelRelease()
	appChannel, appVersion := "", ""
	if rel != nil {
		appChannel = rel.ChannelID
		appVersion = rel.VersionLabel
	}

	Send(ctx, r.baseURL, types.InstallationStarted{
		BaseEvent:    r.newBaseEvent(false),
		BinaryName:   runtimeconfig.BinaryName(),
		Type:         "centralized",
		LicenseID:    licenseID,
		AppChannelID: appChannel,
		AppVersion:   appVersion,
	})
}

// ReportInstallationSucceeded reports that the installation has succeeded.
func (r *Reporter) ReportInstallationSucceeded(ctx context.Context) {
	Send(ctx, r.baseURL, types.InstallationSucceeded{
		BaseEvent: r.newBaseEvent(true),
	})
}

// ReportInstallationFailed reports that the installation has failed.
func (r *Reporter) ReportInstallationFailed(ctx context.Context, err error) {
	if errors.As(err, &ErrorNoFail{}) {
		return
	}
	Send(ctx, r.baseURL, types.InstallationFailed{
		BaseEvent: r.newBaseEvent(true),
		Reason:    err.Error(),
	})
}

// ReportJoinStarted reports that a join has started.
func (r *Reporter) ReportJoinStarted(ctx context.Context) {
	Send(ctx, r.baseURL, types.JoinStarted{
		BaseEvent: r.newBaseEvent(false),
		NodeName:  getHostname(),
	})
}

// ReportJoinSucceeded reports that a join has finished successfully.
func (r *Reporter) ReportJoinSucceeded(ctx context.Context) {
	Send(ctx, r.baseURL, types.JoinSucceeded{
		BaseEvent: r.newBaseEvent(true),
		NodeName:  getHostname(),
	})
}

// ReportJoinFailed reports that a join has failed.
func (r *Reporter) ReportJoinFailed(ctx context.Context, err error) {
	if errors.As(err, &ErrorNoFail{}) {
		return
	}
	Send(ctx, r.baseURL, types.JoinFailed{
		BaseEvent: r.newBaseEvent(true),
		NodeName:  getHostname(),
		Reason:    err.Error(),
	})
}

// ReportPreflightsFailed reports that the preflights failed.
func (r *Reporter) ReportPreflightsFailed(ctx context.Context, output preflightstypes.Output) {
	outputJSON, err := json.Marshal(output)
	if err != nil {
		logrus.Warnf("unable to marshal preflight output: %s", err)
		return
	}

	ev := types.PreflightsFailed{
		BaseEvent:       r.newBaseEvent(true),
		NodeName:        getHostname(),
		PreflightOutput: string(outputJSON),
		EventType:       "PreflightsFailed",
	}
	go Send(ctx, r.baseURL, ev)
}

// ReportPreflightsBypassed reports that the preflights failed but were bypassed.
func (r *Reporter) ReportPreflightsBypassed(ctx context.Context, output preflightstypes.Output) {
	outputJSON, err := json.Marshal(output)
	if err != nil {
		logrus.Warnf("unable to marshal preflight output: %s", err)
		return
	}

	ev := types.PreflightsBypassed{
		BaseEvent:       r.newBaseEvent(false),
		NodeName:        getHostname(),
		PreflightOutput: string(outputJSON),
		EventType:       "PreflightsBypassed",
	}
	go Send(ctx, r.baseURL, ev)
}

// ReportSignalAborted reports that a process was terminated by a signal.
func (r *Reporter) ReportSignalAborted(ctx context.Context, signal os.Signal) {
	ev := types.SignalAborted{
		BaseEvent:  r.newBaseEvent(true),
		NodeName:   getHostname(),
		SignalName: signal.String(),
		EventType:  "SignalAborted",
	}
	go Send(ctx, r.baseURL, ev)
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
