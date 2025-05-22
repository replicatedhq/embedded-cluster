package types

type InstallationConfig struct {
	AdminConsolePassword    string `json:"adminConsolePassword"`
	AdminConsolePort        int    `json:"adminConsolePort"`
	DataDirectory           string `json:"dataDirectory"`
	HostCABundlePath        string `json:"hostCaBundlePath"`
	LocalArtifactMirrorPort int    `json:"localArtifactMirrorPort"`
	HTTPProxy               string `json:"httpProxy"`
	HTTPSProxy              string `json:"httpsProxy"`
	NoProxy                 string `json:"noProxy"`
	NetworkInterface        string `json:"networkInterface"`
	PodCIDR                 string `json:"podCidr"`
	ServiceCIDR             string `json:"serviceCidr"`
	GlobalCIDR              string `json:"globalCidr"`
	EndUserConfigOverrides  string `json:"endUserConfigOverrides"`
}
