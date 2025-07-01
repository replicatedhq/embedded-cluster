package types

// LinuxInfraSetupRequest represents a request to set up infrastructure
type LinuxInfraSetupRequest struct {
	IgnoreHostPreflights bool `json:"ignoreHostPreflights"`
}

type LinuxInfra struct {
	Components []LinuxInfraComponent `json:"components"`
	Logs       string                `json:"logs"`
	Status     Status                `json:"status"`
}

type LinuxInfraComponent struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
}
