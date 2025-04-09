package types

import (
	"strings"

	"github.com/google/uuid"
)

// Event is implemented by all events. Title returns a string that identifies the
// event type.
type Event interface {
	Title() string
}

// BaseEvent contains common fields shared by all events.
type BaseEvent struct {
	ExecutionID  string    `json:"executionID"`
	ClusterID    uuid.UUID `json:"clusterID"`
	Version      string    `json:"version"`
	EntryCommand string    `json:"entryCommand"`
	CommandArgs  string    `json:"commandArgs"`
}

// NewBaseEvent creates a new BaseEvent with the given parameters.
func NewBaseEvent(executionID string, clusterID uuid.UUID, version, entryCommand string, commandArgs []string) BaseEvent {
	return BaseEvent{
		ExecutionID:  executionID,
		ClusterID:    clusterID,
		Version:      version,
		EntryCommand: entryCommand,
		CommandArgs:  strings.Join(commandArgs, " "),
	}
}

// InstallationStarted event is send back home when the installation starts.
type InstallationStarted struct {
	BaseEvent    `json:",inline"`
	Flags        string `json:"flags"`
	BinaryName   string `json:"binaryName"`
	Type         string `json:"type"`
	LicenseID    string `json:"licenseID"`
	AppChannelID string `json:"appChannelID"`
	AppVersion   string `json:"appVersion"`
}

// Title returns the name of the event.
func (e InstallationStarted) Title() string {
	return "InstallationStarted"
}

// InstallationSucceeded event is send back home when the installation finishes.
type InstallationSucceeded struct {
	BaseEvent `json:",inline"`
}

// Title returns the name of the event.
func (e InstallationSucceeded) Title() string {
	return "InstallationSucceeded"
}

// InstallationFailed event is send back home when the installation fails.
type InstallationFailed struct {
	BaseEvent `json:",inline"`
	Reason    string `json:"reason"`
}

// Title returns the name of the event.
func (e InstallationFailed) Title() string {
	return "InstallationFailed"
}

// JoinStarted event is send back home when a node join starts.
type JoinStarted struct {
	BaseEvent `json:",inline"`
	NodeName  string `json:"nodeName"`
}

// Title returns the name of the event.
func (e JoinStarted) Title() string {
	return "JoinStarted"
}

// JoinSucceeded event is send back home when a node join succeeds.
type JoinSucceeded struct {
	BaseEvent `json:",inline"`
	NodeName  string `json:"nodeName"`
}

// Title returns the name of the event.
func (e JoinSucceeded) Title() string {
	return "JoinSucceeded"
}

// JoinFailed event is send back home when a node join fails.
type JoinFailed struct {
	BaseEvent `json:",inline"`
	NodeName  string `json:"nodeName"`
	Reason    string `json:"reason"`
}

// Title returns the name of the event.
func (e JoinFailed) Title() string {
	return "JoinFailed"
}

// PreflightsFailed event is send back home when the preflights failed.
type PreflightsFailed struct {
	BaseEvent       `json:",inline"`
	NodeName        string `json:"nodeName"`
	PreflightOutput string `json:"preflightOutput"`
	EventType       string `json:"eventType"`
}

// Title returns the name of the event.
func (e PreflightsFailed) Title() string {
	// GenericEvents are added to the events table, but do not update the cluster status
	return "GenericEvent"
}

// PreflightsBypassed event is send back home when the preflights failed but were bypassed.
type PreflightsBypassed struct {
	BaseEvent       `json:",inline"`
	NodeName        string `json:"nodeName"`
	PreflightOutput string `json:"preflightOutput"`
	EventType       string `json:"eventType"`
}

// Title returns the name of the event.
func (e PreflightsBypassed) Title() string {
	// GenericEvents are added to the events table, but do not update the cluster status
	return "GenericEvent"
}

// SignalAborted event is sent back home when a process is terminated by a signal.
type SignalAborted struct {
	BaseEvent  `json:",inline"`
	NodeName   string `json:"nodeName"`
	SignalName string `json:"signalName"`
	EventType  string `json:"eventType"`
}

// Title returns the name of the event.
func (e SignalAborted) Title() string {
	// GenericEvents are added to the events table, but do not update the cluster status
	return "GenericEvent"
}
