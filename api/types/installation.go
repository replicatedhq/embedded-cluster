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

// LinuxInstallationConfigResponse represents the API response with the user provided values and defaults separated as well as the final resolved config
type LinuxInstallationConfigResponse struct {
	Values   LinuxInstallationConfig `json:"values"`
	Defaults LinuxInstallationConfig `json:"defaults"`
	Resolved LinuxInstallationConfig `json:"resolved"`
}

// KubernetesInstallationConfigResponse represents the API response with the user provided values and defaults separated as well as the final resolved config
type KubernetesInstallationConfigResponse struct {
	Values   KubernetesInstallationConfig `json:"values"`
	Defaults KubernetesInstallationConfig `json:"defaults"`
	Resolved KubernetesInstallationConfig `json:"resolved"`
}
