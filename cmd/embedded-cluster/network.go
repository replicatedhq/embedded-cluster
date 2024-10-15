package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/apparentlymart/go-cidr/cidr"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/urfave/cli/v2"
)

func withSubnetCIDRFlags(flags []cli.Flag) []cli.Flag {
	return append(flags,
		&cli.StringFlag{
			Name:   "pod-cidr",
			Usage:  "IP address range for Pods",
			Value:  k0sv1beta1.DefaultNetwork().PodCIDR,
			Hidden: true,
		},
		&cli.StringFlag{
			Name:   "service-cidr",
			Usage:  "IP address range for Services",
			Value:  k0sv1beta1.DefaultNetwork().ServiceCIDR,
			Hidden: true,
		},
		&cli.StringFlag{
			Name:  "cidr",
			Usage: "IP address range for Services and Pods (needs to be at least a /16)",
			Value: k0sv1beta1.DefaultNetwork().PodCIDR,
			Action: func(c *cli.Context, value string) error {
				return validateCIDR(value)
			},
		},
	)
}

// validateCIDR parses the cidr provided by the user and returns an error if it
// is invalid or if it is smaller of a /16.
func validateCIDR(value string) error {
	_, ipnet, err := net.ParseCIDR(value)
	if err != nil {
		return fmt.Errorf("invalid cidr: %w", err)
	}

	size, _ := ipnet.Mask.Size()
	if size > 16 {
		return fmt.Errorf("cidr needs to be at least a /16")
	}

	privates := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"}
	for _, cidr := range privates {
		if _, privnet, _ := net.ParseCIDR(cidr); privnet.Contains(ipnet.IP) {
			return nil
		}
	}

	return fmt.Errorf("cidr is not within the private ranges %s", strings.Join(privates, ", "))
}

// SplitNetworkCIDR splits the provided network CIDR into pod and service
// CIDRs. The network is split in half, with the first half being the pod CIDR
// and the second half being the service CIDR.
func SplitNetworkCIDR(netaddr string) (string, string, error) {
	_, ipnet, err := net.ParseCIDR(netaddr)
	if err != nil {
		return "", "", fmt.Errorf("unable to parse cidr: %w", err)
	}

	podnet, err := cidr.Subnet(ipnet, 1, 0)
	if err != nil {
		return "", "", fmt.Errorf("unable to determine pod cidr: %w", err)
	}

	svcnet, err := cidr.Subnet(ipnet, 1, 1)
	if err != nil {
		return "", "", fmt.Errorf("unable to determine service cidr: %w", err)
	}

	return podnet.String(), svcnet.String(), nil
}

// DeterminePodAndServiceCIDRS determines, based on the command line flags,
// what are the pod and service CIDRs to be used for the cluster. If both
// --pod-cidr and --service-cidr have been set, they are used. Otherwise,
// the cidr flag is split into pod and service CIDRs.
func DeterminePodAndServiceCIDRs(c *cli.Context) (string, string, error) {
	if c.IsSet("pod-cidr") && c.IsSet("service-cidr") {
		return c.String("pod-cidr"), c.String("service-cidr"), nil
	}
	return SplitNetworkCIDR(c.String("cidr"))
}
