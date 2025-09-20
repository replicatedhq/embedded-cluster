package types

// Event type constants
const (
	EventTypeInstallationStarted     = "InstallationStarted"
	EventTypeInstallationSucceeded   = "InstallationSucceeded"
	EventTypeInstallationFailed      = "InstallationFailed"
	EventTypeUpgradeStarted          = "UpgradeStarted"
	EventTypeUpgradeSucceeded        = "UpgradeSucceeded"
	EventTypeUpgradeFailed           = "UpgradeFailed"
	EventTypeJoinStarted             = "JoinStarted"
	EventTypeJoinSucceeded           = "JoinSucceeded"
	EventTypeJoinFailed              = "JoinFailed"
	EventTypeHostPreflightsFailed    = "PreflightsFailed"    // event name remains the same for backwards compatibility
	EventTypeHostPreflightsBypassed  = "PreflightsBypassed"  // event name remains the same for backwards compatibility
	EventTypeHostPreflightsSucceeded = "PreflightsSucceeded" // event name remains the same for backwards compatibility
	EventTypeAppPreflightsFailed     = "AppPreflightsFailed"
	EventTypeAppPreflightsBypassed   = "AppPreflightsBypassed"
	EventTypeAppPreflightsSucceeded  = "AppPreflightsSucceeded"
	EventTypeSignalAborted           = "SignalAborted"
)

// Event is implemented by all events. Title returns a string that identifies the
// event type.
type Event interface {
	Title() string
	Type() string
}

// GenericEvent contains common fields shared by all events.
type GenericEvent struct {
	// ExecutionID is a unique identifier for the current execution
	ExecutionID string `json:"executionID"`
	// ClusterID is the unique identifier of the cluster
	ClusterID string `json:"clusterID"`
	// Version is the version of the embedded-cluster software
	Version string `json:"version"`
	// Hostname is the hostname of the server
	Hostname string `json:"hostname"`
	// EntryCommand is the main command being executed (e.g. "install", "join")
	EntryCommand string `json:"entryCommand"`
	// Flags contains the command-line flags passed to the command
	Flags string `json:"flags"`
	// EventType is the type of event
	EventType string `json:"eventType"`
	// Reason is the reason for the event
	Reason string `json:"reason"`
	// IsExitEvent is true if the command indicates this is the end of the execution
	IsExitEvent bool `json:"isExitEvent"`
}

// NewGenericEvent creates a new GenericEvent with the specified name.
func NewGenericEvent(eventName string, executionID string, clusterID string, version string, hostname string,
	entryCommand string, flags string, reason string, isExitEvent bool) GenericEvent {
	return GenericEvent{
		ExecutionID:  executionID,
		ClusterID:    clusterID,
		Version:      version,
		Hostname:     hostname,
		EntryCommand: entryCommand,
		Flags:        flags,
		EventType:    eventName,
		Reason:       reason,
		IsExitEvent:  isExitEvent,
	}
}

// Title returns the name of the event.
func (e GenericEvent) Title() string {
	return "GenericEvent"
}

// Type returns the type of the event.
func (e GenericEvent) Type() string {
	return e.EventType
}

// InstallationStarted event is send back home when the installation starts.
type InstallationStarted struct {
	GenericEvent `json:",inline"`
	BinaryName   string `json:"binaryName"`
	LegacyType   string `json:"type"` // TODO(ethan): confirm if we can remove this
	LicenseID    string `json:"licenseID"`
	AppChannelID string `json:"appChannelID"`
	AppVersion   string `json:"appVersion"`
}

// Title returns the name of the event.
func (e InstallationStarted) Title() string {
	return EventTypeInstallationStarted
}

// Type returns the type of the event.
func (e InstallationStarted) Type() string {
	return EventTypeInstallationStarted
}

// InstallationSucceeded event is send back home when the installation finishes.
type InstallationSucceeded struct {
	GenericEvent `json:",inline"`
}

// InstallationFailed event is send back home when the installation fails.
type InstallationFailed struct {
	GenericEvent `json:",inline"`
}

// UpgradeStarted event is sent back home when an upgrade starts.
type UpgradeStarted struct {
	GenericEvent `json:",inline"`
	BinaryName   string `json:"binaryName"`
	LegacyType   string `json:"type"` // TODO(ethan): confirm if we can remove this
	LicenseID    string `json:"licenseID"`
	AppChannelID string `json:"appChannelID"`
	AppVersion   string `json:"appVersion"`
}

// Title returns the name of the event.
func (e UpgradeStarted) Title() string {
	return EventTypeUpgradeStarted
}

// Type returns the type of the event.
func (e UpgradeStarted) Type() string {
	return EventTypeUpgradeStarted
}

// UpgradeSucceeded event is sent back home when an upgrade finishes successfully.
type UpgradeSucceeded struct {
	GenericEvent `json:",inline"`
}

// UpgradeFailed event is sent back home when an upgrade fails.
type UpgradeFailed struct {
	GenericEvent `json:",inline"`
}

// JoinStarted event is send back home when a node join starts.
type JoinStarted struct {
	GenericEvent `json:",inline"`
	NodeName     string `json:"nodeName"`
}

// Title returns the name of the event.
func (e JoinStarted) Title() string {
	return EventTypeJoinStarted
}

// Type returns the type of the event.
func (e JoinStarted) Type() string {
	return EventTypeJoinStarted
}

// JoinSucceeded event is send back home when a node join succeeds.
type JoinSucceeded struct {
	GenericEvent `json:",inline"`
	NodeName     string `json:"nodeName"`
}

// Title returns the name of the event.
func (e JoinSucceeded) Title() string {
	return EventTypeJoinSucceeded
}

// Type returns the type of the event.
func (e JoinSucceeded) Type() string {
	return EventTypeJoinSucceeded
}

// JoinFailed event is send back home when a node join fails.
type JoinFailed struct {
	GenericEvent `json:",inline"`
	NodeName     string `json:"nodeName"`
}

// Title returns the name of the event.
func (e JoinFailed) Title() string {
	return EventTypeJoinFailed
}

// Type returns the type of the event.
func (e JoinFailed) Type() string {
	return EventTypeJoinFailed
}

// PreflightsFailed event is send back home when the preflights failed.
type PreflightsFailed struct {
	GenericEvent    `json:",inline"`
	NodeName        string `json:"nodeName"`
	PreflightOutput string `json:"preflightOutput"`
}

// PreflightsBypassed event is send back home when the preflights failed but were bypassed.
type PreflightsBypassed struct {
	GenericEvent    `json:",inline"`
	NodeName        string `json:"nodeName"`
	PreflightOutput string `json:"preflightOutput"`
}

// PreflightsSucceeded event is send back home when the preflights succeeded.
type PreflightsSucceeded struct {
	GenericEvent `json:",inline"`
	NodeName     string `json:"nodeName"`
}
