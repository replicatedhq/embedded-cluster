package models

import (
	"fmt"
	"net"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

// NetworkLookup defines the interface for network lookups
type NetworkLookup interface {
	FirstValidIPNet(networkInterface string) (*net.IPNet, error)
}

type defaultNetworkLookup struct{}

func (d *defaultNetworkLookup) FirstValidIPNet(networkInterface string) (*net.IPNet, error) {
	return netutils.FirstValidIPNet(networkInterface)
}

var defaultNetworkLookupImpl NetworkLookup = &defaultNetworkLookup{}

func getNetworkIPNet(networkInterface string, lookup NetworkLookup) (*net.IPNet, error) {
	if lookup == nil {
		lookup = defaultNetworkLookupImpl
	}
	return lookup.FirstValidIPNet(networkInterface)
}

func combineNoProxySuppliedValuesAndDefaults(config InstallationConfig, proxy ecv1beta1.ProxySpec, lookup NetworkLookup) (string, error) {
	// Start with runtime defaults
	noProxy := runtimeconfig.DefaultNoProxy

	// Add pod and service CIDRs
	noProxy = append(noProxy, config.PodCIDR, config.ServiceCIDR)

	// Add user-provided no-proxy values
	if proxy.ProvidedNoProxy != "" {
		noProxy = append(noProxy, strings.Split(proxy.ProvidedNoProxy, ",")...)
	}

	// If we have a proxy set, ensure the local IP is in the no-proxy list
	if proxy.HTTPProxy != "" || proxy.HTTPSProxy != "" {
		ipnet, err := getNetworkIPNet(config.NetworkInterface, lookup)
		if err != nil {
			return "", fmt.Errorf("get first valid ip net: %w", err)
		}
		cleanIPNet, err := cleanCIDR(ipnet)
		if err != nil {
			return "", fmt.Errorf("clean subnet: %w", err)
		}

		// Check if the local IP is already covered by any of the no-proxy entries
		isValid, err := validateNoProxy(strings.Join(noProxy, ","), ipnet.IP.String())
		if err != nil {
			return "", fmt.Errorf("validate no-proxy: %w", err)
		} else if !isValid {
			// TODO: how do we log this?
			// logrus.Infof("The node IP (%q) is not included in the no-proxy list. Adding the network interface's subnet (%q).", ipnet.IP.String(), cleanIPNet)
			noProxy = append(noProxy, cleanIPNet)
		}
	}

	return strings.Join(noProxy, ","), nil
}

func validateNoProxy(newNoProxy string, localIP string) (bool, error) {
	foundLocal := false
	localIPParsed := net.ParseIP(localIP)
	if localIPParsed == nil {
		return false, fmt.Errorf("failed to parse local IP %q", localIP)
	}

	for _, oneEntry := range strings.Split(newNoProxy, ",") {
		if oneEntry == localIP {
			foundLocal = true
		} else if strings.Contains(oneEntry, "/") {
			_, ipnet, err := net.ParseCIDR(oneEntry)
			if err != nil {
				return false, fmt.Errorf("failed to parse CIDR within no-proxy: %w", err)
			}
			if ipnet.Contains(localIPParsed) {
				foundLocal = true
			}
		}
	}

	return foundLocal, nil
}

// cleanCIDR returns a `.0/x` subnet instead of a `.2/x` etc subnet
func cleanCIDR(ipnet *net.IPNet) (string, error) {
	_, newNet, err := net.ParseCIDR(ipnet.String())
	if err != nil {
		return "", fmt.Errorf("failed to parse local inet CIDR %q: %w", ipnet.String(), err)
	}
	return newNet.String(), nil
}
