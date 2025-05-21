package models

import (
	"net"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock network interface for testing
type mockNetworkLookup struct{}

func (m *mockNetworkLookup) FirstValidIPNet(networkInterface string) (*net.IPNet, error) {
	_, ipnet, _ := net.ParseCIDR("192.168.1.0/24")
	return ipnet, nil
}

// TODO: move to api package
func Test_getProxySpecFromConfig(t *testing.T) {
	tests := []struct {
		name               string
		installationConfig *InstallationConfig
		init               func(t *testing.T)
		want               *ecv1beta1.ProxySpec
	}{
		{
			name:               "config empty and no env vars should not set proxy",
			installationConfig: &InstallationConfig{},
			init: func(t *testing.T) {
				// No env vars
			},
			want: nil,
		},
		{
			name:               "lowercase env vars should be used when config is empty",
			installationConfig: &InstallationConfig{},
			init: func(t *testing.T) {
				t.Setenv("http_proxy", "http://lower-proxy")
				t.Setenv("https_proxy", "https://lower-proxy")
				t.Setenv("no_proxy", "lower-no-proxy-1,lower-no-proxy-2")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:       "http://lower-proxy",
				HTTPSProxy:      "https://lower-proxy",
				ProvidedNoProxy: "lower-no-proxy-1,lower-no-proxy-2",
				NoProxy:         "localhost,127.0.0.1,.cluster.local,.svc,169.254.169.254,10.244.0.0/17,10.244.128.0/17,lower-no-proxy-1,lower-no-proxy-2,192.168.1.0/24",
			},
		},
		{
			name:               "uppercase env vars should be used when config is empty and no lowercase vars",
			installationConfig: &InstallationConfig{},
			init: func(t *testing.T) {
				t.Setenv("HTTP_PROXY", "http://upper-proxy")
				t.Setenv("HTTPS_PROXY", "https://upper-proxy")
				t.Setenv("NO_PROXY", "upper-no-proxy-1,upper-no-proxy-2")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:       "http://upper-proxy",
				HTTPSProxy:      "https://upper-proxy",
				ProvidedNoProxy: "upper-no-proxy-1,upper-no-proxy-2",
				NoProxy:         "localhost,127.0.0.1,.cluster.local,.svc,169.254.169.254,10.244.0.0/17,10.244.128.0/17,upper-no-proxy-1,upper-no-proxy-2,192.168.1.0/24",
			},
		},
		{
			name:               "lowercase should take precedence over uppercase",
			installationConfig: &InstallationConfig{},
			init: func(t *testing.T) {
				t.Setenv("http_proxy", "http://lower-proxy")
				t.Setenv("https_proxy", "https://lower-proxy")
				t.Setenv("no_proxy", "lower-no-proxy-1,lower-no-proxy-2")
				t.Setenv("HTTP_PROXY", "http://upper-proxy")
				t.Setenv("HTTPS_PROXY", "https://upper-proxy")
				t.Setenv("NO_PROXY", "upper-no-proxy-1,upper-no-proxy-2")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:       "http://lower-proxy",
				HTTPSProxy:      "https://lower-proxy",
				ProvidedNoProxy: "lower-no-proxy-1,lower-no-proxy-2",
				NoProxy:         "localhost,127.0.0.1,.cluster.local,.svc,169.254.169.254,10.244.0.0/17,10.244.128.0/17,lower-no-proxy-1,lower-no-proxy-2,192.168.1.0/24",
			},
		},
		{
			name: "config should override env vars",
			installationConfig: &InstallationConfig{
				HTTPProxy:  "http://flag-proxy",
				HTTPSProxy: "https://flag-proxy",
				NoProxy:    "flag-no-proxy-1,flag-no-proxy-2",
			},
			init: func(t *testing.T) {
				t.Setenv("http_proxy", "http://lower-proxy")
				t.Setenv("https_proxy", "https://lower-proxy")
				t.Setenv("no_proxy", "lower-no-proxy-1,lower-no-proxy-2")
				t.Setenv("HTTP_PROXY", "http://upper-proxy")
				t.Setenv("HTTPS_PROXY", "https://upper-proxy")
				t.Setenv("NO_PROXY", "upper-no-proxy-1,upper-no-proxy-2")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:       "http://flag-proxy",
				HTTPSProxy:      "https://flag-proxy",
				ProvidedNoProxy: "flag-no-proxy-1,flag-no-proxy-2",
				NoProxy:         "localhost,127.0.0.1,.cluster.local,.svc,169.254.169.254,10.244.0.0/17,10.244.128.0/17,flag-no-proxy-1,flag-no-proxy-2,192.168.1.0/24",
			},
		},
		{
			name: "pod and service CIDR should override default no proxy",
			installationConfig: &InstallationConfig{
				HTTPProxy:   "http://flag-proxy",
				HTTPSProxy:  "https://flag-proxy",
				NoProxy:     "flag-no-proxy-1,flag-no-proxy-2",
				PodCIDR:     "1.1.1.1/24",
				ServiceCIDR: "2.2.2.2/24",
			},
			init: func(t *testing.T) {
				// No env vars
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:       "http://flag-proxy",
				HTTPSProxy:      "https://flag-proxy",
				ProvidedNoProxy: "flag-no-proxy-1,flag-no-proxy-2",
				NoProxy:         "localhost,127.0.0.1,.cluster.local,.svc,169.254.169.254,1.1.1.1/24,2.2.2.2/24,flag-no-proxy-1,flag-no-proxy-2,192.168.1.0/24",
			},
		},
		{
			name: "global cidr should be present in the no-proxy",
			installationConfig: &InstallationConfig{
				HTTPProxy:  "http://flag-proxy",
				HTTPSProxy: "https://flag-proxy",
				NoProxy:    "flag-no-proxy-1,flag-no-proxy-2",
				GlobalCIDR: "10.0.0.0/16",
			},
			init: func(t *testing.T) {
				// No env vars
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:       "http://flag-proxy",
				HTTPSProxy:      "https://flag-proxy",
				ProvidedNoProxy: "flag-no-proxy-1,flag-no-proxy-2",
				NoProxy:         "localhost,127.0.0.1,.cluster.local,.svc,169.254.169.254,10.0.0.0/17,10.0.128.0/17,flag-no-proxy-1,flag-no-proxy-2,192.168.1.0/24",
			},
		},
		{
			name: "partial env vars with partial config",
			installationConfig: &InstallationConfig{
				// Only set https-proxy flag
				HTTPSProxy: "https://flag-proxy",
			},
			init: func(t *testing.T) {
				t.Setenv("http_proxy", "http://lower-proxy")
				// No https_proxy set
				t.Setenv("no_proxy", "lower-no-proxy-1,lower-no-proxy-2")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:       "http://lower-proxy",
				HTTPSProxy:      "https://flag-proxy",
				ProvidedNoProxy: "lower-no-proxy-1,lower-no-proxy-2",
				NoProxy:         "localhost,127.0.0.1,.cluster.local,.svc,169.254.169.254,10.244.0.0/17,10.244.128.0/17,lower-no-proxy-1,lower-no-proxy-2,192.168.1.0/24",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.init != nil {
				tt.init(t)
			}

			// Override the network lookup with our mock
			defaultNetworkLookupImpl = &mockNetworkLookup{}

			err := tt.installationConfig.setCIDRDefaults()
			require.NoError(t, err, "unexpected error setting cidr defaults")
			tt.installationConfig.setProxyDefaults()

			got, err := getProxySpecFromConfig(*tt.installationConfig)
			require.NoError(t, err, "unexpected error getting proxy spec")
			assert.Equal(t, tt.want, got)
		})
	}
}
