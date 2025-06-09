package types

type Installation struct {
	Config *InstallationConfig `json:"config"`
	Status *Status             `json:"status"`
}

// InstallationConfig represents the configuration for an installation
type InstallationConfig struct {
	DataDirectory           string `json:"dataDirectory"`
	AdminConsolePort        int    `json:"adminConsolePort"`
	LocalArtifactMirrorPort int    `json:"localArtifactMirrorPort"`
	NetworkInterface        string `json:"networkInterface"`
	GlobalCIDR              string `json:"globalCidr"`
	PodCIDR                 string `json:"podCidr"`
	ServiceCIDR             string `json:"serviceCidr"`
	HTTPProxy               string `json:"httpProxy"`
	HTTPSProxy              string `json:"httpsProxy"`
	NoProxy                 string `json:"noProxy"`
}

// NewInstallation initializes a new installation state
func NewInstallation() *Installation {
	return &Installation{
		Config: &InstallationConfig{},
		Status: NewStatus(),
	}
}
