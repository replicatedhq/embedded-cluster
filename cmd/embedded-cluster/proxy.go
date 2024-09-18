package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
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

func getProxySpecFromFlags(c *cli.Context) *ecv1beta1.ProxySpec {
	proxy := &ecv1beta1.ProxySpec{}
	var providatedNoProxy []string
	if c.Bool("proxy") {
		proxy.HTTPProxy = os.Getenv("HTTP_PROXY")
		proxy.HTTPSProxy = os.Getenv("HTTPS_PROXY")
		if os.Getenv("NO_PROXY") != "" {
			providatedNoProxy = append(providatedNoProxy, os.Getenv("NO_PROXY"))
		}
	}
	if c.IsSet("http-proxy") {
		proxy.HTTPProxy = c.String("http-proxy")
	}
	if c.IsSet("https-proxy") {
		proxy.HTTPSProxy = c.String("https-proxy")
	}
	if c.String("no-proxy") != "" {
		providatedNoProxy = append(providatedNoProxy, c.String("no-proxy"))
	}
	proxy.ProvidedNoProxy = strings.Join(providatedNoProxy, ",")
	regenerateNoProxy(c, proxy)
	if proxy.HTTPProxy == "" && proxy.HTTPSProxy == "" && proxy.NoProxy == "" {
		return nil
	}
	return proxy
}

func regenerateNoProxy(c *cli.Context, proxy *ecv1beta1.ProxySpec) {
	if proxy.ProvidedNoProxy == "" {
		return
	}
	noProxy := strings.Split(proxy.ProvidedNoProxy, ",")
	if len(noProxy) > 0 || proxy.HTTPProxy != "" || proxy.HTTPSProxy != "" {
		noProxy = append(defaults.DefaultNoProxy, noProxy...)
		noProxy = append(noProxy, c.String("pod-cidr"), c.String("service-cidr"))
		proxy.NoProxy = strings.Join(noProxy, ",")
	}
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

func maybePromptForNoProxy(c *cli.Context, proxy *ecv1beta1.ProxySpec) (*ecv1beta1.ProxySpec, error) {
	if proxy != nil && (proxy.HTTPProxy != "" || proxy.HTTPSProxy != "") {
		// if there is a proxy set, then there needs to be a no proxy set
		// if it is not set, prompt with a default (the local IP or subnet)
		// if it is set, we need to check that it covers the local IP
		defaultIPNet, err := netutils.GetDefaultIPNet()
		if err != nil {
			return nil, fmt.Errorf("failed to get default IPNet: %w", err)
		}
		cleanDefaultIPNet, err := cleanCIDR(defaultIPNet)
		if err != nil {
			return nil, fmt.Errorf("failed to clean subnet: %w", err)
		}
		if proxy.ProvidedNoProxy == "" {
			if c.Bool("no-prompt") {
				logrus.Infof("no-proxy was not set, using default no proxy %s", cleanDefaultIPNet)
				proxy.ProvidedNoProxy = cleanDefaultIPNet
				regenerateNoProxy(c, proxy)
				logrus.Infof("final (default) no-proxy is %q", proxy.NoProxy)
				return proxy, nil
			} else {
				return promptForNoProxy(c, proxy, cleanDefaultIPNet, defaultIPNet.IP.String())
			}
		} else {
			shouldPrompt := false
			isValid, err := validateNoProxy(proxy.NoProxy, defaultIPNet.IP.String())
			if err != nil {
				logrus.Infof("The provided no-proxy %q failed to parse with error %q, falling back to prompt", proxy.NoProxy, err)
				shouldPrompt = true
			} else if !isValid {
				logrus.Infof("The provided no-proxy %q does not cover the local IP %q, falling back to prompt", proxy.NoProxy, defaultIPNet.IP.String())
				shouldPrompt = true
			}
			if shouldPrompt {
				return promptForNoProxy(c, proxy, cleanDefaultIPNet, defaultIPNet.IP.String())
			}
		}
	}
	return proxy, nil
}

// cleanCIDR returns a `.0/x` subnet instead of a `.2/x` etc subnet
func cleanCIDR(defaultIPNet *net.IPNet) (string, error) {
	_, newNet, err := net.ParseCIDR(defaultIPNet.String())
	if err != nil {
		return "", fmt.Errorf("failed to parse local inet CIDR %q: %w", defaultIPNet.String(), err)
	}
	return newNet.String(), nil
}

func promptForNoProxy(c *cli.Context, proxy *ecv1beta1.ProxySpec, subnet, IP string) (*ecv1beta1.ProxySpec, error) {
	logrus.Infof("A no-proxy address is required when a proxy is set. We suggest either the node subnet %q or the addresses of every node that will be a member of the cluster. The current node's IP address is %q.", subnet, IP)
	newProxy := prompts.New().Input("No proxy:", "", true)
	isValid, err := validateNoProxy(newProxy, IP)
	if err != nil {
		return nil, fmt.Errorf("failed to validate no-proxy: %w", err)
	}
	if !isValid {
		return nil, fmt.Errorf("provided no-proxy %q does not cover the local IP %q", newProxy, IP)
	}
	proxy.ProvidedNoProxy = newProxy
	regenerateNoProxy(c, proxy)
	fmt.Printf("final (provided) no-proxy is %q\n", proxy.NoProxy)
	return proxy, nil
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
