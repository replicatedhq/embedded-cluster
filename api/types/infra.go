package types

// LinuxInfraSetupRequest represents a request to set up infrastructure
type LinuxInfraSetupRequest struct {
	IgnoreHostPreflights bool `json:"ignoreHostPreflights"`
}

type Infra struct {
	Components []InfraComponent `json:"components"`
	Logs       string           `json:"logs"`
	Status     Status           `json:"status"`
}

type InfraComponent struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
}
