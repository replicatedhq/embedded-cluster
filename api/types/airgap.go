package types

// Airgap represents the current state of airgap processing
type Airgap struct {
	Status Status `json:"status"`
	Logs   string `json:"logs"`
}
