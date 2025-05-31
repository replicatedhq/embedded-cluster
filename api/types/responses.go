package types

// InstallHostPreflightsStatusResponse represents the response when polling install host preflights status
type InstallHostPreflightsStatusResponse struct {
	Titles []string             `json:"titles"`
	Output *HostPreflightOutput `json:"output,omitempty"`
	Status *Status              `json:"status,omitempty"`
}
