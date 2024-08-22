package main

import (
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/urfave/cli/v2"
)

func withSubnetCIDRFlags(flags []cli.Flag) []cli.Flag {
	return append(flags,
		&cli.StringFlag{
			Name:   "pod-cidr",
			Usage:  "IP address range for pods",
			Hidden: false,
		},
		&cli.StringFlag{
			Name:   "service-cidr",
			Usage:  "IP address range for services",
			Hidden: false,
		},
	)
}

func getPodCIDR(c *cli.Context) string {
	if c.String("pod-cidr") != "" {
		return c.String("pod-cidr")
	}
	return k0sv1beta1.DefaultNetwork().PodCIDR
}

func getServiceCIDR(c *cli.Context) string {
	if c.String("service-cidr") != "" {
		return c.String("service-cidr")
	}
	return k0sv1beta1.DefaultNetwork().ServiceCIDR
}
