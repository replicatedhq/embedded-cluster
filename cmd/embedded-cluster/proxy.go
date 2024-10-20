package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

func withProxyFlags(flags []cli.Flag) []cli.Flag {
	return append(flags,
		&cli.StringFlag{
			Name:   "http-proxy",
			Usage:  "Proxy server to use for HTTP",
			Hidden: false,
		},
		&cli.StringFlag{
			Name:   "https-proxy",
			Usage:  "Proxy server to use for HTTPS",
			Hidden: false,
		},
		&cli.StringFlag{
			Name:   "no-proxy",
			Usage:  "Comma-separated list of hosts for which not to use a proxy",
			Hidden: false,
		},
		&cli.BoolFlag{
			Name:   "proxy",
			Usage:  "Use the system proxy settings for the install operation. These variables are currently only passed through to Velero and the Admin Console.",
			Hidden: true,
		},
	)
}

func getProxySpecFromFlags(c *cli.Context) (*ecv1beta1.ProxySpec, error) {
	proxy := &ecv1beta1.ProxySpec{}
	var providedNoProxy []string
	if c.Bool("proxy") {
		proxy.HTTPProxy = os.Getenv("HTTP_PROXY")
		proxy.HTTPSProxy = os.Getenv("HTTPS_PROXY")
		if os.Getenv("NO_PROXY") != "" {
			providedNoProxy = append(providedNoProxy, os.Getenv("NO_PROXY"))
		}
	}
	if c.IsSet("http-proxy") {
		proxy.HTTPProxy = c.String("http-proxy")
	}
	if c.IsSet("https-proxy") {
		proxy.HTTPSProxy = c.String("https-proxy")
	}
	if c.String("no-proxy") != "" {
		providedNoProxy = append(providedNoProxy, c.String("no-proxy"))
	}

	proxy.ProvidedNoProxy = strings.Join(providedNoProxy, ",")
	if err := combineNoProxySuppliedValuesAndDefaults(c, proxy); err != nil {
		return nil, fmt.Errorf("unable to combine no-proxy supplied values and defaults: %w", err)
	}

	if proxy.HTTPProxy == "" && proxy.HTTPSProxy == "" && proxy.NoProxy == "" {
		return nil, nil
	}
	return proxy, nil
}

func combineNoProxySuppliedValuesAndDefaults(c *cli.Context, proxy *ecv1beta1.ProxySpec) error {
	if proxy.ProvidedNoProxy == "" {
		return nil
	}
	noProxy := strings.Split(proxy.ProvidedNoProxy, ",")
	if len(noProxy) > 0 || proxy.HTTPProxy != "" || proxy.HTTPSProxy != "" {
		noProxy = append(defaults.DefaultNoProxy, noProxy...)
		podnet, svcnet, err := DeterminePodAndServiceCIDRs(c)
		if err != nil {
			return fmt.Errorf("unable to determine pod and service CIDRs: %w", err)
		}
		noProxy = append(noProxy, podnet, svcnet)
		proxy.NoProxy = strings.Join(noProxy, ",")
	}
	return nil
}

// setProxyEnv sets the HTTP_PROXY, HTTPS_PROXY, and NO_PROXY environment variables based on the provided ProxySpec.
// If the provided ProxySpec is nil, this environment variables are not set.
func setProxyEnv(proxy *ecv1beta1.ProxySpec) {
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

func includeLocalIPInNoProxy(c *cli.Context, proxy *ecv1beta1.ProxySpec) (*ecv1beta1.ProxySpec, error) {
	if proxy != nil && (proxy.HTTPProxy != "" || proxy.HTTPSProxy != "") {
		// if there is a proxy set, then there needs to be a no proxy set
		// if it is not set, prompt with a default (the local IP or subnet)
		// if it is set, we need to check that it covers the local IP
		ipnet, err := netutils.FirstValidIPNet(c.String("network-interface"))
		if err != nil {
			return nil, fmt.Errorf("failed to get first valid ip net: %w", err)
		}
		cleanIPNet, err := cleanCIDR(ipnet)
		if err != nil {
			return nil, fmt.Errorf("failed to clean subnet: %w", err)
		}
		if proxy.ProvidedNoProxy == "" {
			logrus.Infof("--no-proxy was not set. Adding the default interface's subnet (%q) to the no-proxy list.", cleanIPNet)
			proxy.ProvidedNoProxy = cleanIPNet
			if err := combineNoProxySuppliedValuesAndDefaults(c, proxy); err != nil {
				return nil, fmt.Errorf("unable to combine no-proxy supplied values and defaults: %w", err)
			}
			return proxy, nil
		} else {
			isValid, err := validateNoProxy(proxy.NoProxy, ipnet.IP.String())
			if err != nil {
				return nil, fmt.Errorf("failed to validate no-proxy: %w", err)
			} else if !isValid {
				logrus.Infof("The node IP (%q) is not included in the provided no-proxy list (%q). Adding the default interface's subnet (%q) to the no-proxy list.", ipnet.IP.String(), proxy.ProvidedNoProxy, cleanIPNet)
				proxy.ProvidedNoProxy = cleanIPNet
				if err := combineNoProxySuppliedValuesAndDefaults(c, proxy); err != nil {
					return nil, fmt.Errorf("unable to combine no-proxy supplied values and defaults: %w", err)
				}
				return proxy, nil
			}
		}
	}
	return proxy, nil
}

// cleanCIDR returns a `.0/x` subnet instead of a `.2/x` etc subnet
func cleanCIDR(ipnet *net.IPNet) (string, error) {
	_, newNet, err := net.ParseCIDR(ipnet.String())
	if err != nil {
		return "", fmt.Errorf("failed to parse local inet CIDR %q: %w", ipnet.String(), err)
	}
	return newNet.String(), nil
}

func validateNoProxy(newNoProxy string, localIP string) (bool, error) {
	foundLocal := false
	for _, oneEntry := range strings.Split(newNoProxy, ",") {
		if oneEntry == localIP {
			foundLocal = true
		} else if strings.Contains(oneEntry, "/") {
			_, ipnet, err := net.ParseCIDR(oneEntry)
			if err != nil {
				return false, fmt.Errorf("failed to parse CIDR within no-proxy: %w", err)
			}
			if ipnet.Contains(net.ParseIP(localIP)) {
				foundLocal = true
			}
		}
	}

	return foundLocal, nil
}

func checkProxyConfigForLocalIP(proxy *ecv1beta1.ProxySpec, networkInterface string) (bool, string, error) {
	if proxy == nil {
		return true, "", nil // no proxy is fine
	}
	if proxy.HTTPProxy == "" && proxy.HTTPSProxy == "" {
		return true, "", nil // no proxy is fine
	}

	ipnet, err := netutils.FirstValidIPNet(networkInterface)
	if err != nil {
		return false, "", fmt.Errorf("failed to get default IPNet: %w", err)
	}

	ok, err := validateNoProxy(proxy.NoProxy, ipnet.IP.String())
	return ok, ipnet.IP.String(), err
}
