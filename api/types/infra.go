package types

// InfraSetupRequest represents a request to set up infrastructure
type InfraSetupRequest struct {
	IgnorePreflightFailures bool `json:"ignorePreflightFailures"`
}

type Infra struct {
	Components []InfraComponent `json:"components"`
	Logs       string           `json:"logs"`
	Status     *Status          `json:"status"`
}

type InfraComponent struct {
	Name   string  `json:"name"`
	Status *Status `json:"status"`
}

func NewInfra() *Infra {
	return &Infra{
		Components: []InfraComponent{},
		Status:     NewStatus(),
	}
}
