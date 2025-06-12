package types

type Infra struct {
	Components []InfraComponent `json:"components"`
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
