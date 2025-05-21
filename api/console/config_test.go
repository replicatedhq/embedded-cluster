package console

import (
	"net"
	"testing"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/replicatedhq/embedded-cluster/api"
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

// TODO: this should test the API handler
func TestConfig_GetProxySpec(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		init   func(t *testing.T)
		want   *ecv1beta1.ProxySpec
	}{
		{
			name:   "config empty and no env vars should not set proxy",
			config: &Config{},
			init: func(t *testing.T) {
				// No env vars
			},
			want: nil,
		},
		{
			name:   "lowercase env vars should be used when config is empty",
			config: &Config{},
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
			name:   "uppercase env vars should be used when config is empty and no lowercase vars",
			config: &Config{},
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
			name:   "lowercase should take precedence over uppercase",
			config: &Config{},
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
			config: &Config{
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
			config: &Config{
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
			config: &Config{
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
			config: &Config{
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

			config := tt.config
			err := configSetCIDRDefaults(config)
			require.NoError(t, err, "unexpected error setting cidr defaults")
			configSetProxyDefaults(api.NewDiscardLogger(), config)

			got, err := config.GetProxySpec()
			require.NoError(t, err, "unexpected error getting proxy spec")
			assert.Equal(t, tt.want, got)
		})
	}
}

// TODO: this should test the API handler
func Test_configSetCIDRDefaults(t *testing.T) {
	defaultServiceCIDR := k0sv1beta1.DefaultNetwork().ServiceCIDR

	tests := []struct {
		name                 string
		config               Config
		expected             Config
		expectVaidationError bool
	}{
		{
			name: "with pod and service flags",
			config: Config{
				PodCIDR:     "10.0.0.0/24",
				ServiceCIDR: "10.1.0.0/24",
				GlobalCIDR:  "",
			},
			expected: Config{
				PodCIDR:     "10.0.0.0/24",
				ServiceCIDR: "10.1.0.0/24",
				GlobalCIDR:  "",
			},
		},
		{
			name: "with pod flag",
			config: Config{
				PodCIDR: "10.0.0.0/24",
			},
			expected: Config{
				PodCIDR:     "10.0.0.0/24",
				ServiceCIDR: defaultServiceCIDR,
				GlobalCIDR:  "",
			},
		},
		{
			name: "with pod, service and cidr flags",
			config: Config{
				PodCIDR:     "10.1.0.0/17",
				ServiceCIDR: "10.1.128.0/17",
				GlobalCIDR:  "10.1.0.0/16",
			},
			expected: Config{
				PodCIDR:     "10.1.0.0/17",
				ServiceCIDR: "10.1.128.0/17",
				GlobalCIDR:  "10.1.0.0/16",
			},
		},
		{
			name: "with pod and cidr flags",
			config: Config{
				PodCIDR:    "10.0.0.0/17",
				GlobalCIDR: "10.2.0.0/16",
			},
			expected: Config{
				PodCIDR:     "10.0.0.0/17",
				ServiceCIDR: defaultServiceCIDR,
				GlobalCIDR:  "10.2.0.0/16",
			},
			expectVaidationError: true,
		},
		{
			name: "with cidr flag",
			config: Config{
				GlobalCIDR: "10.2.0.0/16",
			},
			expected: Config{
				PodCIDR:     "10.2.0.0/17",
				ServiceCIDR: "10.2.128.0/17",
				GlobalCIDR:  "10.2.0.0/16",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := configSetCIDRDefaults(&test.config)
			require.NoError(t, err)
			assert.Equal(t, test.expected, test.config)

			validateErr := validateConfigCIDR(test.config)
			if test.expectVaidationError {
				assert.Error(t, validateErr)
			} else {
				assert.NoError(t, validateErr)
			}
		})
	}
}
