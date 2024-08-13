package main

import (
	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
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

func setSubnetCIDRFromFlags(c *cli.Context, cfg *k0sconfig.ClusterConfig) *k0sconfig.ClusterConfig {
	if c.String("pod-cidr") != "" {
		cfg.Spec.Network.PodCIDR = c.String("pod-cidr")
	}
	if c.String("service-cidr") != "" {
		cfg.Spec.Network.ServiceCIDR = c.String("service-cidr")
	}
	return cfg
}
