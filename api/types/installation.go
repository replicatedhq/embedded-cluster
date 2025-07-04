package types

type LinuxInstallation struct {
	Config LinuxInstallationConfig `json:"config"`
	Status Status                  `json:"status"`
}

// LinuxInstallationConfig represents the configuration for an installation
type LinuxInstallationConfig struct {
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

type KubernetesInstallation struct {
	Config KubernetesInstallationConfig `json:"config"`
	Status Status                       `json:"status"`
}

type KubernetesInstallationConfig struct {
	AdminConsolePort int    `json:"adminConsolePort"`
	HTTPProxy        string `json:"httpProxy"`
	HTTPSProxy       string `json:"httpsProxy"`
	NoProxy          string `json:"noProxy"`
}
