package main

import (
	"fmt"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
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
			Usage: "CIDR block of available private IP addresses (/16 or larger)",
			Value: ecv1beta1.DefaultNetworkCIDR,
			Action: func(c *cli.Context, addr string) error {
				if c.IsSet("pod-cidr") || c.IsSet("service-cidr") {
					return fmt.Errorf("--cidr flag can't be used with --pod-cidr or --service-cidr")
				}
				if err := netutils.ValidateCIDR(addr, 16, true); err != nil {
					return err
				}
				return nil
			},
		},
	)
}

// DeterminePodAndServiceCIDRS determines, based on the command line flags,
// what are the pod and service CIDRs to be used for the cluster. If both
// --pod-cidr and --service-cidr have been set, they are used. Otherwise,
// the cidr flag is split into pod and service CIDRs.
func DeterminePodAndServiceCIDRs(c *cli.Context) (string, string, error) {
	if c.IsSet("pod-cidr") || c.IsSet("service-cidr") {
		return c.String("pod-cidr"), c.String("service-cidr"), nil
	}
	return netutils.SplitNetworkCIDR(c.String("cidr"))
}

// getCIDRs returns the CIDRs in use based on the provided cli flags.
func getCIDRs(c *cli.Context) (string, string, string) {
	if c.IsSet("pod-cidr") || c.IsSet("service-cidr") {
		return c.String("pod-cidr"), c.String("service-cidr"), ""
	}
	return "", "", c.String("cidr")
}
