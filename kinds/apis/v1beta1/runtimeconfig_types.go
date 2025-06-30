package v1beta1

import (
	"encoding/json"
)

const (
	DefaultDataDir                 = "/var/lib/embedded-cluster"
	DefaultAdminConsolePort        = 30000
	DefaultLocalArtifactMirrorPort = 50000
	DefaultNetworkCIDR             = "10.244.0.0/16"
	DefaultNetworkNodePortRange    = "80-32767"
	DefaultManagerPort             = 30080
)

// RuntimeConfigSpec defines the configuration for the Embedded Cluster at runtime.
type RuntimeConfigSpec struct {
	// DataDir holds the data directory for the Embedded Cluster
	// (default: /var/lib/embedded-cluster).
	DataDir string `json:"dataDir,omitempty"`
	// K0sDataDirOverride holds the override for the data directory for K0s. By default the data
	// will be stored in a subdirectory of DataDir.
	K0sDataDirOverride string `json:"k0sDataDirOverride,omitempty"`
	// OpenEBSDataDirOverride holds the override for the data directory for the OpenEBS storage
	// provisioner. By default the data will be stored in a subdirectory of DataDir.
	OpenEBSDataDirOverride string `json:"openEBSDataDirOverride,omitempty"`
	// HostCABundlePath holds the path to the CA bundle for the host.
	HostCABundlePath string `json:"hostCABundlePath,omitempty"`

	// Proxy holds the proxy configuration.
	Proxy *ProxySpec `json:"proxy,omitempty"`
	// Network holds the network configuration.
	Network NetworkSpec `json:"network,omitempty"`

	// AdminConsole holds the Admin Console configuration.
	AdminConsole AdminConsoleSpec `json:"adminConsole,omitempty"`
	// LocalArtifactMirrorPort holds the Local Artifact Mirror configuration.
	LocalArtifactMirror LocalArtifactMirrorSpec `json:"localArtifactMirror,omitempty"`
	// Manager holds the Manager configuration.
	Manager ManagerSpec `json:"manager,omitempty"`
}

func (c *RuntimeConfigSpec) UnmarshalJSON(data []byte) error {
	type alias RuntimeConfigSpec
	jc := (*alias)(c)
	err := json.Unmarshal(data, &jc)
	if err != nil {
		return err
	}
	return nil
}

func GetDefaultRuntimeConfig() *RuntimeConfigSpec {
	c := &RuntimeConfigSpec{}
	runtimeConfigSetDefaults(c)
	return c
}

func runtimeConfigSetDefaults(c *RuntimeConfigSpec) {
	if c.DataDir == "" {
		c.DataDir = DefaultDataDir
	}
	networkSpecSetDefaults(&c.Network)
	adminConsoleSpecSetDefaults(&c.AdminConsole)
	localArtifactMirrorSpecSetDefaults(&c.LocalArtifactMirror)
	managerSpecSetDefaults(&c.Manager)
}

func networkSpecSetDefaults(s *NetworkSpec) {
	if s.GlobalCIDR == "" {
		s.GlobalCIDR = DefaultNetworkCIDR
	}
	if s.NodePortRange == "" {
		s.NodePortRange = DefaultNetworkNodePortRange
	}
}

func adminConsoleSpecSetDefaults(s *AdminConsoleSpec) {
	if s.Port == 0 {
		s.Port = DefaultAdminConsolePort
	}
}

func localArtifactMirrorSpecSetDefaults(s *LocalArtifactMirrorSpec) {
	if s.Port == 0 {
		s.Port = DefaultLocalArtifactMirrorPort
	}
}

func managerSpecSetDefaults(s *ManagerSpec) {
	if s.Port == 0 {
		s.Port = DefaultManagerPort
	}
}
