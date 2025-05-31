package types

// HostPreflight represents the host preflight checks state
type HostPreflight struct {
	Titles []string             `json:"titles"`
	Output *HostPreflightOutput `json:"output"`
	Status *Status              `json:"status"`
}

// HostPreflightOutput represents the output of host preflight checks
type HostPreflightOutput struct {
	Pass []HostPreflightRecord `json:"pass"`
	Warn []HostPreflightRecord `json:"warn"`
	Fail []HostPreflightRecord `json:"fail"`
}

// HostPreflightRecord represents a single host preflight check result
type HostPreflightRecord struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

func NewHostPreflight() *HostPreflight {
	return &HostPreflight{
		Status: NewStatus(),
	}
}
