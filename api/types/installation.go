package types

import "time"

type InstallationConfig struct {
	AdminConsolePort        int    `json:"adminConsolePort"`
	DataDirectory           string `json:"dataDirectory"`
	LocalArtifactMirrorPort int    `json:"localArtifactMirrorPort"`
	HTTPProxy               string `json:"httpProxy"`
	HTTPSProxy              string `json:"httpsProxy"`
	NoProxy                 string `json:"noProxy"`
	NetworkInterface        string `json:"networkInterface"`
	PodCIDR                 string `json:"podCidr"`
	ServiceCIDR             string `json:"serviceCidr"`
	GlobalCIDR              string `json:"globalCidr"`
}

type InstallationStatus struct {
	State       InstallationState `json:"state"`
	Description string            `json:"description"`
	LastUpdated time.Time         `json:"lastUpdated"`
}

type InstallationState string

const (
	InstallationStatePending   InstallationState = "Pending"
	InstallationStateRunning   InstallationState = "Running"
	InstallationStateSucceeded InstallationState = "Succeeded"
	InstallationStateFailed    InstallationState = "Failed"
)
