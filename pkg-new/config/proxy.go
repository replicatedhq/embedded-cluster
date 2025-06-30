package config

import (
	"fmt"
	"net"
	"os"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
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

func GetNetworkIPNet(networkInterface string, lookup NetworkLookup) (*net.IPNet, error) {
	if lookup == nil {
		lookup = defaultNetworkLookupImpl
	}
	return lookup.FirstValidIPNet(networkInterface)
}

func GetProxySpec(httpProxy, httpsProxy, noProxy string, podCIDR string, serviceCIDR string, networkInterface string, lookup NetworkLookup) (*ecv1beta1.ProxySpec, error) {
	proxy := &ecv1beta1.ProxySpec{
		HTTPProxy:       httpProxy,
		HTTPSProxy:      httpsProxy,
		ProvidedNoProxy: noProxy,
	}

	SetProxyDefaults(proxy)

	// Now that we have all no-proxy entries (from flags/env), merge in defaults
	if err := populateNoProxy(proxy, podCIDR, serviceCIDR, networkInterface, lookup); err != nil {
		return nil, fmt.Errorf("unable to combine no-proxy supplied values and defaults: %w", err)
	}

	if proxy.HTTPProxy == "" && proxy.HTTPSProxy == "" && proxy.NoProxy == "" {
		return nil, nil
	}
	return proxy, nil
}

func SetProxyDefaults(proxy *ecv1beta1.ProxySpec) {
	if proxy.HTTPProxy == "" {
		if envValue := os.Getenv("http_proxy"); envValue != "" {
			// logrus.Debug("got http_proxy from http_proxy env var")
			proxy.HTTPProxy = envValue
		} else if envValue := os.Getenv("HTTP_PROXY"); envValue != "" {
			// logrus.Debug("got http_proxy from HTTP_PROXY env var")
			proxy.HTTPProxy = envValue
		}
	}

	if proxy.HTTPSProxy == "" {
		if envValue := os.Getenv("https_proxy"); envValue != "" {
			// logrus.Debug("got https_proxy from https_proxy env var")
			proxy.HTTPSProxy = envValue
		} else if envValue := os.Getenv("HTTPS_PROXY"); envValue != "" {
			// logrus.Debug("got https_proxy from HTTPS_PROXY env var")
			proxy.HTTPSProxy = envValue
		}
	}

	if proxy.ProvidedNoProxy == "" {
		if envValue := os.Getenv("no_proxy"); envValue != "" {
			// logrus.Debug("got no_proxy from no_proxy env var")
			proxy.ProvidedNoProxy = envValue
		} else if envValue := os.Getenv("NO_PROXY"); envValue != "" {
			// logrus.Debug("got no_proxy from NO_PROXY env var")
			proxy.ProvidedNoProxy = envValue
		}
	}
}

func populateNoProxy(proxy *ecv1beta1.ProxySpec, podCIDR string, serviceCIDR string, networkInterface string, lookup NetworkLookup) error {
	if proxy.ProvidedNoProxy == "" && proxy.HTTPProxy == "" && proxy.HTTPSProxy == "" {
		return nil
	}

	// Start with runtime defaults
	noProxy := runtimeconfig.DefaultNoProxy

	// Add pod and service CIDRs
	noProxy = append(noProxy, podCIDR, serviceCIDR)

	// Add user-provided no-proxy values
	if proxy.ProvidedNoProxy != "" {
		noProxy = append(noProxy, strings.Split(proxy.ProvidedNoProxy, ",")...)
	}

	// If we have a proxy set, ensure the local IP is in the no-proxy list
	if proxy.HTTPProxy != "" || proxy.HTTPSProxy != "" {
		ipnet, err := GetNetworkIPNet(networkInterface, lookup)
		if err != nil {
			return fmt.Errorf("failed to get first valid ip net: %w", err)
		}
		cleanIPNet, err := cleanCIDR(ipnet)
		if err != nil {
			return fmt.Errorf("failed to clean subnet: %w", err)
		}

		// Check if the local IP is already covered by any of the no-proxy entries
		isValid, err := NoProxyHasLocalIP(strings.Join(noProxy, ","), ipnet.IP.String())
		if err != nil {
			return fmt.Errorf("failed to validate no-proxy: %w", err)
		} else if !isValid {
			logrus.Debugf("The node IP (%q) is not included in the no-proxy list. Adding the network interface's subnet (%q).", ipnet.IP.String(), cleanIPNet)
			noProxy = append(noProxy, cleanIPNet)
		}
	}

	proxy.NoProxy = strings.Join(noProxy, ",")
	return nil
}

// SetProxyEnv sets the HTTP_PROXY, HTTPS_PROXY, and NO_PROXY environment variables based on the provided ProxySpec.
// If the provided ProxySpec is nil, this environment variables are not set.
func SetProxyEnv(proxy *ecv1beta1.ProxySpec) {
	if proxy == nil {
		return
	}
	if proxy.HTTPProxy != "" {
		os.Setenv("HTTP_PROXY", proxy.HTTPProxy)
	}
	if proxy.HTTPSProxy != "" {
		os.Setenv("HTTPS_PROXY", proxy.HTTPSProxy)
	}
	if proxy.NoProxy != "" {
		os.Setenv("NO_PROXY", proxy.NoProxy)
	}
}

func NoProxyHasLocalIP(noProxy string, localIP string) (bool, error) {
	foundLocal := false
	localIPParsed := net.ParseIP(localIP)
	if localIPParsed == nil {
		return false, fmt.Errorf("failed to parse local IP %q", localIP)
	}

	for _, oneEntry := range strings.Split(noProxy, ",") {
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

func CheckProxyConfigForLocalIP(proxy *ecv1beta1.ProxySpec, networkInterface string, lookup NetworkLookup) (bool, string, error) {
	if proxy == nil {
		return true, "", nil // no proxy is fine
	}
	if proxy.HTTPProxy == "" && proxy.HTTPSProxy == "" {
		return true, "", nil // no proxy is fine
	}

	ipnet, err := GetNetworkIPNet(networkInterface, lookup)
	if err != nil {
		return false, "", fmt.Errorf("failed to get default IPNet: %w", err)
	}

	ok, err := NoProxyHasLocalIP(proxy.NoProxy, ipnet.IP.String())
	return ok, ipnet.IP.String(), err
}
