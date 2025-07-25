package netutils

import (
	"fmt"
	"net"
	"strings"

	"github.com/apparentlymart/go-cidr/cidr"
)

// SplitNetworkCIDR splits the provided network CIDR into two separated
// subnets.
func SplitNetworkCIDR(netaddr string) (string, string, error) {
	_, ipnet, err := net.ParseCIDR(netaddr)
	if err != nil {
		return "", "", fmt.Errorf("unable to parse cidr: %w", err)
	}

	podnet, err := cidr.Subnet(ipnet, 1, 0)
	if err != nil {
		return "", "", fmt.Errorf("unable to determine first cidr: %w", err)
	}

	svcnet, err := cidr.Subnet(ipnet, 1, 1)
	if err != nil {
		return "", "", fmt.Errorf("unable to determine second cidr: %w", err)
	}

	return podnet.String(), svcnet.String(), nil
}

// ValidateCIDR is a function that helps validating a network CIDR, it can
// check if the provided CIDR has a minimal size and if it is in the addresses
// reserved for private networks.
func ValidateCIDR(cidr string, notLessThan int, private bool) error {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}

	if ipnet.String() != cidr {
		//nolint:staticcheck // ST1005
		return fmt.Errorf("The provided CIDR block (%s) is not valid", cidr)
	}

	if size, _ := ipnet.Mask.Size(); size > notLessThan {
		//nolint:staticcheck // ST1005
		return fmt.Errorf("The provided CIDR block (%s) is too small. It must be /%d or larger.", cidr, notLessThan)
	}

	if !private {
		return nil
	}

	privates := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	for _, cidr := range privates {
		if _, privnet, _ := net.ParseCIDR(cidr); privnet.Contains(ipnet.IP) {
			return nil
		}
	}

	//nolint:staticcheck // ST1005
	return fmt.Errorf("The provided CIDR block (%s) is not in a private IP address range (%s)", cidr, strings.Join(privates, ", "))
}

// NetworksAreAdjacentAndSameSize returns true if the two provided CIDRs are
// adjacent and have the same size. If this function returns true then the
// second returned value is the CIDR that encompasses the two provided CIDRs.
func NetworksAreAdjacentAndSameSize(a, b string) (bool, string, error) {
	_, anet, err := net.ParseCIDR(a)
	if err != nil {
		return false, "", fmt.Errorf("unable to parse first cidr %s: %w", a, err)
	}

	_, bnet, err := net.ParseCIDR(b)
	if err != nil {
		return false, "", fmt.Errorf("unable to parse second cidr %s: %w", b, err)
	}

	// if the mask is different we can bail out immediately.
	if anet.Mask.String() != bnet.Mask.String() {
		return false, "", nil
	}

	first, last := cidr.AddressRange(anet)
	last = cidr.Inc(last)
	if bnetfirst, _ := cidr.AddressRange(bnet); !bnetfirst.Equal(last) {
		return false, "", nil
	}

	suffix, _ := anet.Mask.Size()
	suffix--
	supernet := fmt.Sprintf("%s/%d", first.String(), suffix)
	return true, supernet, nil
}
