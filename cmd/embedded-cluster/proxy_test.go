package main

import (
	"flag"
	"testing"

	k0sconfig "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v2"
)

func Test_getProxySpecFromFlags(t *testing.T) {
	type args struct {
		cfg *k0sconfig.ClusterConfig
	}
	tests := []struct {
		name string
		init func(t *testing.T, flagSet *flag.FlagSet)
		want *ecv1beta1.ProxySpec
	}{
		{
			name: "no flags set should not set proxy",
			init: func(t *testing.T, flagSet *flag.FlagSet) {
				t.Setenv("HTTP_PROXY", "http://proxy")
				t.Setenv("HTTPS_PROXY", "https://proxy")
				t.Setenv("NO_PROXY", "no-proxy-1,no-proxy-2")
			},
			want: nil,
		},
		{
			name: "proxy from env flag set should set proxy",
			init: func(t *testing.T, flagSet *flag.FlagSet) {
				t.Setenv("HTTP_PROXY", "http://proxy")
				t.Setenv("HTTPS_PROXY", "https://proxy")
				t.Setenv("NO_PROXY", "no-proxy-1,no-proxy-2")

				flagSet.Set("proxy", "true")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:  "http://proxy",
				HTTPSProxy: "https://proxy",
				NoProxy:    "localhost,127.0.0.1,.default,.local,.svc,kubernetes,kotsadm-rqlite,no-proxy-1,no-proxy-2,10.244.0.0/16,10.96.0.0/12",
			},
		},
		{
			name: "proxy from env flag set should not set proxy for lowercase env vars",
			init: func(t *testing.T, flagSet *flag.FlagSet) {
				t.Setenv("http_proxy", "http://proxy")
				t.Setenv("https_proxy", "https://proxy")
				t.Setenv("no_proxy", "no-proxy-1,no-proxy-2")

				flagSet.Set("proxy", "true")
			},
			want: nil,
		},
		{
			name: "proxy flags set should set proxy",
			init: func(t *testing.T, flagSet *flag.FlagSet) {
				t.Setenv("HTTP_PROXY", "http://proxy")
				t.Setenv("HTTPS_PROXY", "https://proxy")
				t.Setenv("NO_PROXY", "no-proxy-1,no-proxy-2")

				flagSet.Set("http-proxy", "http://other-proxy")
				flagSet.Set("https-proxy", "https://other-proxy")
				flagSet.Set("no-proxy", "other-no-proxy-1,other-no-proxy-2")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:  "http://other-proxy",
				HTTPSProxy: "https://other-proxy",
				NoProxy:    "localhost,127.0.0.1,.default,.local,.svc,kubernetes,kotsadm-rqlite,other-no-proxy-1,other-no-proxy-2,10.244.0.0/16,10.96.0.0/12",
			},
		},
		{
			name: "proxy flags should override proxy from env",
			init: func(t *testing.T, flagSet *flag.FlagSet) {
				t.Setenv("HTTP_PROXY", "http://proxy")
				t.Setenv("HTTPS_PROXY", "https://proxy")
				t.Setenv("NO_PROXY", "no-proxy-1,no-proxy-2")

				flagSet.Set("proxy", "true")
				flagSet.Set("http-proxy", "http://other-proxy")
				flagSet.Set("https-proxy", "https://other-proxy")
				flagSet.Set("no-proxy", "other-no-proxy-1,other-no-proxy-2")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:  "http://other-proxy",
				HTTPSProxy: "https://other-proxy",
				NoProxy:    "localhost,127.0.0.1,.default,.local,.svc,kubernetes,kotsadm-rqlite,no-proxy-1,no-proxy-2,other-no-proxy-1,other-no-proxy-2,10.244.0.0/16,10.96.0.0/12",
			},
		},
		{
			name: "pod and service CIDR should override default no proxy",
			init: func(t *testing.T, flagSet *flag.FlagSet) {
				flagSet.Set("http-proxy", "http://other-proxy")
				flagSet.Set("https-proxy", "https://other-proxy")
				flagSet.Set("no-proxy", "other-no-proxy-1,other-no-proxy-2")

				flagSet.Set("pod-cidr", "1.1.1.1/24")
				flagSet.Set("service-cidr", "2.2.2.2/24")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:  "http://other-proxy",
				HTTPSProxy: "https://other-proxy",
				NoProxy:    "localhost,127.0.0.1,.default,.local,.svc,kubernetes,kotsadm-rqlite,other-no-proxy-1,other-no-proxy-2,1.1.1.1/24,2.2.2.2/24",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := withProxyFlags(withSubnetCIDRFlags([]cli.Flag{}))
			flagSet := flag.NewFlagSet("test", 0)
			for _, flag := range flags {
				flag.Apply(flagSet)
			}
			if tt.init != nil {
				tt.init(t, flagSet)
			}
			c := cli.NewContext(cli.NewApp(), flagSet, nil)
			got := getProxySpecFromFlags(c)
			assert.Equal(t, tt.want, got)
		})
	}
}
