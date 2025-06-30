package config

import (
	"fmt"
	"net"

	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
)

func ValidateCIDR(cidr string) error {
	if err := netutils.ValidateCIDR(cidr, 16, true); err != nil {
		return fmt.Errorf("unable to validate cidr flag: %w", err)
	}
	return nil
}

type CIDRConfig struct {
	PodCIDR     string
	ServiceCIDR string
	GlobalCIDR  *string
}

// SplitCIDR takes a CIDR string and splits it into pod and service CIDRs
// to be used for the cluster. It returns a CIDRConfig containing the split CIDRs
// and the original global CIDR.
func SplitCIDR(cidr string) (string, string, error) {
	podCIDR, serviceCIDR, err := netutils.SplitNetworkCIDR(cidr)
	if err != nil {
		return "", "", fmt.Errorf("unable to split cidr flag: %w", err)
	}
	return podCIDR, serviceCIDR, nil
}

// cleanCIDR returns a `.0/x` subnet instead of a `.2/x` etc subnet
func cleanCIDR(ipnet *net.IPNet) (string, error) {
	_, newNet, err := net.ParseCIDR(ipnet.String())
	if err != nil {
		return "", fmt.Errorf("failed to parse local inet CIDR %q: %w", ipnet.String(), err)
	}
	return newNet.String(), nil
}
