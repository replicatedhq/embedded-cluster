package main

import (
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/urfave/cli/v2"
)

func withSubnetCIDRFlags(flags []cli.Flag) []cli.Flag {
	return append(flags,
		&cli.StringFlag{
			Name:   "pod-cidr",
			Usage:  "IP address range for Pods",
			Value:  k0sv1beta1.DefaultNetwork().PodCIDR,
			Hidden: false,
		},
		&cli.StringFlag{
			Name:   "service-cidr",
			Usage:  "IP address range for Services",
			Value:  k0sv1beta1.DefaultNetwork().ServiceCIDR,
			Hidden: false,
		},
	)
}
