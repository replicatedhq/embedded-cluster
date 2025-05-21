package cli

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/replicatedhq/embedded-cluster/api/models"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/spf13/cobra"
)

func addProxyFlags(cmd *cobra.Command, installConfig *models.InstallationConfig) error {
	cmd.Flags().StringVar(&installConfig.HTTPProxy, "http-proxy", "", "HTTP proxy to use for the installation (overrides http_proxy/HTTP_PROXY environment variables)")
	cmd.Flags().StringVar(&installConfig.HTTPSProxy, "https-proxy", "", "HTTPS proxy to use for the installation (overrides https_proxy/HTTPS_PROXY environment variables)")
	cmd.Flags().StringVar(&installConfig.NoProxy, "no-proxy", "", "Comma-separated list of hosts for which not to use a proxy (overrides no_proxy/NO_PROXY environment variables)")

	return nil
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
