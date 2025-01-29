package cli

import (
	"fmt"
	"net"
	"os"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func addProxyFlags(cmd *cobra.Command) error {
	cmd.Flags().String("http-proxy", "", "HTTP proxy to use for the installation")
	cmd.Flags().String("https-proxy", "", "HTTPS proxy to use for the installation")
	cmd.Flags().String("no-proxy", "", "Comma-separated list of hosts for which not to use a proxy")

	// the "proxy" flag is hidden and used in tests to detect the proxy from the env
	cmd.Flags().Bool("proxy", false, "Use the system proxy settings for the install operation. These variables are currently only passed through to Velero and the Admin Console.")
	if err := cmd.Flags().MarkHidden("proxy"); err != nil {
		return err
	}

	return nil
}

func parseProxyFlags(cmd *cobra.Command) (*ecv1beta1.ProxySpec, error) {
	p, err := getProxySpecFromFlags(cmd)
	if err != nil {
		return nil, fmt.Errorf("unable to get proxy spec from flags: %w", err)
	}

	p, err = includeLocalIPInNoProxy(cmd, p)
	if err != nil {
		return nil, fmt.Errorf("unable to include local IP in no proxy: %w", err)
	}
	setProxyEnv(p)

	return p, nil
}

func getProxySpecFromFlags(cmd *cobra.Command) (*ecv1beta1.ProxySpec, error) {
	proxy := &ecv1beta1.ProxySpec{}
	var providedNoProxy []string

	proxyFlag, err := cmd.Flags().GetBool("proxy")
	if err != nil {
		return nil, fmt.Errorf("unable to get proxy flag: %w", err)
	}

	httpProxyFlag, err := cmd.Flags().GetString("http-proxy")
	if err != nil {
		return nil, fmt.Errorf("unable to get http-proxy flag: %w", err)
	}
	httpsProxyFlag, err := cmd.Flags().GetString("https-proxy")
	if err != nil {
		return nil, fmt.Errorf("unable to get https-proxy flag: %w", err)
	}
	noProxyFlag, err := cmd.Flags().GetString("no-proxy")
	if err != nil {
		return nil, fmt.Errorf("unable to get no-proxy flag: %w", err)
	}

	if proxyFlag {
		proxy.HTTPProxy = os.Getenv("HTTP_PROXY")
		proxy.HTTPSProxy = os.Getenv("HTTPS_PROXY")
		if os.Getenv("NO_PROXY") != "" {
			providedNoProxy = append(providedNoProxy, os.Getenv("NO_PROXY"))
		}
	}

	if httpProxyFlag != "" {
		proxy.HTTPProxy = httpProxyFlag
	}
	if httpsProxyFlag != "" {
		proxy.HTTPSProxy = httpsProxyFlag
	}
	if noProxyFlag != "" {
		providedNoProxy = append(providedNoProxy, noProxyFlag)
	}

	proxy.ProvidedNoProxy = strings.Join(providedNoProxy, ",")
	if err := combineNoProxySuppliedValuesAndDefaults(cmd, proxy); err != nil {
		return nil, fmt.Errorf("unable to combine no-proxy supplied values and defaults: %w", err)
	}

	if proxy.HTTPProxy == "" && proxy.HTTPSProxy == "" && proxy.NoProxy == "" {
		return nil, nil
	}
	return proxy, nil
}

func combineNoProxySuppliedValuesAndDefaults(cmd *cobra.Command, proxy *ecv1beta1.ProxySpec) error {
	if proxy.ProvidedNoProxy == "" {
		return nil
	}
	noProxy := strings.Split(proxy.ProvidedNoProxy, ",")
	if len(noProxy) > 0 || proxy.HTTPProxy != "" || proxy.HTTPSProxy != "" {
		noProxy = append(runtimeconfig.DefaultNoProxy, noProxy...)
		cidrCfg, err := getCIDRConfig(cmd)
		if err != nil {
			return fmt.Errorf("unable to determine pod and service CIDRs: %w", err)
		}
		noProxy = append(noProxy, cidrCfg.PodCIDR, cidrCfg.ServiceCIDR)
		proxy.NoProxy = strings.Join(noProxy, ",")
	}
	return nil
}

func includeLocalIPInNoProxy(cmd *cobra.Command, proxy *ecv1beta1.ProxySpec) (*ecv1beta1.ProxySpec, error) {
	if proxy != nil && (proxy.HTTPProxy != "" || proxy.HTTPSProxy != "") {
		// if there is a proxy set, then there needs to be a no proxy set
		// if it is not set, prompt with a default (the local IP or subnet)
		// if it is set, we need to check that it covers the local IP
		networkInterfaceFlag, err := cmd.Flags().GetString("network-interface")
		if err != nil {
			return nil, fmt.Errorf("unable to get network-interface flag: %w", err)
		}

		ipnet, err := netutils.FirstValidIPNet(networkInterfaceFlag)
		if err != nil {
			return nil, fmt.Errorf("failed to get first valid ip net: %w", err)
		}
		cleanIPNet, err := cleanCIDR(ipnet)
		if err != nil {
			return nil, fmt.Errorf("failed to clean subnet: %w", err)
		}
		if proxy.ProvidedNoProxy == "" {
			logrus.Infof("--no-proxy was not set. Adding the network interface's subnet (%q) to the no-proxy list.", cleanIPNet)
			proxy.ProvidedNoProxy = cleanIPNet
			if err := combineNoProxySuppliedValuesAndDefaults(cmd, proxy); err != nil {
				return nil, fmt.Errorf("unable to combine no-proxy supplied values and defaults: %w", err)
			}
			return proxy, nil
		} else {
			isValid, err := validateNoProxy(proxy.NoProxy, ipnet.IP.String())
			if err != nil {
				return nil, fmt.Errorf("failed to validate no-proxy: %w", err)
			} else if !isValid {
				logrus.Infof("The node IP (%q) is not included in the provided no-proxy list (%q). Adding the network interface's subnet (%q) to the no-proxy list.", ipnet.IP.String(), proxy.ProvidedNoProxy, cleanIPNet)
				proxy.ProvidedNoProxy = cleanIPNet
				if err := combineNoProxySuppliedValuesAndDefaults(cmd, proxy); err != nil {
					return nil, fmt.Errorf("unable to combine no-proxy supplied values and defaults: %w", err)
				}
				return proxy, nil
			}
		}
	}
	return proxy, nil
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
