package types

type Install struct {
	Config InstallationConfig `json:"config"`
	Status InstallationStatus `json:"status"`
}
