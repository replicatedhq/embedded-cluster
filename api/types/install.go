package types

type Install struct {
	Steps  InstallSteps `json:"steps"`
	Status *Status      `json:"status"`
}

type InstallSteps struct {
	Installation  *Installation  `json:"installation"`
	HostPreflight *HostPreflight `json:"hostPreflight"`
}

func NewInstall() *Install {
	return &Install{
		Steps: InstallSteps{
			Installation:  NewInstallation(),
			HostPreflight: NewHostPreflight(),
		},
		Status: NewStatus(),
	}
}
