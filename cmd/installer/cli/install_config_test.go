package cli

import (
	"net"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

// Mock network interface for testing
type mockNetworkLookup struct{}

func (m *mockNetworkLookup) FirstValidIPNet(networkInterface string) (*net.IPNet, error) {
	_, ipnet, _ := net.ParseCIDR("192.168.1.0/24")
	return ipnet, nil
}

// Helper function to create bool pointer
func boolPtr(b bool) *bool {
	return &b
}

func Test_buildInstallFlags_ProxyConfig(t *testing.T) {
	tests := []struct {
		name string
		init func(t *testing.T, flagSet *pflag.FlagSet)
		want *ecv1beta1.ProxySpec
	}{
		{
			name: "no flags set and no env vars should not set proxy",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				// No env vars, no flags
			},
			want: nil,
		},
		{
			name: "lowercase env vars should be used when no flags set",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
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
			name: "uppercase env vars should be used when no flags set and no lowercase vars",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
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
			name: "lowercase should take precedence over uppercase",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
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
			name: "proxy flags should override env vars",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				t.Setenv("http_proxy", "http://lower-proxy")
				t.Setenv("https_proxy", "https://lower-proxy")
				t.Setenv("no_proxy", "lower-no-proxy-1,lower-no-proxy-2")
				t.Setenv("HTTP_PROXY", "http://upper-proxy")
				t.Setenv("HTTPS_PROXY", "https://upper-proxy")
				t.Setenv("NO_PROXY", "upper-no-proxy-1,upper-no-proxy-2")

				flagSet.Set("http-proxy", "http://flag-proxy")
				flagSet.Set("https-proxy", "https://flag-proxy")
				flagSet.Set("no-proxy", "flag-no-proxy-1,flag-no-proxy-2")
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
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				flagSet.Set("http-proxy", "http://flag-proxy")
				flagSet.Set("https-proxy", "https://flag-proxy")
				flagSet.Set("no-proxy", "flag-no-proxy-1,flag-no-proxy-2")

				flagSet.Set("pod-cidr", "1.1.1.1/24")
				flagSet.Set("service-cidr", "2.2.2.2/24")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:       "http://flag-proxy",
				HTTPSProxy:      "https://flag-proxy",
				ProvidedNoProxy: "flag-no-proxy-1,flag-no-proxy-2",
				NoProxy:         "localhost,127.0.0.1,.cluster.local,.svc,169.254.169.254,1.1.1.1/24,2.2.2.2/24,flag-no-proxy-1,flag-no-proxy-2,192.168.1.0/24",
			},
		},
		{
			name: "custom --cidr should be present in the no-proxy",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				flagSet.Set("http-proxy", "http://flag-proxy")
				flagSet.Set("https-proxy", "https://flag-proxy")
				flagSet.Set("no-proxy", "flag-no-proxy-1,flag-no-proxy-2")

				flagSet.Set("cidr", "10.0.0.0/16")
			},
			want: &ecv1beta1.ProxySpec{
				HTTPProxy:       "http://flag-proxy",
				HTTPSProxy:      "https://flag-proxy",
				ProvidedNoProxy: "flag-no-proxy-1,flag-no-proxy-2",
				NoProxy:         "localhost,127.0.0.1,.cluster.local,.svc,169.254.169.254,10.0.0.0/17,10.0.128.0/17,flag-no-proxy-1,flag-no-proxy-2,192.168.1.0/24",
			},
		},
		{
			name: "partial env vars with partial flag vars",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				t.Setenv("http_proxy", "http://lower-proxy")
				// No https_proxy set
				t.Setenv("no_proxy", "lower-no-proxy-1,lower-no-proxy-2")

				// Only set https-proxy flag
				flagSet.Set("https-proxy", "https://flag-proxy")
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
			// Setup flags struct
			flags := &installFlags{
				networkInterface: "eth0", // Skip network interface auto-detection
			}

			// Setup cobra command with flags
			cmd := &cobra.Command{}
			mustAddCIDRFlags(cmd.Flags())
			mustAddProxyFlags(cmd.Flags())

			flagSet := cmd.Flags()
			if tt.init != nil {
				tt.init(t, flagSet)
			}

			// Override the network lookup with our mock
			defaultNetworkLookupImpl = &mockNetworkLookup{}

			err := buildInstallFlags(cmd, flags)
			assert.NoError(t, err, "unexpected error")
			assert.Equal(t, tt.want, flags.proxySpec)
		})
	}
}

func Test_buildInstallFlags_SkipHostPreflightsEnvVar(t *testing.T) {
	tests := []struct {
		name                   string
		envVarValue            string
		flagValue              *bool // nil means not set, true/false means explicitly set
		expectedSkipPreflights bool
	}{
		{
			name:                   "env var set to 1, no flag",
			envVarValue:            "1",
			flagValue:              nil,
			expectedSkipPreflights: true,
		},
		{
			name:                   "env var set to true, no flag",
			envVarValue:            "true",
			flagValue:              nil,
			expectedSkipPreflights: true,
		},
		{
			name:                   "env var set, flag explicitly false (flag takes precedence)",
			envVarValue:            "1",
			flagValue:              boolPtr(false),
			expectedSkipPreflights: false,
		},
		{
			name:                   "env var set, flag explicitly true",
			envVarValue:            "1",
			flagValue:              boolPtr(true),
			expectedSkipPreflights: true,
		},
		{
			name:                   "env var not set, no flag",
			envVarValue:            "",
			flagValue:              nil,
			expectedSkipPreflights: false,
		},
		{
			name:                   "env var not set, flag explicitly false",
			envVarValue:            "",
			flagValue:              boolPtr(false),
			expectedSkipPreflights: false,
		},
		{
			name:                   "env var not set, flag explicitly true",
			envVarValue:            "",
			flagValue:              boolPtr(true),
			expectedSkipPreflights: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variable
			if tt.envVarValue != "" {
				t.Setenv("SKIP_HOST_PREFLIGHTS", tt.envVarValue)
			}

			// Create a mock cobra command to simulate flag behavior
			cmd := &cobra.Command{}
			flags := &installFlags{
				networkInterface: "eth0", // Skip network interface auto-detection
			}

			// Add the flags
			cmd.Flags().BoolVar(&flags.skipHostPreflights, "skip-host-preflights", false, "Skip host preflight checks")
			mustAddCIDRFlags(cmd.Flags())
			mustAddProxyFlags(cmd.Flags())

			// Set the flag if explicitly provided in test
			if tt.flagValue != nil {
				err := cmd.Flags().Set("skip-host-preflights", "true")
				if *tt.flagValue {
					assert.NoError(t, err)
				} else {
					// For false, we need to mark the flag as changed but set to false
					cmd.Flags().Set("skip-host-preflights", "false")
				}
			}

			err := buildInstallFlags(cmd, flags)
			assert.NoError(t, err)

			// Verify the flag was set correctly
			assert.Equal(t, tt.expectedSkipPreflights, flags.skipHostPreflights)
		})
	}
}

func Test_buildInstallFlags_CIDRConfig(t *testing.T) {
	// Compute expected split CIDR values for the default CIDR
	defaultPodCIDR, defaultServiceCIDR, err := newconfig.SplitCIDR(ecv1beta1.DefaultNetworkCIDR)
	assert.NoError(t, err, "failed to split default CIDR")

	tests := []struct {
		name        string
		init        func(t *testing.T, flagSet *pflag.FlagSet)
		expected    *newconfig.CIDRConfig
		expectError bool
	}{
		{
			name: "with pod and service flags",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				flagSet.Set("pod-cidr", "10.0.0.0/24")
				flagSet.Set("service-cidr", "10.1.0.0/24")
			},
			expected: &newconfig.CIDRConfig{
				PodCIDR:     "10.0.0.0/24",
				ServiceCIDR: "10.1.0.0/24",
				GlobalCIDR:  nil,
			},
		},
		{
			name: "with pod flag",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				flagSet.Set("pod-cidr", "10.0.0.0/24")
			},
			expected: &newconfig.CIDRConfig{
				PodCIDR:     "10.0.0.0/24",
				ServiceCIDR: v1beta1.DefaultNetwork().ServiceCIDR,
				GlobalCIDR:  nil,
			},
		},
		{
			name: "with pod, service and cidr flags - should error",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				flagSet.Set("pod-cidr", "10.0.0.0/24")
				flagSet.Set("service-cidr", "10.1.0.0/24")
				flagSet.Set("cidr", "10.2.0.0/24")
			},
			expectError: true,
		},
		{
			name: "with pod and cidr flags - should error",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				flagSet.Set("pod-cidr", "10.0.0.0/24")
				flagSet.Set("cidr", "10.2.0.0/24")
			},
			expectError: true,
		},
		{
			name: "with service flag only",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				flagSet.Set("service-cidr", "10.1.0.0/24")
			},
			expected: &newconfig.CIDRConfig{
				PodCIDR:     v1beta1.DefaultNetwork().PodCIDR,
				ServiceCIDR: "10.1.0.0/24",
				GlobalCIDR:  nil,
			},
		},
		{
			name: "with cidr flag",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				flagSet.Set("cidr", "10.2.0.0/16")
			},
			expected: &newconfig.CIDRConfig{
				PodCIDR:     "10.2.0.0/17",
				ServiceCIDR: "10.2.128.0/17",
				GlobalCIDR:  ptr.To("10.2.0.0/16"),
			},
		},
		{
			name: "with no flags (defaults)",
			init: func(t *testing.T, flagSet *pflag.FlagSet) {
				// No flags set, should use default cidr value and split it
			},
			expected: &newconfig.CIDRConfig{
				PodCIDR:     defaultPodCIDR,
				ServiceCIDR: defaultServiceCIDR,
				GlobalCIDR:  ptr.To(ecv1beta1.DefaultNetworkCIDR),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup flags struct
			flags := &installFlags{
				networkInterface: "eth0", // Skip network interface auto-detection
			}

			// Setup cobra command with flags
			cmd := &cobra.Command{}
			mustAddCIDRFlags(cmd.Flags())
			mustAddProxyFlags(cmd.Flags())

			flagSet := cmd.Flags()
			if tt.init != nil {
				tt.init(t, flagSet)
			}

			err := buildInstallFlags(cmd, flags)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err, "unexpected error")
				assert.Equal(t, tt.expected, flags.cidrConfig)
			}
		})
	}
}
