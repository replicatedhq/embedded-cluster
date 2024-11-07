// Package preflights manages running host preflights on remote hosts.
package preflights

import (
	"bytes"
	"fmt"
	"net"
	"text/template"
)

type CIDRData struct {
	// Should specify the IP Address and size of the network e.g. "10.0.0.0/8"
	CIDR string
	// The size of the CIDR network. Should match the CIDR size above.
	Size int
}

type TemplateData struct {
	IsAirgap                bool
	ReplicatedAPIURL        string
	ProxyRegistryURL        string
	AdminConsolePort        int
	LocalArtifactMirrorPort int
	DataDir                 string
	K0sDataDir              string
	OpenEBSDataDir          string
	SystemArchitecture      string
	ServiceCIDR             CIDRData
	PodCIDR                 CIDRData
	GlobalCIDR              CIDRData
	PrivateCA               string
	HTTPProxy               string
	HTTPSProxy              string
	ProvidedNoProxy         string
	NoProxy                 string
	FromCIDR                string
	ToCIDR                  string
}

// WithCIDRData sets the respective CIDR properties in the TemplateData struct based on the provided CIDR strings
func (t TemplateData) WithCIDRData(podCIDR, serviceCIDR, globalCIDR string) (TemplateData, error) {
	if globalCIDR != "" {
		_, cidr, err := net.ParseCIDR(globalCIDR)
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

func renderTemplate(spec string, data TemplateData) (string, error) {
	tmpl, err := template.New("preflight").Parse(spec)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	buf := bytes.NewBuffer(nil)
	err = tmpl.Execute(buf, data)
	if err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}
