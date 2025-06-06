package types

// HostPreflight represents the host preflight checks state
type HostPreflight struct {
	Titles []string             `json:"titles"`
	Output *HostPreflightOutput `json:"output"`
	Status *Status              `json:"status"`
}

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

// HasFail returns true if any of the preflight checks failed.
func (o HostPreflightOutput) HasFail() bool {
	return len(o.Fail) > 0
}

// HasWarn returns true if any of the preflight checks returned a warning.
func (o HostPreflightOutput) HasWarn() bool {
	return len(o.Warn) > 0
}
