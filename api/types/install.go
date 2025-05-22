package types

import (
	"time"
)

const (
	InstallStatusRunning   InstallStatus = "Running"
	InstallStatusSucceeded InstallStatus = "Succeeded"
	InstallStatusFailed    InstallStatus = "Failed"
)

const (
	InstallPhaseNameSettingConfig InstallPhaseName = "SettingConfig"
	InstallPhaseNameStarting      InstallPhaseName = "Starting"
)

const (
	InstallPhaseStatusRunning   InstallPhaseStatus = "Running"
	InstallPhaseStatusSucceeded InstallPhaseStatus = "Succeeded"
	InstallPhaseStatusFailed    InstallPhaseStatus = "Failed"
)

type InstallStatus string
type InstallPhaseName string
type InstallPhaseStatus string

type Install struct {
	Config InstallationConfig `json:"config"`
	Status InstallStatus      `json:"status"`
	Phases []InstallPhase     `json:"phases"`
}

type InstallPhase struct {
	Name        InstallPhaseName   `json:"name"`
	Status      InstallPhaseStatus `json:"status"`
	StartedAt   time.Time          `json:"startedAt"`
	CompletedAt time.Time          `json:"completedAt"`
}
