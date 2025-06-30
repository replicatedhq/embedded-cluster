package types

// Install represents the install workflow state
type Install struct {
	Steps  InstallSteps `json:"steps"`
	Status Status       `json:"status"`
}

// InstallSteps represents the steps of the install workflow
type InstallSteps struct {
	Installation  Installation   `json:"installation"`
	HostPreflight HostPreflights `json:"hostPreflight"`
	Infra         Infra          `json:"infra"`
}
