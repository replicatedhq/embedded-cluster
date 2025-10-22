package types

type LinuxInstallation struct {
	Config LinuxInstallationConfig `json:"config"`
	Status Status                  `json:"status"`
}

// LinuxInstallationConfig represents the configuration for an installation
type LinuxInstallationConfig struct {
	AdminConsolePort        int    `json:"adminConsolePort" validate:"optional"`
	DataDirectory           string `json:"dataDirectory" validate:"optional"`
	LocalArtifactMirrorPort int    `json:"localArtifactMirrorPort" validate:"optional"`
	HTTPProxy               string `json:"httpProxy" validate:"optional"`
	HTTPSProxy              string `json:"httpsProxy" validate:"optional"`
	NoProxy                 string `json:"noProxy" validate:"optional"`
	NetworkInterface        string `json:"networkInterface" validate:"optional"`
	PodCIDR                 string `json:"podCidr" validate:"optional"`
	ServiceCIDR             string `json:"serviceCidr" validate:"optional"`
	GlobalCIDR              string `json:"globalCidr" validate:"optional"`
}

type KubernetesInstallation struct {
	Config KubernetesInstallationConfig `json:"config"`
	Status Status                       `json:"status"`
}

type KubernetesInstallationConfig struct {
	AdminConsolePort int    `json:"adminConsolePort" validate:"optional"`
	HTTPProxy        string `json:"httpProxy" validate:"optional"`
	HTTPSProxy       string `json:"httpsProxy" validate:"optional"`
	NoProxy          string `json:"noProxy" validate:"optional"`
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
