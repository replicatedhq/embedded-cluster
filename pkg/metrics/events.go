package metrics

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
	ClusterID  uuid.UUID `json:"clusterID"`
	Version    string    `json:"version"`
	Flags      string    `json:"flags"`
	BinaryName string    `json:"binaryName"`
	Type       string    `json:"type"`
	LicenseID  string    `json:"licenseID"`
}

// Title returns the name of the event.
func (e InstallationStarted) Title() string {
	return "InstallationStarted"
}

// InstallationFinished event is send back home when the installation finishes.
type InstallationSucceeded struct {
	ClusterID uuid.UUID `json:"clusterID"`
}

// Title returns the name of the event.
func (e InstallationSucceeded) Title() string {
	return "InstallationSucceeded"
}

// InstallationFailed event is send back home when the installation fails.
type InstallationFailed struct {
	ClusterID uuid.UUID `json:"clusterID"`
	Reason    string    `json:"reason"`
}

// Title returns the name of the event.
func (e InstallationFailed) Title() string {
	return "InstallationFailed"
}

// UpgradeStarted event is send back home when the upgrade starts.
type UpgradeStarted InstallationStarted

// Title returns the name of the event.
func (e UpgradeStarted) Title() string {
	return "UpgradeStarted"
}

// UpgradeSucceeded event is send back home when the upgrade finishes.
type UpgradeSucceeded InstallationSucceeded

// Title returns the name of the event.
func (e UpgradeSucceeded) Title() string {
	return "UpgradeSucceeded"
}

// UpgradeFailed event is send back home when the upgrade fails.
type UpgradeFailed InstallationFailed

// Title returns the name of the event.
func (e UpgradeFailed) Title() string {
	return "UpgradeFailed"
}

// JoinStarted event is send back home when a node join starts.
type JoinStarted struct {
	ClusterID uuid.UUID `json:"clusterID"`
	Name      string    `json:"name"`
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
	Name      string    `json:"name"`
	Reason    string    `json:"reason"`
}

// Title returns the name of the event.
func (e JoinFailed) Title() string {
	return "JoinFailed"
}
