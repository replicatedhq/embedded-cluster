package main

import (
	"fmt"
	"net"
	"strings"

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
