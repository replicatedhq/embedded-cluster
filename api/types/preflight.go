package types

import "time"

// RunHostPreflightResponse represents the response from starting host preflight checks
type RunHostPreflightResponse struct {
	Status HostPreflightStatus `json:"status"`
}

// HostPreflightStatusResponse represents the response when polling host preflight status
type HostPreflightStatusResponse struct {
	Status HostPreflightStatus  `json:"status"`
	Output *HostPreflightOutput `json:"output,omitempty"`
}

// HostPreflightStatus represents the current status of host preflight checks
type HostPreflightStatus struct {
	State       HostPreflightState `json:"state"`
	Description string             `json:"description"`
	LastUpdated time.Time          `json:"lastUpdated"`
}

// HostPreflightState represents the possible states of host preflight execution
type HostPreflightState string

const (
	HostPreflightStatePending   HostPreflightState = "Pending"
	HostPreflightStateRunning   HostPreflightState = "Running"
	HostPreflightStateSucceeded HostPreflightState = "Succeeded"
	HostPreflightStateFailed    HostPreflightState = "Failed"
)

// HostPreflightOutput represents the output of host preflight checks
type HostPreflightOutput struct {
	Pass []HostPreflightRecord `json:"pass"`
	Warn []HostPreflightRecord `json:"warn"`
	Fail []HostPreflightRecord `json:"fail"`
}

// HostPreflightRecord represents a single host preflight check result
type HostPreflightRecord struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}
