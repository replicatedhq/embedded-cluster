package cli

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func Test_buildInstallFlags_TLSValidation(t *testing.T) {
	tests := []struct {
		name        string
		tlsCertFile string
		tlsKeyFile  string
		wantErr     string
	}{
		{
			name:        "both cert and key provided",
			tlsCertFile: "/path/to/cert.pem",
			tlsKeyFile:  "/path/to/key.pem",
			wantErr:     "",
		},
		{
			name:        "neither cert nor key provided",
			tlsCertFile: "",
			tlsKeyFile:  "",
			wantErr:     "",
		},
		{
			name:        "only cert file provided",
			tlsCertFile: "/path/to/cert.pem",
			tlsKeyFile:  "",
			wantErr:     "both --tls-cert and --tls-key must be provided together",
		},
		{
			name:        "only key file provided",
			tlsCertFile: "",
			tlsKeyFile:  "/path/to/key.pem",
			wantErr:     "both --tls-cert and --tls-key must be provided together",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup flags struct
			flags := &installFlags{
				networkInterface: "eth0", // Skip network interface auto-detection
				tlsCertFile:      tt.tlsCertFile,
				tlsKeyFile:       tt.tlsKeyFile,
			}

			// Setup cobra command with flags
			cmd := &cobra.Command{}
			mustAddCIDRFlags(cmd.Flags())
			mustAddProxyFlags(cmd.Flags())

			err := buildInstallFlags(cmd, flags)

			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_buildInstallFlags_HeadlessValidation(t *testing.T) {
	tests := []struct {
		name          string
		headless      bool
		configValues  string
		adminPassword string
		target        string
		wantErr       string
	}{
		{
			name:          "valid headless flags with valid config file",
			headless:      true,
			configValues:  "/path/to/valid-config.yaml",
			adminPassword: "password123",
			target:        string(apitypes.InstallTargetLinux),
			wantErr:       "",
		},
		{
			name:          "not headless - should pass even with invalid flags",
			headless:      false,
			configValues:  "",
			adminPassword: "",
			target:        string(apitypes.InstallTargetLinux),
			wantErr:       "",
		},
		{
			name:          "missing config values flag",
			headless:      true,
			configValues:  "",
			adminPassword: "password123",
			target:        string(apitypes.InstallTargetLinux),
			wantErr:       "--config-values flag is required for headless installation",
		},
		{
			name:          "missing admin console password",
			headless:      true,
			configValues:  "/path/to/valid-config.yaml",
			adminPassword: "",
			target:        string(apitypes.InstallTargetLinux),
			wantErr:       "--admin-console-password flag is required for headless installation",
		},
		{
			name:          "unsupported target",
			headless:      true,
			configValues:  "/path/to/valid-config.yaml",
			adminPassword: "password123",
			target:        string(apitypes.InstallTargetKubernetes),
			wantErr:       "headless installation only supports --target=linux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Enable V3 for headless validation to work
			t.Setenv("ENABLE_V3", "1")

			// Setup flags struct
			flags := &installFlags{
				networkInterface:     "eth0", // Skip network interface auto-detection
				headless:             tt.headless,
				configValues:         tt.configValues,
				adminConsolePassword: tt.adminPassword,
				target:               tt.target,
			}

			// Setup cobra command with flags
			cmd := &cobra.Command{}
			mustAddCIDRFlags(cmd.Flags())
			mustAddProxyFlags(cmd.Flags())

			// Call buildInstallFlags (this will call validateHeadlessInstallFlags internally if V3 is enabled)
			err := buildInstallFlags(cmd, flags)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_buildInstallConfig_HeadlessValidation(t *testing.T) {
	// Create temporary directory for test files
	tmpDir := t.TempDir()

	// Create a valid config file
	validConfigFile := filepath.Join(tmpDir, "valid-config.yaml")
	validConfigContent := `apiVersion: kots.io/v1beta1
kind: ConfigValues
metadata:
  name: test-config
spec:
  values:
    database_host:
      value: "postgres.example.com"
    database_password:
      value: "secretpassword"`
	err := os.WriteFile(validConfigFile, []byte(validConfigContent), 0644)
	require.NoError(t, err)

	// Create an invalid YAML file
	invalidConfigFile := filepath.Join(tmpDir, "invalid-config.yaml")
	invalidConfigContent := `this is not valid: yaml: content [`
	err = os.WriteFile(invalidConfigFile, []byte(invalidConfigContent), 0644)
	require.NoError(t, err)

	// Create an empty config file
	emptyConfigFile := filepath.Join(tmpDir, "empty-config.yaml")
	emptyConfigContent := `apiVersion: kots.io/v1beta1
kind: ConfigValues
metadata:
  name: test-config`
	err = os.WriteFile(emptyConfigFile, []byte(emptyConfigContent), 0644)
	require.NoError(t, err)

	tests := []struct {
		name             string
		configValues     string
		wantErr          string
		wantConfigValues *kotsv1beta1.ConfigValues
	}{
		{
			name:         "valid headless flags with valid config file",
			configValues: validConfigFile,
			wantErr:      "",
			wantConfigValues: &kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"database_host": {
							Value: "postgres.example.com",
						},
						"database_password": {
							Value: "secretpassword",
						},
					},
				},
			},
		},
		{
			name:         "valid headless flags with empty config file",
			configValues: emptyConfigFile,
			wantErr:      "",
			wantConfigValues: &kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
			},
		},
		{
			name:             "config file not found",
			configValues:     "/nonexistent/config.yaml",
			wantErr:          "config values file not found",
			wantConfigValues: nil,
		},
		{
			name:             "invalid YAML in config file",
			configValues:     invalidConfigFile,
			wantErr:          "config values file is not valid",
			wantConfigValues: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ENABLE_V3", "1")

			flags := &installFlags{
				headless:     true,
				configValues: tt.configValues,
			}

			installCfg, err := buildInstallConfig(flags)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantConfigValues, installCfg.configValues)
			}
		})
	}
}
