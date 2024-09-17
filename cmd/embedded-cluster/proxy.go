package main

import (
	"fmt"
	"github.com/replicatedhq/embedded-cluster/pkg/netutils"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/sirupsen/logrus"
	"os"
	"strings"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
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
	var noProxy []string
	if c.Bool("proxy") {
		proxy.HTTPProxy = os.Getenv("HTTP_PROXY")
		proxy.HTTPSProxy = os.Getenv("HTTPS_PROXY")
		if os.Getenv("NO_PROXY") != "" {
			noProxy = append(noProxy, os.Getenv("NO_PROXY"))
		}
	}
	if c.IsSet("http-proxy") {
		proxy.HTTPProxy = c.String("http-proxy")
	}
	if c.IsSet("https-proxy") {
		proxy.HTTPSProxy = c.String("https-proxy")
	}
	if c.String("no-proxy") != "" {
		noProxy = append(noProxy, c.String("no-proxy"))
	}
	if len(noProxy) > 0 || proxy.HTTPProxy != "" || proxy.HTTPSProxy != "" {
		noProxy = append(defaults.DefaultNoProxy, noProxy...)
		noProxy = append(noProxy, c.String("pod-cidr"), c.String("service-cidr"))
		proxy.NoProxy = strings.Join(noProxy, ",")
	}
	if proxy.HTTPProxy == "" && proxy.HTTPSProxy == "" && proxy.NoProxy == "" {
		return nil
	}
	return proxy
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
	if proxy.HTTPProxy != "" || proxy.HTTPSProxy != "" {
		// if there is a proxy set, then there needs to be a no proxy set
		// if it is not set, prompt with a default (the local IP or subnet)
		// if it is set, we need to check that it covers the local IP
		defaultIPNet, err := netutils.GetDefaultIPNet()
		if err != nil {
			return nil, fmt.Errorf("failed to get default IPNet: %w", err)
		}
		if proxy.NoProxy == "" {
			if c.Bool("no-prompt") {
				logrus.Infof("No proxy not set, using default no proxy %s", defaultIPNet.String())
				proxy.NoProxy = defaultIPNet.String()
				return proxy, nil
			} else {
				logrus.Infof("A noproxy is required when a proxy is set. We suggest either the node subnet (%s) or the addresses of every node that will be a member of the cluster. The current node's IP address is %q.", defaultIPNet.String(), defaultIPNet.IP.String())
				newProxy := prompts.New().Input("No proxy:", "", true)
				// TODO validate the new no proxy
				proxy.NoProxy = newProxy
				return proxy, nil
			}
		} else {
			logrus.Infof("A noproxy is already set (%q), checking if it covers the local IP", proxy.NoProxy)
			// TODO validate the existing no proxy
			return proxy, nil
		}
	}
	return proxy, nil
}
