package cli

import (
	"fmt"
	"net"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/spf13/cobra"
)

func addCIDRFlags(cmd *cobra.Command) {
	cmd.Flags().String("pod-cidr", k0sv1beta1.DefaultNetwork().PodCIDR, "IP address range for Pods")
	cmd.Flags().MarkHidden("pod-cidr")
	cmd.Flags().String("service-cidr", k0sv1beta1.DefaultNetwork().ServiceCIDR, "IP address range for Services")
	cmd.Flags().MarkHidden("service-cidr")
	cmd.Flags().String("cidr", ecv1beta1.DefaultNetworkCIDR, "CIDR block of available private IP addresses (/16 or larger)")
}

func validateCIDRFlags(cmd *cobra.Command) error {
	if cmd.Flags().Changed("cidr") && (cmd.Flags().Changed("pod-cidr") || cmd.Flags().Changed("service-cidr")) {
		return fmt.Errorf("--cidr flag can't be used with --pod-cidr or --service-cidr")
	}

	cidr, err := cmd.Flags().GetString("cidr")
	if err != nil {
		return fmt.Errorf("unable to get cidr flag: %w", err)
	}

	if err := netutils.ValidateCIDR(cidr, 16, true); err != nil {
		return fmt.Errorf("invalid cidr %q: %w", cidr, err)
	}

	return nil
}

// getPODAndServiceCIDR determines, based on the command line flags,
// what are the pod and service CIDRs to be used for the cluster. If both
// --pod-cidr and --service-cidr have been set, they are used. Otherwise,
// the cidr flag is split into pod and service CIDRs.
func getPODAndServiceCIDR(cmd *cobra.Command) (string, string, error) {
	if cmd.Flags().Changed("pod-cidr") || cmd.Flags().Changed("service-cidr") {
		podCIDRFlag, err := cmd.Flags().GetString("pod-cidr")
		if err != nil {
			return "", "", fmt.Errorf("unable to get pod-cidr flag: %w", err)
		}
		serviceCIDRFlag, err := cmd.Flags().GetString("service-cidr")
		if err != nil {
			return "", "", fmt.Errorf("unable to get service-cidr flag: %w", err)
		}
		return podCIDRFlag, serviceCIDRFlag, nil
	}

	cidrFlag, err := cmd.Flags().GetString("cidr")
	if err != nil {
		return "", "", fmt.Errorf("unable to get cidr flag: %w", err)
	}

	return netutils.SplitNetworkCIDR(cidrFlag)
}

// cleanCIDR returns a `.0/x` subnet instead of a `.2/x` etc subnet
func cleanCIDR(ipnet *net.IPNet) (string, error) {
	_, newNet, err := net.ParseCIDR(ipnet.String())
	if err != nil {
		return "", fmt.Errorf("failed to parse local inet CIDR %q: %w", ipnet.String(), err)
	}
	return newNet.String(), nil
}
