package types

type Infra struct {
	Components      []InfraComponent `json:"components"`
	Logs            string           `json:"logs"`
	Status          Status           `json:"status"`
	RequiresUpgrade bool             `json:"requiresUpgrade"`
}

type InfraComponent struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
}
