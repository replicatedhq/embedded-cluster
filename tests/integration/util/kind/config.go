package kind

type Cluster struct {
	APIVersion string     `yaml:"apiVersion" json:"apiVersion"`
	Kind       string     `yaml:"kind" json:"kind"`
	Name       string     `yaml:"name" json:"name"`
	Nodes      []Node     `yaml:"nodes,omitempty" json:"nodes,omitempty"`
	Networking Networking `yaml:"networking,omitempty" json:"networking,omitempty"`
}

type Node struct {
	Role              string        `yaml:"role,omitempty" json:"role,omitempty"`
	ExtraPortMappings []PortMapping `yaml:"extraPortMappings,omitempty" json:"extraPortMappings,omitempty"`
	ExtraMounts       []Mount       `yaml:"extraMounts,omitempty" json:"extraMounts,omitempty"`
}

type PortMapping struct {
	ContainerPort int32  `yaml:"containerPort" json:"containerPort"`
	HostPort      int32  `yaml:"hostPort" json:"hostPort"`
	ListenAddress string `yaml:"listenAddress,omitempty" json:"listenAddress,omitempty"`
	Protocol      string `yaml:"protocol,omitempty" json:"protocol,omitempty"`
}

type Mount struct {
	HostPath      string `yaml:"hostPath" json:"hostPath"`
	ContainerPath string `yaml:"containerPath" json:"containerPath"`
	ReadOnly      bool   `yaml:"readOnly,omitempty" json:"readOnly,omitempty"`
}

type Networking struct {
	PodSubnet     string `yaml:"podSubnet,omitempty" json:"podSubnet,omitempty"`
	ServiceSubnet string `yaml:"serviceSubnet,omitempty" json:"serviceSubnet,omitempty"`
}
