package types

// HostPreflights represents the host preflight checks state
type HostPreflights struct {
	Titles []string              `json:"titles"`
	Output *HostPreflightsOutput `json:"output"`
	Status *Status               `json:"status"`
}

type HostPreflightsOutput struct {
	Pass []HostPreflightsRecord `json:"pass"`
	Warn []HostPreflightsRecord `json:"warn"`
	Fail []HostPreflightsRecord `json:"fail"`
}

// HostPreflightsRecord represents a single host preflight check result
type HostPreflightsRecord struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

func NewHostPreflights() *HostPreflights {
	return &HostPreflights{
		Status: NewStatus(),
	}
}

// HasFail returns true if any of the preflight checks failed.
func (o HostPreflightsOutput) HasFail() bool {
	return len(o.Fail) > 0
}

// HasWarn returns true if any of the preflight checks returned a warning.
func (o HostPreflightsOutput) HasWarn() bool {
	return len(o.Warn) > 0
}
