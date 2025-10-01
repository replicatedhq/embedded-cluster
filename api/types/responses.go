package types

// InstallHostPreflightsStatusResponse represents the response when polling install host preflights status
type InstallHostPreflightsStatusResponse struct {
	Titles                    []string          `json:"titles"`
	Output                    *PreflightsOutput `json:"output,omitempty"`
	Status                    Status            `json:"status,omitempty"`
	AllowIgnoreHostPreflights bool              `json:"allowIgnoreHostPreflights"`
}

// InstallAppPreflightsStatusResponse represents the response when polling install app preflights status
type InstallAppPreflightsStatusResponse struct {
	Titles                        []string          `json:"titles"`
	Output                        *PreflightsOutput `json:"output,omitempty"`
	Status                        Status            `json:"status"`
	HasStrictAppPreflightFailures bool              `json:"hasStrictAppPreflightFailures"`
	AllowIgnoreAppPreflights      bool              `json:"allowIgnoreAppPreflights"`
}

// UpgradeAppPreflightsStatusResponse represents the response when polling upgrade app preflights status
type UpgradeAppPreflightsStatusResponse struct {
	Titles                        []string          `json:"titles"`
	Output                        *PreflightsOutput `json:"output,omitempty"`
	Status                        Status            `json:"status"`
	HasStrictAppPreflightFailures bool              `json:"hasStrictAppPreflightFailures"`
	AllowIgnoreAppPreflights      bool              `json:"allowIgnoreAppPreflights"`
}

// GetListAvailableNetworkInterfacesResponse represents the response when listing available network interfaces
type GetListAvailableNetworkInterfacesResponse struct {
	NetworkInterfaces []string `json:"networkInterfaces"`
}

// AppConfigValuesResponse represents a response containing app config values
type AppConfigValuesResponse struct {
	Values AppConfigValues `json:"values"`
}
