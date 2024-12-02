package types

import (
	"github.com/google/uuid"
)

// Event is implemented by all events. Title returns a string that identifies the
// event type.
type Event interface {
	Title() string
}

// InstallationStarted event is send back home when the installation starts.
type InstallationStarted struct {
	ClusterID    uuid.UUID `json:"clusterID"`
	Version      string    `json:"version"`
	Flags        string    `json:"flags"`
	BinaryName   string    `json:"binaryName"`
	Type         string    `json:"type"`
	LicenseID    string    `json:"licenseID"`
	AppChannelID string    `json:"appChannelID"`
	AppVersion   string    `json:"appVersion"`
}

// Title returns the name of the event.
func (e InstallationStarted) Title() string {
	return "InstallationStarted"
}

// InstallationSucceeded event is send back home when the installation finishes.
type InstallationSucceeded struct {
	ClusterID uuid.UUID `json:"clusterID"`
	Version   string    `json:"version"`
}

// Title returns the name of the event.
func (e InstallationSucceeded) Title() string {
	return "InstallationSucceeded"
}

// InstallationFailed event is send back home when the installation fails.
type InstallationFailed struct {
	ClusterID uuid.UUID `json:"clusterID"`
	Version   string    `json:"version"`
	Reason    string    `json:"reason"`
}

// Title returns the name of the event.
func (e InstallationFailed) Title() string {
	return "InstallationFailed"
}

// JoinStarted event is send back home when a node join starts.
type JoinStarted struct {
	ClusterID uuid.UUID `json:"clusterID"`
	Version   string    `json:"version"`
	NodeName  string    `json:"nodeName"`
}

// Title returns the name of the event.
func (e JoinStarted) Title() string {
	return "JoinStarted"
}

// JoinSucceeded event is send back home when a node join succeeds.
type JoinSucceeded JoinStarted

// Title returns the name of the event.
func (e JoinSucceeded) Title() string {
	return "JoinSucceeded"
}

// JoinFailed event is send back home when a node join fails.
type JoinFailed struct {
	ClusterID uuid.UUID `json:"clusterID"`
	Version   string    `json:"version"`
	NodeName  string    `json:"nodeName"`
	Reason    string    `json:"reason"`
}

// Title returns the name of the event.
func (e JoinFailed) Title() string {
	return "JoinFailed"
}

// PreflightsFailed event is send back home when the preflights failed but were bypassed.
type PreflightsFailed struct {
	ClusterID       uuid.UUID `json:"clusterID"`
	Version         string    `json:"version"`
	NodeName        string    `json:"nodeName"`
	PreflightOutput string    `json:"preflightOutput"`
	EventType       string    `json:"eventType"`
}

// Title returns the name of the event.
func (e PreflightsFailed) Title() string {
	// GenericEvents are added to the events table, but do not update the cluster status
	return "GenericEvent"
}
