package types

type PostInstallRunHostPreflightsRequest struct {
	IsUI bool `json:"isUi"`
}

// HostPreflights represents the host preflight checks state
type HostPreflights struct {
	Titles                    []string          `json:"titles"`
	Output                    *PreflightsOutput `json:"output"`
	Status                    Status            `json:"status"`
	AllowIgnoreHostPreflights bool              `json:"allowIgnoreHostPreflights"`
}

// PreflightsOutput represents the output from both host and app preflight checks
type PreflightsOutput struct {
	Pass []PreflightsRecord `json:"pass"`
	Warn []PreflightsRecord `json:"warn"`
	Fail []PreflightsRecord `json:"fail"`
}

// PreflightsRecord represents a single preflight check result
type PreflightsRecord struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

// HasFail returns true if any of the preflight checks failed.
func (o PreflightsOutput) HasFail() bool {
	return len(o.Fail) > 0
}

// HasWarn returns true if any of the preflight checks returned a warning.
func (o PreflightsOutput) HasWarn() bool {
	return len(o.Warn) > 0
}
