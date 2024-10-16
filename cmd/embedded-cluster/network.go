package main

import (
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
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
	)
}

// DeterminePodAndServiceCIDRS determines, based on the command line flags,
// what are the pod and service CIDRs to be used for the cluster. If both
// --pod-cidr and --service-cidr have been set, they are used. Otherwise,
// the cidr flag is split into pod and service CIDRs.
func DeterminePodAndServiceCIDRs(c *cli.Context, provider *defaults.Provider) (string, string, error) {
	if c.IsSet("pod-cidr") && c.IsSet("service-cidr") {
		return c.String("pod-cidr"), c.String("service-cidr"), nil
	}
	return provider.PodAndServiceCIDRs()
}
