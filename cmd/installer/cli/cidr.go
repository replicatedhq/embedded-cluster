package cli

import (
	"fmt"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func mustAddCIDRFlags(flagSet *pflag.FlagSet) {
	flagSet.String("cidr", ecv1beta1.DefaultNetworkCIDR, "CIDR block of available private IP addresses (/16 or larger)")

	flagSet.String("pod-cidr", k0sv1beta1.DefaultNetwork().PodCIDR, "IP address range for Pods")
	mustMarkFlagHidden(flagSet, "pod-cidr")

	flagSet.String("service-cidr", k0sv1beta1.DefaultNetwork().ServiceCIDR, "IP address range for Services")
	mustMarkFlagHidden(flagSet, "service-cidr")
}

func validateCIDRFlags(cmd *cobra.Command) error {
	if cmd.Flags().Changed("cidr") && (cmd.Flags().Changed("pod-cidr") || cmd.Flags().Changed("service-cidr")) {
		return fmt.Errorf("--cidr can't be used with --pod-cidr or --service-cidr")
	}

	cidr, err := cmd.Flags().GetString("cidr")
	if err != nil {
		return fmt.Errorf("unable to get cidr flag: %w", err)
	}

	if err := newconfig.ValidateCIDR(cidr); err != nil {
		return err
	}

	return nil
}

// getCIDRConfig determines, based on the command line flags,
// what are the pod and service CIDRs to be used for the cluster. If either
// of --pod-cidr or --service-cidr have been set, they are used. Otherwise,
// the cidr flag is split into pod and service CIDRs.
func getCIDRConfig(cmd *cobra.Command) (*newconfig.CIDRConfig, error) {
	if cmd.Flags().Changed("pod-cidr") || cmd.Flags().Changed("service-cidr") {
		podCIDR, err := cmd.Flags().GetString("pod-cidr")
		if err != nil {
			return nil, fmt.Errorf("unable to get pod-cidr flag: %w", err)
		}
		serviceCIDR, err := cmd.Flags().GetString("service-cidr")
		if err != nil {
			return nil, fmt.Errorf("unable to get service-cidr flag: %w", err)
		}
		return &newconfig.CIDRConfig{
			PodCIDR:     podCIDR,
			ServiceCIDR: serviceCIDR,
		}, nil
	}

	globalCIDR, err := cmd.Flags().GetString("cidr")
	if err != nil {
		return nil, fmt.Errorf("unable to get cidr flag: %w", err)
	}
	podCIDR, serviceCIDR, err := newconfig.SplitCIDR(globalCIDR)
	if err != nil {
		return nil, fmt.Errorf("unable to split cidr flag: %w", err)
	}
	return &newconfig.CIDRConfig{
		PodCIDR:     podCIDR,
		ServiceCIDR: serviceCIDR,
		GlobalCIDR:  &globalCIDR,
	}, nil
}
