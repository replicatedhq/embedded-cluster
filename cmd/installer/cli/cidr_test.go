package cli

import (
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func Test_getCIDRConfig(t *testing.T) {
	tests := []struct {
		name     string
		setFlags func(flagSet *pflag.FlagSet)
		expected *newconfig.CIDRConfig
	}{
		{
			name: "with pod and service flags",
			expected: &newconfig.CIDRConfig{
				PodCIDR:     "10.0.0.0/24",
				ServiceCIDR: "10.1.0.0/24",
				GlobalCIDR:  nil,
			},
			setFlags: func(flagSet *pflag.FlagSet) {
				flagSet.Set("pod-cidr", "10.0.0.0/24")
				flagSet.Set("service-cidr", "10.1.0.0/24")
			},
		},
		{
			name: "with pod flag",
			expected: &newconfig.CIDRConfig{
				PodCIDR:     "10.0.0.0/24",
				ServiceCIDR: v1beta1.DefaultNetwork().ServiceCIDR,
				GlobalCIDR:  nil,
			},
			setFlags: func(flagSet *pflag.FlagSet) {
				flagSet.Set("pod-cidr", "10.0.0.0/24")
			},
		},
		{
			name: "with pod, service and cidr flags",
			expected: &newconfig.CIDRConfig{
				PodCIDR:     "10.0.0.0/24",
				ServiceCIDR: "10.1.0.0/24",
				GlobalCIDR:  nil,
			},
			setFlags: func(flagSet *pflag.FlagSet) {
				flagSet.Set("pod-cidr", "10.0.0.0/24")
				flagSet.Set("service-cidr", "10.1.0.0/24")
				flagSet.Set("cidr", "10.2.0.0/24")
			},
		},
		{
			name: "with pod and cidr flags",
			expected: &newconfig.CIDRConfig{
				PodCIDR:     "10.0.0.0/24",
				ServiceCIDR: v1beta1.DefaultNetwork().ServiceCIDR,
				GlobalCIDR:  nil,
			},
			setFlags: func(flagSet *pflag.FlagSet) {
				flagSet.Set("pod-cidr", "10.0.0.0/24")
				flagSet.Set("cidr", "10.2.0.0/24")
			},
		},
		{
			name: "with cidr flag",
			expected: &newconfig.CIDRConfig{
				PodCIDR:     "10.2.0.0/25",
				ServiceCIDR: "10.2.0.128/25",
				GlobalCIDR:  ptr.To("10.2.0.0/24"),
			},
			setFlags: func(flagSet *pflag.FlagSet) {
				flagSet.Set("cidr", "10.2.0.0/24")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			cmd := &cobra.Command{}
			mustAddCIDRFlags(cmd.Flags())

			test.setFlags(cmd.Flags())

			got, err := getCIDRConfig(cmd)
			req.NoError(err)
			req.Equal(test.expected, got)
		})
	}
}
