package types

// InstallHostPreflightsStatusResponse represents the response when polling install host preflights status
type InstallHostPreflightsStatusResponse struct {
	Titles                    []string          `json:"titles"`
	Output                    *PreflightsOutput `json:"output,omitempty"`
	Status                    Status            `json:"status,omitempty"`
	AllowIgnoreHostPreflights bool              `json:"allowIgnoreHostPreflights"`
}

// GetListAvailableNetworkInterfacesResponse represents the response when listing available network interfaces
type GetListAvailableNetworkInterfacesResponse struct {
	NetworkInterfaces []string `json:"networkInterfaces"`
}

// AppConfigValuesResponse represents a response containing app config values
type AppConfigValuesResponse struct {
	Values AppConfigValues `json:"values"`
}
