package cli

import (
	"fmt"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/api/console"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/spf13/cobra"
)

func addCIDRFlags(cmd *cobra.Command) error {
	cmd.Flags().String("pod-cidr", k0sv1beta1.DefaultNetwork().PodCIDR, "IP address range for Pods")
	if err := cmd.Flags().MarkHidden("pod-cidr"); err != nil {
		return err
	}
	cmd.Flags().String("service-cidr", k0sv1beta1.DefaultNetwork().ServiceCIDR, "IP address range for Services")
	if err := cmd.Flags().MarkHidden("service-cidr"); err != nil {
		return err
	}
	cmd.Flags().String("cidr", ecv1beta1.DefaultNetworkCIDR, "CIDR block of available private IP addresses (/16 or larger)")

	return nil
}

func validateCIDRFlags(cmd *cobra.Command) error {
	if cmd.Flags().Changed("cidr") && (cmd.Flags().Changed("pod-cidr") || cmd.Flags().Changed("service-cidr")) {
		return fmt.Errorf("--cidr can't be used with --pod-cidr or --service-cidr")
	}

	return nil
}

type CIDRConfig struct {
	PodCIDR     string
	ServiceCIDR string
	GlobalCIDR  *string
}

// parseCIDRFlags determines, based on the command line flags,
// what are the CIDRs to be used for the cluster. If either
// of --pod-cidr or --service-cidr have been set, they are used. Otherwise,
// the cidr flag is used.
func parseCIDRFlags(cmd *cobra.Command, consoleConfig *console.Config) error {
	if cmd.Flags().Changed("pod-cidr") || cmd.Flags().Changed("service-cidr") {
		podCIDR, err := cmd.Flags().GetString("pod-cidr")
		if err != nil {
			return fmt.Errorf("unable to get pod-cidr flag: %w", err)
		}
		serviceCIDR, err := cmd.Flags().GetString("service-cidr")
		if err != nil {
			return fmt.Errorf("unable to get service-cidr flag: %w", err)
		}
		consoleConfig.PodCIDR = podCIDR
		consoleConfig.ServiceCIDR = serviceCIDR
		return nil
	}

	globalCIDR, err := cmd.Flags().GetString("cidr")
	if err != nil {
		return fmt.Errorf("unable to get cidr flag: %w", err)
	}
	consoleConfig.GlobalCIDR = globalCIDR
	return nil
}
