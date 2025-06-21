package types

type Installation struct {
	Config InstallationConfig `json:"config"`
	Status Status             `json:"status"`
}

// InstallationConfig represents the configuration for an installation
type InstallationConfig struct {
	AdminConsolePort        int    `json:"adminConsolePort"`
	DataDirectory           string `json:"dataDirectory"`
	LocalArtifactMirrorPort int    `json:"localArtifactMirrorPort"`
	HTTPProxy               string `json:"httpProxy"`
	HTTPSProxy              string `json:"httpsProxy"`
	NoProxy                 string `json:"noProxy"`
	NetworkInterface        string `json:"networkInterface"`
	PodCIDR                 string `json:"podCidr"`
	ServiceCIDR             string `json:"serviceCidr"`
	GlobalCIDR              string `json:"globalCidr"`
}
