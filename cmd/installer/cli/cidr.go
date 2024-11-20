package cli

import (
	"fmt"
	"net"

	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/spf13/cobra"
)

// DeterminePodAndServiceCIDRS determines, based on the command line flags,
// what are the pod and service CIDRs to be used for the cluster. If both
// --pod-cidr and --service-cidr have been set, they are used. Otherwise,
// the cidr flag is split into pod and service CIDRs.
func determinePodAndServiceCIDRs(cmd *cobra.Command) (string, string, error) {
	podCIDRFlag, err := cmd.Flags().GetString("pod-cidr")
	if err != nil {
		return "", "", fmt.Errorf("unable to get pod-cidr flag: %w", err)
	}
	serviceCIDRFlag, err := cmd.Flags().GetString("service-cidr")
	if err != nil {
		return "", "", fmt.Errorf("unable to get service-cidr flag: %w", err)
	}

	if podCIDRFlag != "" || serviceCIDRFlag != "" {
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
