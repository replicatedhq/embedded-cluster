package cli

import (
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func Test_getProxySpecFromFlags(t *testing.T) {
	tests := []struct {
		name string
		init func(t *testing.T, flagSet *pflag.FlagSet)
		want *ecv1beta1.ProxySpec
	}{
		{
			name: "no flags set should not set proxy",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				t.Setenv("HTTP_PROXY", "http://proxy")
				t.Setenv("HTTPS_PROXY", "https://proxy")
				t.Setenv("NO_PROXY", "no-proxy-1,no-proxy-2")
			},
			want: nil,
		},
		{
			name: "proxy from env flag set should set proxy",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				t.Setenv("HTTP_PROXY", "http://proxy")
				t.Setenv("HTTPS_PROXY", "https://proxy")
				t.Setenv("NO_PROXY", "no-proxy-1,no-proxy-2")

				flagSet.Set("proxy", "true")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:       "http://proxy",
				HTTPSProxy:      "https://proxy",
				ProvidedNoProxy: "no-proxy-1,no-proxy-2",
				NoProxy:         "localhost,127.0.0.1,.cluster.local,.svc,no-proxy-1,no-proxy-2,10.244.0.0/17,10.244.128.0/17",
			},
		},
		{
			name: "proxy from env flag set should not set proxy for lowercase env vars",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				t.Setenv("http_proxy", "http://proxy")
				t.Setenv("https_proxy", "https://proxy")
				t.Setenv("no_proxy", "no-proxy-1,no-proxy-2")

				flagSet.Set("proxy", "true")
			},
			want: nil,
		},
		{
			name: "proxy flags set should set proxy",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				t.Setenv("HTTP_PROXY", "http://proxy")
				t.Setenv("HTTPS_PROXY", "https://proxy")
				t.Setenv("NO_PROXY", "no-proxy-1,no-proxy-2")

				flagSet.Set("http-proxy", "http://other-proxy")
				flagSet.Set("https-proxy", "https://other-proxy")
				flagSet.Set("no-proxy", "other-no-proxy-1,other-no-proxy-2")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:       "http://other-proxy",
				HTTPSProxy:      "https://other-proxy",
				ProvidedNoProxy: "other-no-proxy-1,other-no-proxy-2",
				NoProxy:         "localhost,127.0.0.1,.cluster.local,.svc,other-no-proxy-1,other-no-proxy-2,10.244.0.0/17,10.244.128.0/17",
			},
		},
		{
			name: "proxy flags should override proxy from env, but merge no-proxy",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				t.Setenv("HTTP_PROXY", "http://proxy")
				t.Setenv("HTTPS_PROXY", "https://proxy")
				t.Setenv("NO_PROXY", "no-proxy-1,no-proxy-2")

				flagSet.Set("proxy", "true")
				flagSet.Set("http-proxy", "http://other-proxy")
				flagSet.Set("https-proxy", "https://other-proxy")
				flagSet.Set("no-proxy", "other-no-proxy-1,other-no-proxy-2")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:       "http://other-proxy",
				HTTPSProxy:      "https://other-proxy",
				ProvidedNoProxy: "no-proxy-1,no-proxy-2,other-no-proxy-1,other-no-proxy-2",
				NoProxy:         "localhost,127.0.0.1,.cluster.local,.svc,no-proxy-1,no-proxy-2,other-no-proxy-1,other-no-proxy-2,10.244.0.0/17,10.244.128.0/17",
			},
		},
		{
			name: "pod and service CIDR should override default no proxy",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				flagSet.Set("http-proxy", "http://other-proxy")
				flagSet.Set("https-proxy", "https://other-proxy")
				flagSet.Set("no-proxy", "other-no-proxy-1,other-no-proxy-2")

				flagSet.Set("pod-cidr", "1.1.1.1/24")
				flagSet.Set("service-cidr", "2.2.2.2/24")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:       "http://other-proxy",
				HTTPSProxy:      "https://other-proxy",
				ProvidedNoProxy: "other-no-proxy-1,other-no-proxy-2",
				NoProxy:         "localhost,127.0.0.1,.cluster.local,.svc,other-no-proxy-1,other-no-proxy-2,1.1.1.1/24,2.2.2.2/24",
			},
		},
		{
			name: "custom --cidr should be present in the no-proxy",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				flagSet.Set("http-proxy", "http://other-proxy")
				flagSet.Set("https-proxy", "https://other-proxy")
				flagSet.Set("no-proxy", "other-no-proxy-1,other-no-proxy-2")

				flagSet.Set("cidr", "10.0.0.0/16")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:       "http://other-proxy",
				HTTPSProxy:      "https://other-proxy",
				ProvidedNoProxy: "other-no-proxy-1,other-no-proxy-2",
				NoProxy:         "localhost,127.0.0.1,.cluster.local,.svc,other-no-proxy-1,other-no-proxy-2,10.0.0.0/17,10.0.128.0/17",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			addCIDRFlags(cmd)
			addProxyFlags(cmd)

			flagSet := cmd.Flags()
			if tt.init != nil {
				tt.init(t, flagSet)
			}

			got, err := getProxySpecFromFlags(cmd)
			assert.NoError(t, err, "unexpected error received")
			assert.Equal(t, tt.want, got)
		})
	}
}
