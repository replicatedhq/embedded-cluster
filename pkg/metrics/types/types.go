package types

// Event type constants
const (
	EventTypeInstallationStarted   = "InstallationStarted"
	EventTypeInstallationSucceeded = "InstallationSucceeded"
	EventTypeInstallationFailed    = "InstallationFailed"
	EventTypeJoinStarted           = "JoinStarted"
	EventTypeJoinSucceeded         = "JoinSucceeded"
	EventTypeJoinFailed            = "JoinFailed"
	EventTypePreflightsFailed      = "PreflightsFailed"
	EventTypePreflightsBypassed    = "PreflightsBypassed"
	EventTypeSignalAborted         = "SignalAborted"
)

// Event is implemented by all events. Title returns a string that identifies the
// event type.
type Event interface {
	Title() string
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

// InstallationStarted event is send back home when the installation starts.
type InstallationStarted struct {
	GenericEvent `json:",inline"`
	BinaryName   string `json:"binaryName"`
	Type         string `json:"type"`
	LicenseID    string `json:"licenseID"`
	AppChannelID string `json:"appChannelID"`
	AppVersion   string `json:"appVersion"`
}

// Title returns the name of the event.
func (e InstallationStarted) Title() string {
	return EventTypeInstallationStarted
}

// InstallationSucceeded event is send back home when the installation finishes.
type InstallationSucceeded struct {
	GenericEvent `json:",inline"`
}

// Title returns the name of the event.
func (e InstallationSucceeded) Title() string {
	return EventTypeInstallationSucceeded
}

// InstallationFailed event is send back home when the installation fails.
type InstallationFailed struct {
	GenericEvent `json:",inline"`
}

// Title returns the name of the event.
func (e InstallationFailed) Title() string {
	return EventTypeInstallationFailed
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

// JoinSucceeded event is send back home when a node join succeeds.
type JoinSucceeded struct {
	GenericEvent `json:",inline"`
	NodeName     string `json:"nodeName"`
}

// Title returns the name of the event.
func (e JoinSucceeded) Title() string {
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
