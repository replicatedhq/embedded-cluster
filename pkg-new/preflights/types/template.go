package types

import (
	"fmt"
	"net"
)

type CIDRData struct {
	// Should specify the IP Address and size of the network e.g. "10.0.0.0/8"
	CIDR string
	// The size of the CIDR network. Should match the CIDR size above.
	Size int
}

type HostPreflightTemplateData struct {
	IsAirgap                     bool
	ReplicatedAppURL             string
	ProxyRegistryURL             string
	AdminConsolePort             int
	LocalArtifactMirrorPort      int
	DataDir                      string
	K0sDataDir                   string
	OpenEBSDataDir               string
	SystemArchitecture           string
	ServiceCIDR                  CIDRData
	PodCIDR                      CIDRData
	GlobalCIDR                   CIDRData
	HTTPProxy                    string
	HTTPSProxy                   string
	ProvidedNoProxy              string
	NoProxy                      string
	FromCIDR                     string
	ToCIDR                       string
	TCPConnectionsRequired       []string
	NodeIP                       string
	IsJoin                       bool
	IsUI                         bool
	ControllerAirgapStorageSpace string
	WorkerAirgapStorageSpace     string
}

// WithCIDRData sets the respective CIDR properties in the HostPreflightTemplateData struct based on the provided CIDR strings
func (t HostPreflightTemplateData) WithCIDRData(podCIDR string, serviceCIDR string, globalCIDR *string) (HostPreflightTemplateData, error) {
	if globalCIDR != nil && *globalCIDR != "" {
		_, cidr, err := net.ParseCIDR(*globalCIDR)
		if err != nil {
			return t, fmt.Errorf("invalid cidr: %w", err)
		}
		size, _ := cidr.Mask.Size()
		t.GlobalCIDR = CIDRData{
			CIDR: cidr.String(),
			Size: size,
		}
		return t, nil
	}

	s := CIDRData{}
	p := CIDRData{}
	if serviceCIDR != "" {
		_, cidr, err := net.ParseCIDR(serviceCIDR)
		if err != nil {
			return t, fmt.Errorf("invalid service cidr: %w", err)
		}
		size, _ := cidr.Mask.Size()
		s.CIDR = cidr.String()
		s.Size = size
	}

	if podCIDR != "" {
		_, cidr, err := net.ParseCIDR(podCIDR)
		if err != nil {
			return t, fmt.Errorf("invalid pod cidr: %w", err)
		}
		size, _ := cidr.Mask.Size()
		p.CIDR = cidr.String()
		p.Size = size
	}

	t.ServiceCIDR = s
	t.PodCIDR = p
	return t, nil
}
