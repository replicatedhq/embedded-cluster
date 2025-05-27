package cli

import (
	"fmt"
	"net"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/spf13/cobra"
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

func addProxyFlags(cmd *cobra.Command) error {
	cmd.Flags().String("http-proxy", "", "HTTP proxy to use for the installation (overrides http_proxy/HTTP_PROXY environment variables)")
	cmd.Flags().String("https-proxy", "", "HTTPS proxy to use for the installation (overrides https_proxy/HTTPS_PROXY environment variables)")
	cmd.Flags().String("no-proxy", "", "Comma-separated list of hosts for which not to use a proxy (overrides no_proxy/NO_PROXY environment variables)")

	return nil
}

func parseProxyFlags(cmd *cobra.Command) (*ecv1beta1.ProxySpec, error) {
	p, err := getProxySpec(cmd)
	if err != nil {
		return nil, fmt.Errorf("unable to get proxy spec from flags: %w", err)
	}
	newconfig.SetProxyEnv(p)

	return p, nil
}

func getProxySpec(cmd *cobra.Command) (*ecv1beta1.ProxySpec, error) {
	// Command-line flags have the highest precedence
	httpProxy, err := cmd.Flags().GetString("http-proxy")
	if err != nil {
		return nil, fmt.Errorf("unable to get http-proxy flag: %w", err)
	}
	httpsProxy, err := cmd.Flags().GetString("https-proxy")
	if err != nil {
		return nil, fmt.Errorf("unable to get https-proxy flag: %w", err)
	}
	noProxy, err := cmd.Flags().GetString("no-proxy")
	if err != nil {
		return nil, fmt.Errorf("unable to get no-proxy flag: %w", err)
	}
	networkInterface, err := cmd.Flags().GetString("network-interface")
	if err != nil {
		return nil, fmt.Errorf("unable to get network-interface flag: %w", err)
	}
	cidrCfg, err := getCIDRConfig(cmd)
	if err != nil {
		return nil, fmt.Errorf("unable to determine pod and service CIDRs: %w", err)
	}
	proxy, err := newconfig.GetProxySpec(httpProxy, httpsProxy, noProxy, cidrCfg.PodCIDR, cidrCfg.ServiceCIDR, networkInterface, defaultNetworkLookupImpl)
	if err != nil {
		return nil, fmt.Errorf("unable to get proxy spec: %w", err)
	}
	return proxy, nil
}
