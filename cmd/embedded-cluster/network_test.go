package main

import (
	"flag"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func Test_getCIDRs(t *testing.T) {
	tests := []struct {
		name            string
		buildCliContext func(*flag.FlagSet) *cli.Context
		expected        []string
	}{
		{
			name: "with pod and service flags",
			expected: []string{
				"10.0.0.0/24",
				"10.1.0.0/24",
				"",
			},
			buildCliContext: func(flagSet *flag.FlagSet) *cli.Context {
				c := cli.NewContext(cli.NewApp(), flagSet, nil)
				c.Set("pod-cidr", "10.0.0.0/24")
				c.Set("service-cidr", "10.1.0.0/24")
				return c
			},
		},
		{
			name: "with pod flag",
			expected: []string{
				"10.0.0.0/24",
				v1beta1.DefaultNetwork().ServiceCIDR,
				"",
			},
			buildCliContext: func(flagSet *flag.FlagSet) *cli.Context {
				c := cli.NewContext(cli.NewApp(), flagSet, nil)
				c.Set("pod-cidr", "10.0.0.0/24")
				return c
			},
		},
		{
			name: "with pod, service and cidr flags",
			expected: []string{
				"10.0.0.0/24",
				"10.1.0.0/24",
				"",
			},
			buildCliContext: func(flagSet *flag.FlagSet) *cli.Context {
				c := cli.NewContext(cli.NewApp(), flagSet, nil)
				c.Set("pod-cidr", "10.0.0.0/24")
				c.Set("service-cidr", "10.1.0.0/24")
				c.Set("cidr", "10.2.0.0/24")
				return c
			},
		},
		{
			name: "with pod and cidr flags",
			expected: []string{
				"10.0.0.0/24",
				v1beta1.DefaultNetwork().ServiceCIDR,
				"",
			},
			buildCliContext: func(flagSet *flag.FlagSet) *cli.Context {
				c := cli.NewContext(cli.NewApp(), flagSet, nil)
				c.Set("pod-cidr", "10.0.0.0/24")
				c.Set("cidr", "10.2.0.0/24")
				return c
			},
		},
		{
			name: "with cidr flag",
			expected: []string{
				"",
				"",
				"10.2.0.0/24",
			},
			buildCliContext: func(flagSet *flag.FlagSet) *cli.Context {
				c := cli.NewContext(cli.NewApp(), flagSet, nil)
				c.Set("cidr", "10.2.0.0/24")
				return c
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			flagSet := flag.NewFlagSet(t.Name(), 0)
			flags := withSubnetCIDRFlags([]cli.Flag{})
			for _, f := range flags {
				err := f.Apply(flagSet)
				req.NoError(err)
			}

			cc := test.buildCliContext(flagSet)
			podCIDR, serviceCIDR, CIDR := getCIDRs(cc)
			req.Equal(test.expected[0], podCIDR)
			req.Equal(test.expected[1], serviceCIDR)
			req.Equal(test.expected[2], CIDR)
		})
	}
}

func Test_DeterminePodAndServiceCIDRs(t *testing.T) {

	tests := []struct {
		name            string
		buildCliContext func(*flag.FlagSet) *cli.Context
		expected        []string
	}{
		{
			name: "with pod flag",
			expected: []string{
				"10.0.0.0/16",
				v1beta1.DefaultNetwork().ServiceCIDR,
			},
			buildCliContext: func(flagSet *flag.FlagSet) *cli.Context {
				c := cli.NewContext(cli.NewApp(), flagSet, nil)
				c.Set("pod-cidr", "10.0.0.0/16")
				return c
			},
		},
		{
			name: "with service flag",
			expected: []string{
				v1beta1.DefaultNetwork().PodCIDR,
				"10.1.0.0/16",
			},
			buildCliContext: func(flagSet *flag.FlagSet) *cli.Context {
				c := cli.NewContext(cli.NewApp(), flagSet, nil)
				c.Set("service-cidr", "10.1.0.0/16")
				return c
			},
		},
		{
			name: "with cidr flag",
			expected: []string{
				"10.0.0.0/16",
				"10.1.0.0/16",
			},
			buildCliContext: func(flagSet *flag.FlagSet) *cli.Context {
				c := cli.NewContext(cli.NewApp(), flagSet, nil)
				c.Set("cidr", "10.0.0.0/15")
				return c
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			flagSet := flag.NewFlagSet(t.Name(), 0)
			flags := withSubnetCIDRFlags([]cli.Flag{})
			for _, f := range flags {
				err := f.Apply(flagSet)
				req.NoError(err)
			}

			cc := test.buildCliContext(flagSet)
			podCIDR, serviceCIDR, err := DeterminePodAndServiceCIDRs(cc)
			req.NoError(err)
			req.Equal(test.expected[0], podCIDR)
			req.Equal(test.expected[1], serviceCIDR)
		})
	}
}
