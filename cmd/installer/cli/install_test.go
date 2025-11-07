package cli

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/cmd/installer/kotscli"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/replicatedhq/embedded-cluster/pkg/addons"
	"github.com/replicatedhq/embedded-cluster/pkg/airgap"
	"github.com/replicatedhq/embedded-cluster/pkg/helm"
	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
	"github.com/replicatedhq/embedded-cluster/pkg/kubeutils"
	"github.com/replicatedhq/embedded-cluster/pkg/metrics"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts"
	"github.com/replicatedhq/embedded-cluster/pkg/prompts/plain"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/replicatedhq/embedded-cluster/web"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	helmcli "helm.sh/helm/v3/pkg/cli"
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
				ServiceCIDR: k0sv1beta1.DefaultNetwork().ServiceCIDR,
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
				PodCIDR:     k0sv1beta1.DefaultNetwork().PodCIDR,
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

func Test_buildInstallConfig_License(t *testing.T) {
	// Create a temporary directory for test license files
	tmpdir := t.TempDir()

	// Valid test license data (YAML format for a kotsv1beta1.License)
	validLicenseData := `apiVersion: kots.io/v1beta1
kind: License
metadata:
  name: test-license
spec:
  licenseID: test-license-id
  appSlug: test-app
  channelID: test-channel-id
  channelName: Test Channel
  customerName: Test Customer
  endpoint: https://replicated.app
  entitlements:
    expires_at:
      title: Expiration
      value: "2030-01-01T00:00:00Z"
      valueType: String
  isEmbeddedClusterDownloadEnabled: true`

	// Create a valid license file
	validLicensePath := filepath.Join(tmpdir, "valid-license.yaml")
	err := os.WriteFile(validLicensePath, []byte(validLicenseData), 0644)
	require.NoError(t, err)

	tests := []struct {
		name          string
		licenseFile   string
		wantErr       string
		expectLicense bool
	}{
		{
			name:          "no license file provided",
			licenseFile:   "",
			wantErr:       "",
			expectLicense: false,
		},
		{
			name:          "license file does not exist",
			licenseFile:   filepath.Join(tmpdir, "nonexistent.yaml"),
			wantErr:       "failed to read license file",
			expectLicense: false,
		},
		{
			name: "invalid license file - not YAML",
			licenseFile: func() string {
				invalidPath := filepath.Join(tmpdir, "invalid-license.txt")
				os.WriteFile(invalidPath, []byte("this is not a valid license file"), 0644)
				return invalidPath
			}(),
			wantErr:       "failed to parse the license file",
			expectLicense: false,
		},
		{
			name: "invalid license file - wrong kind",
			licenseFile: func() string {
				wrongKindPath := filepath.Join(tmpdir, "wrong-kind.yaml")
				wrongKindData := `apiVersion: v1
kind: ConfigMap
metadata:
  name: not-a-license`
				os.WriteFile(wrongKindPath, []byte(wrongKindData), 0644)
				return wrongKindPath
			}(),
			wantErr:       "failed to parse the license file",
			expectLicense: false,
		},
		{
			name: "corrupt license file - invalid YAML",
			licenseFile: func() string {
				corruptPath := filepath.Join(tmpdir, "corrupt-license.yaml")
				corruptData := `apiVersion: kots.io/v1beta1
kind: License
metadata:
  name: test
spec:
  this is not valid yaml: [[[`
				os.WriteFile(corruptPath, []byte(corruptData), 0644)
				return corruptPath
			}(),
			wantErr:       "failed to parse the license file",
			expectLicense: false,
		},
		{
			name:          "valid license file",
			licenseFile:   validLicensePath,
			wantErr:       "",
			expectLicense: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := &installFlags{
				licenseFile: tt.licenseFile,
			}

			installCfg, err := buildInstallConfig(flags)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)

				if tt.expectLicense {
					assert.NotEmpty(t, installCfg.licenseBytes, "License bytes should be populated")
					assert.NotNil(t, installCfg.license, "License should be parsed")
					assert.Equal(t, "test-license-id", installCfg.license.Spec.LicenseID)
					assert.Equal(t, "test-app", installCfg.license.Spec.AppSlug)
				} else {
					assert.Empty(t, installCfg.licenseBytes, "License bytes should be empty")
					assert.Nil(t, installCfg.license, "License should be nil")
				}
			}
		})
	}
}

func Test_buildInstallConfig_TLS(t *testing.T) {
	// Create a temporary directory for test certificates
	tmpdir := t.TempDir()
	certPath, keyPath := writeTestCertAndKey(t, tmpdir)

	// Create a valid config file for headless tests
	validConfigFile := filepath.Join(tmpdir, "valid-config.yaml")
	validConfigContent := `apiVersion: kots.io/v1beta1
kind: ConfigValues
metadata:
  name: test-config
spec:
  values:
    database_host:
      value: "postgres.example.com"`
	err := os.WriteFile(validConfigFile, []byte(validConfigContent), 0644)
	require.NoError(t, err)

	tests := []struct {
		name        string
		tlsCertFile string
		tlsKeyFile  string
		headless    bool
		assumeYes   bool
		wantErr     string
		expectTLS   bool
	}{
		{
			name:        "no TLS files provided",
			tlsCertFile: "",
			tlsKeyFile:  "",
			wantErr:     "",
			expectTLS:   false,
		},
		{
			name:        "cert file does not exist",
			tlsCertFile: filepath.Join(tmpdir, "nonexistent.pem"),
			tlsKeyFile:  keyPath,
			wantErr:     "failed to read TLS certificate",
			expectTLS:   false,
		},
		{
			name:        "key file does not exist",
			tlsCertFile: certPath,
			tlsKeyFile:  filepath.Join(tmpdir, "nonexistent.key"),
			wantErr:     "failed to read TLS key",
			expectTLS:   false,
		},
		{
			name: "invalid cert file",
			tlsCertFile: func() string {
				invalidCertPath := filepath.Join(tmpdir, "invalid-cert.pem")
				os.WriteFile(invalidCertPath, []byte("invalid cert data"), 0644)
				return invalidCertPath
			}(),
			tlsKeyFile: keyPath,
			wantErr:    "failed to parse TLS certificate",
			expectTLS:  false,
		},
		{
			name:        "valid cert and key files",
			tlsCertFile: certPath,
			tlsKeyFile:  keyPath,
			wantErr:     "",
			expectTLS:   true,
		},
		{
			name:        "headless with valid TLS cert and key",
			tlsCertFile: certPath,
			tlsKeyFile:  keyPath,
			headless:    true,
			wantErr:     "",
			expectTLS:   true,
		},
		{
			name:        "headless with no TLS cert and key and assumeYes=false - cancels installation",
			tlsCertFile: "",
			tlsKeyFile:  "",
			headless:    true,
			wantErr:     "failed to get confirmation",
			expectTLS:   false,
		},
		{
			name:        "headless with no TLS cert and key and assumeYes=true - generates self-signed cert",
			tlsCertFile: "",
			tlsKeyFile:  "",
			headless:    true,
			assumeYes:   true,
			wantErr:     "",
			expectTLS:   true, // Self-signed cert is automatically generated in headless mode
		},
		{
			name: "headless with invalid cert file",
			tlsCertFile: func() string {
				invalidCertPath := filepath.Join(tmpdir, "invalid-headless-cert.pem")
				os.WriteFile(invalidCertPath, []byte("invalid cert data"), 0644)
				return invalidCertPath
			}(),
			tlsKeyFile: keyPath,
			headless:   true,
			wantErr:    "failed to parse TLS certificate",
			expectTLS:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := &installFlags{
				tlsCertFile: tt.tlsCertFile,
				tlsKeyFile:  tt.tlsKeyFile,
				headless:    tt.headless,
				assumeYes:   tt.assumeYes,
			}

			if tt.headless {
				t.Setenv("ENABLE_V3", "1")

				flags.configValues = validConfigFile
				flags.adminConsolePassword = "password123"
			}

			installCfg, err := buildInstallConfig(flags)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)

				if tt.expectTLS {
					assert.NotEmpty(t, installCfg.tlsCertBytes, "TLS cert bytes should be populated")
					assert.NotEmpty(t, installCfg.tlsKeyBytes, "TLS key bytes should be populated")
					assert.NotNil(t, installCfg.tlsCert.Certificate, "TLS cert should be loaded")
				} else {
					assert.Empty(t, installCfg.tlsCertBytes, "TLS cert bytes should be empty")
					assert.Empty(t, installCfg.tlsKeyBytes, "TLS key bytes should be empty")
				}
			}
		})
	}
}

func writeTestCertAndKey(t *testing.T, tmpdir string) (string, string) {
	// Create valid test certificate and key files
	certPath := filepath.Join(tmpdir, "test-cert.pem")
	keyPath := filepath.Join(tmpdir, "test-key.pem")

	// Valid test certificate and key data
	certData := `-----BEGIN CERTIFICATE-----
MIIDizCCAnOgAwIBAgIUJaAILNY7l9MR4mfMP4WiUObo6TIwDQYJKoZIhvcNAQEL
BQAwVTELMAkGA1UEBhMCVVMxDTALBgNVBAgMBFRlc3QxDTALBgNVBAcMBFRlc3Qx
DTALBgNVBAoMBFRlc3QxGTAXBgNVBAMMEHRlc3QuZXhhbXBsZS5jb20wHhcNMjUw
ODE5MTcwNTU4WhcNMjYwODE5MTcwNTU4WjBVMQswCQYDVQQGEwJVUzENMAsGA1UE
CAwEVGVzdDENMAsGA1UEBwwEVGVzdDENMAsGA1UECgwEVGVzdDEZMBcGA1UEAwwQ
dGVzdC5leGFtcGxlLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
AMhkRyxUJE4JLrTbqq/Etdvd2osmkZJA5GXCRkWcGLBppNNqO1v8K0zy5dV9jgno
gjeQD2nTqZ++vmzR3wPObeB6MJY+2SYtFHvnT3G9HR4DcSX3uHUOBDjbUsW0OT6z
weT3t3eTVqNIY96rZRHz9VYrdC4EPlWyfoYTCHceZey3AqSgHWnHIxVaATWT/LFQ
yvRRlEBNf7/M5NX0qis91wKgGwe6u+P/ebmT1cXURufM0jSAMUbDIqr73Qq5m6t4
fv6/8XKAiVpA1VcACvR79kTi6hYMls88ShHuYLJK175ZQfkeJx77TI/UebALL9CZ
SCI1B08SMZOsr9GQMOKNIl8CAwEAAaNTMFEwHQYDVR0OBBYEFCQWAH7mJ0w4Iehv
PL72t8GCJ90uMB8GA1UdIwQYMBaAFCQWAH7mJ0w4IehvPL72t8GCJ90uMA8GA1Ud
EwEB/wQFMAMBAf8wDQYJKoZIhvcNAQELBQADggEBAFfEICcE4eFZkRfjcEkvrJ3T
KmMikNP2nPXv3h5Ie0DpprejPkDyOWe+UJBanYwAf8xXVwRTmE5PqQhEik2zTBlN
N745Izq1cUYIlyt9GHHycx384osYHKkGE9lAPEvyftlc9hCLSu/FVQ3+8CGwGm9i
cFNYLx/qrKkJxT0Lohi7VCAf7+S9UWjIiLaETGlejm6kPNLRZ0VoxIPgUmqePXfp
6gY5FSIzvH1kZ+bPZ3nqsGyT1l7TsubeTPDDGhpKgIFzcJX9WeY//bI4q1SpU1Fl
koNnBhDuuJxjiafIFCz4qVlf0kmRrz4jeXGXym8IjxUq0EpMgxGuSIkguPKiwFQ=
-----END CERTIFICATE-----`

	keyData := `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDIZEcsVCROCS60
26qvxLXb3dqLJpGSQORlwkZFnBiwaaTTajtb/CtM8uXVfY4J6II3kA9p06mfvr5s
0d8Dzm3gejCWPtkmLRR7509xvR0eA3El97h1DgQ421LFtDk+s8Hk97d3k1ajSGPe
q2UR8/VWK3QuBD5Vsn6GEwh3HmXstwKkoB1pxyMVWgE1k/yxUMr0UZRATX+/zOTV
9KorPdcCoBsHurvj/3m5k9XF1EbnzNI0gDFGwyKq+90KuZureH7+v/FygIlaQNVX
AAr0e/ZE4uoWDJbPPEoR7mCySte+WUH5Hice+0yP1HmwCy/QmUgiNQdPEjGTrK/R
kDDijSJfAgMBAAECggEAHnl1g23GWaG22yU+110cZPPfrOKwJ6Q7t6fsRODAtm9S
dB5HKa13LkwQHL/rzmDwEKAVX/wi4xrAXc8q0areddFPO0IShuY7I76hC8R9PZe7
aNE72X1IshbUhyFpxTnUBkyPt50OA2XaXj4FcE3/5NtV3zug+SpcaGpTkr3qNS24
0Qf5X8AA1STec81c4BaXc8GgLsXz/4kWUSiwK0fjXcIpHkW28gtUyVmYu3FAPSdo
4bKdbqNUiYxF+JYLCQ9PyvFAqy7EhFLM4QkMICnSBNqNCPq3hVOr8K4V9luNnAmS
oU5gEHXmGM8a+kkdvLoZn3dO5tRk8ctV0vnLMYnXrQKBgQDl4/HDbv3oMiqS9nJK
+vQ7/yzLUb00fVzvWbvSLdEfGCgbRlDRKkNMgI5/BnFTJcbG5o3rIdBW37FY3iAy
p4iIm+VGiDz4lFApAQdiQXk9d2/mfB9ZVryUsKskvk6WTjom6+BRSvakqe2jIa/i
udnMFNGkJj6HzZqss1LKDiR5DQKBgQDfJqj5AlCyNUxjokWMH0BapuBVSHYZnxxD
xR5xX/5Q5fKDBpp4hMn8vFS4L8a5mCOBUPbuxEj7KY0Ho5bqYWmt+HyxP5TvDS9h
ZqgDdJuWdLB4hfzlUKekufFrpALvUT4AbmYdQ+ufkggU0mWGCfKaijlk4Hy/VRH7
w5ConbJWGwKBgADkF0XIoldKCnwzVFISEuxAmu3WzULs0XVkBaRU5SCXuWARr7J/
1W7weJzpa3sFBHY04ovsv5/2kftkMP/BQng1EnhpgsL74Cuog1zQICYq1lYwWPbB
rU1uOduUmT1f5D3OYDowbjBJMFCXitT4H235Dq7yLv/bviO5NjLuRxnpAoGBAJBj
LnA4jEhS7kOFiuSYkAZX9c2Y3jnD1wEOuZz4VNC5iMo46phSq3Np1JN87mPGSirx
XWWvAd3py8QGmK69KykTIHN7xX1MFb07NDlQKSAYDttdLv6dymtumQRiEjgRZEHZ
LR+AhCQy1CHM5T3uj9ho2awpCO6wN7uklaRUrUDDAoGBAK/EPsIxm5yj+kFIc/qk
SGwCw13pfbshh9hyU6O//h3czLnN9dgTllfsC7qqxsgrMCVZO9ZIfh5eb44+p7Id
r3glM4yhSJwf/cAWmt1A7DGOYnV7FF2wkDJJPX/Vag1uEsqrzwnAdFBymK5dwDsu
oxhVqyhpk86rf0rT5DcD/sBw
-----END PRIVATE KEY-----`

	err := os.WriteFile(certPath, []byte(certData), 0644)
	require.NoError(t, err)
	err = os.WriteFile(keyPath, []byte(keyData), 0644)
	require.NoError(t, err)

	return certPath, keyPath
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
				assumeYes:    true,
				headless:     true,
				configValues: tt.configValues,
			}

			installCfg, err := buildInstallConfig(flags)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantConfigValues, installCfg.configValues)
			}
		})
	}
}

func Test_buildRuntimeConfig(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		flags       *installFlags
		installCfg  *installConfig
		sslCertFile string
		wantErr     bool
		validate    func(*testing.T, runtimeconfig.RuntimeConfig)
	}{
		{
			name: "all ports and network settings set",
			flags: &installFlags{
				adminConsolePort:        8800,
				managerPort:             8801,
				localArtifactMirrorPort: 8802,
				proxySpec:               nil,
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "10.0.0.0/24",
					ServiceCIDR: "10.1.0.0/24",
					GlobalCIDR:  stringPtr("10.0.0.0/16"),
				},
			},
			sslCertFile: filepath.Join(tmpDir, "ca-certificates.crt"),
			installCfg:  &installConfig{},
			wantErr:     false,
			validate: func(t *testing.T, rc runtimeconfig.RuntimeConfig) {
				req := require.New(t)
				req.Equal(8800, rc.AdminConsolePort())
				req.Equal(8801, rc.ManagerPort())
				req.Equal(8802, rc.LocalArtifactMirrorPort())
				req.Equal("10.0.0.0/24", rc.PodCIDR())
				req.Equal("10.1.0.0/24", rc.ServiceCIDR())
				req.Equal("10.0.0.0/16", rc.GlobalCIDR())
				req.Equal(filepath.Join(tmpDir, "ca-certificates.crt"), rc.HostCABundlePath())
			},
		},
		{
			name: "with proxy spec",
			flags: &installFlags{
				adminConsolePort:        30000,
				managerPort:             30001,
				localArtifactMirrorPort: 30002,
				proxySpec: &ecv1beta1.ProxySpec{
					HTTPProxy:  "http://proxy.example.com:8080",
					HTTPSProxy: "https://proxy.example.com:8080",
				},
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "192.168.0.0/16",
					ServiceCIDR: "10.96.0.0/12",
					GlobalCIDR:  nil,
				},
			},
			installCfg: &installConfig{},
			wantErr:    false,
			validate: func(t *testing.T, rc runtimeconfig.RuntimeConfig) {
				req := require.New(t)
				req.Equal(30000, rc.AdminConsolePort())
				req.Equal(30001, rc.ManagerPort())
				req.Equal(30002, rc.LocalArtifactMirrorPort())
				req.Equal("192.168.0.0/16", rc.PodCIDR())
				req.Equal("10.96.0.0/12", rc.ServiceCIDR())
				proxySpec := rc.ProxySpec()
				req.NotNil(proxySpec)
				req.Equal("http://proxy.example.com:8080", proxySpec.HTTPProxy)
				req.Equal("https://proxy.example.com:8080", proxySpec.HTTPSProxy)
			},
		},
		{
			name: "with global CIDR",
			flags: &installFlags{
				adminConsolePort:        8800,
				managerPort:             8801,
				localArtifactMirrorPort: 8802,
				proxySpec:               nil,
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "10.0.0.0/24",
					ServiceCIDR: "10.1.0.0/24",
					GlobalCIDR:  stringPtr("10.0.0.0/16"),
				},
			},
			installCfg: &installConfig{},
			wantErr:    false,
			validate: func(t *testing.T, rc runtimeconfig.RuntimeConfig) {
				req := require.New(t)
				req.Equal("10.0.0.0/16", rc.GlobalCIDR())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			if tt.sslCertFile != "" {
				err := os.WriteFile(tt.sslCertFile, []byte("test cert"), 0644)
				require.NoError(t, err)

				t.Setenv("SSL_CERT_FILE", tt.sslCertFile)
				defer os.Unsetenv("SSL_CERT_FILE")
			}

			// Create a temporary directory for dataDir testing
			tt.flags.dataDir = tmpDir
			absoluteDataDir, err := filepath.Abs(tmpDir)
			req.NoError(err)

			rc := runtimeconfig.New(nil)

			err = buildRuntimeConfig(tt.flags, tt.installCfg, rc)

			if tt.wantErr {
				req.Error(err)
				return
			}

			req.NoError(err)
			req.Equal(absoluteDataDir, rc.Get().DataDir)
			req.NotEmpty(rc.HostCABundlePath())

			if tt.validate != nil {
				tt.validate(t, rc)
			}
		})
	}
}

func Test_buildKubernetesInstallation(t *testing.T) {
	tests := []struct {
		name     string
		flags    *installFlags
		wantErr  bool
		validate func(*testing.T, kubernetesinstallation.Installation)
	}{
		{
			name: "all values set",
			flags: &installFlags{
				adminConsolePort: 8800,
				managerPort:      8801,
				proxySpec: &ecv1beta1.ProxySpec{
					HTTPProxy:       "http://proxy.example.com:8080",
					HTTPSProxy:      "https://proxy.example.com:8080",
					NoProxy:         "example.com,192.168.0.0/16",
					ProvidedNoProxy: "provided-no-proxy.example.com",
				},
				kubernetesEnvSettings: helmcli.New(),
			},
			wantErr: false,
			validate: func(t *testing.T, ki kubernetesinstallation.Installation) {
				req := require.New(t)
				req.Equal(8800, ki.AdminConsolePort())
				req.Equal(8801, ki.ManagerPort())
				proxySpec := ki.ProxySpec()
				req.NotNil(proxySpec)
				req.Equal("http://proxy.example.com:8080", proxySpec.HTTPProxy)
				req.Equal("https://proxy.example.com:8080", proxySpec.HTTPSProxy)
				req.Equal("example.com,192.168.0.0/16", proxySpec.NoProxy)
				req.Equal("provided-no-proxy.example.com", proxySpec.ProvidedNoProxy)
				req.NotNil(ki.GetKubernetesEnvSettings())
			},
		},
		{
			name: "minimal values",
			flags: &installFlags{
				adminConsolePort:      30000,
				managerPort:           30001,
				proxySpec:             nil,
				kubernetesEnvSettings: helmcli.New(),
			},
			wantErr: false,
			validate: func(t *testing.T, ki kubernetesinstallation.Installation) {
				req := require.New(t)
				req.Equal(30000, ki.AdminConsolePort())
				req.Equal(30001, ki.ManagerPort())
				req.Nil(ki.ProxySpec())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			ki := kubernetesinstallation.New(nil)

			err := buildKubernetesInstallation(tt.flags, ki)

			if tt.wantErr {
				req.Error(err)
				return
			}

			req.NoError(err)

			if tt.validate != nil {
				tt.validate(t, ki)
			}
		})
	}
}

func Test_buildMetricsReporter(t *testing.T) {
	tests := []struct {
		name       string
		cmd        *cobra.Command
		installCfg *installConfig
		validate   func(*testing.T, *installReporter)
	}{
		{
			name: "all values set with flags",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{}
				cmd.Use = "install"
				cmd.Flags().String("flag1", "value1", "")
				cmd.Flags().String("flag2", "value2", "")
				return cmd
			}(),
			installCfg: &installConfig{
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						LicenseID: "license-123",
						AppSlug:   "my-app",
					},
				},
				clusterID: "cluster-456",
			},
			validate: func(t *testing.T, reporter *installReporter) {
				req := require.New(t)
				req.Equal("license-123", reporter.licenseID)
				req.Equal("my-app", reporter.appSlug)
				req.NotNil(reporter.reporter)
			},
		},
		{
			name: "minimal values without flags",
			cmd: func() *cobra.Command {
				cmd := &cobra.Command{}
				cmd.Use = "install"
				return cmd
			}(),
			installCfg: &installConfig{
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						LicenseID: "license-789",
						AppSlug:   "simple-app",
					},
				},
				clusterID: "cluster-012",
			},
			validate: func(t *testing.T, reporter *installReporter) {
				req := require.New(t)
				req.Equal("license-789", reporter.licenseID)
				req.Equal("simple-app", reporter.appSlug)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := buildMetricsReporter(tt.cmd, tt.installCfg)

			if tt.validate != nil {
				tt.validate(t, reporter)
			}
		})
	}
}

func Test_buildAPIOptions(t *testing.T) {
	tests := []struct {
		name            string
		flags           installFlags
		installCfg      *installConfig
		rc              runtimeconfig.RuntimeConfig
		ki              kubernetesinstallation.Installation
		metricsReporter metrics.ReporterInterface
		wantErr         bool
		validate        func(*testing.T, apiOptions)
	}{
		{
			name: "all options set",
			flags: installFlags{
				adminConsolePassword: "password123",
				target:               "linux",
				hostname:             "example.com",
				managerPort:          8800,
				headless:             false,
				ignoreHostPreflights: true,
				airgapBundle:         "/path/to/bundle.airgap",
			},
			installCfg: &installConfig{
				licenseBytes: []byte("license-data"),
				tlsCertBytes: []byte("cert-data"),
				tlsKeyBytes:  []byte("key-data"),
				clusterID:    "cluster-123",
				airgapMetadata: &airgap.AirgapMetadata{
					AirgapInfo: &kotsv1beta1.Airgap{},
				},
				embeddedAssetsSize: 1024 * 1024,
				configValues: &kotsv1beta1.ConfigValues{
					Spec: kotsv1beta1.ConfigValuesSpec{
						Values: map[string]kotsv1beta1.ConfigValue{
							"key1": {Value: "value1"},
							"key2": {Value: "value2"},
						},
					},
				},
				endUserConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{},
				},
			},
			rc: func() runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(nil)
				rc.SetAdminConsolePort(30303)
				return rc
			}(),
			ki: func() kubernetesinstallation.Installation {
				ki := kubernetesinstallation.New(nil)
				ki.SetAdminConsolePort(30304)
				return ki
			}(),
			metricsReporter: &metrics.MockReporter{},
			wantErr:         false,
			validate: func(t *testing.T, opts apiOptions) {
				req := require.New(t)
				req.Equal(apitypes.InstallTargetLinux, opts.InstallTarget)
				req.Equal("password123", opts.Password)
				req.NotEmpty(opts.PasswordHash)
				err := bcrypt.CompareHashAndPassword(opts.PasswordHash, []byte("password123"))
				req.NoError(err)
				req.Equal([]byte("cert-data"), opts.TLSConfig.CertBytes)
				req.Equal([]byte("key-data"), opts.TLSConfig.KeyBytes)
				req.Equal("example.com", opts.TLSConfig.Hostname)
				req.Equal([]byte("license-data"), opts.License)
				req.Equal("/path/to/bundle.airgap", opts.AirgapBundle)
				req.NotNil(opts.AirgapMetadata)
				req.Equal(int64(1024*1024), opts.EmbeddedAssetsSize)
				req.Equal(apitypes.AppConfigValues{"key1": {Value: "value1"}, "key2": {Value: "value2"}}, opts.ConfigValues)
				req.NotNil(opts.EndUserConfig)
				req.Equal("cluster-123", opts.ClusterID)
				req.Equal(apitypes.ModeInstall, opts.Mode)
				req.Equal(30303, opts.RuntimeConfig.AdminConsolePort())
				req.Equal(30304, opts.Installation.AdminConsolePort())
				req.Equal(false, opts.RequiresInfraUpgrade)
				req.Equal(8800, opts.ManagerPort)
				req.Equal(false, opts.Headless)
				req.Equal(true, opts.AllowIgnoreHostPreflights)
				req.Equal(web.ModeInstall, opts.WebMode)
			},
		},
		{
			name: "minimal options",
			flags: installFlags{
				adminConsolePassword: "pass",
				target:               "kubernetes",
				hostname:             "",
				managerPort:          30000,
				headless:             true,
				ignoreHostPreflights: false,
			},
			installCfg: &installConfig{
				clusterID: "cluster-123",
			},
			rc:              runtimeconfig.New(nil),
			ki:              kubernetesinstallation.New(nil),
			metricsReporter: &metrics.MockReporter{},
			wantErr:         false,
			validate: func(t *testing.T, opts apiOptions) {
				req := require.New(t)
				req.Equal(apitypes.InstallTargetKubernetes, opts.InstallTarget)
				req.Equal("pass", opts.Password)
				req.NotEmpty(opts.PasswordHash)
				req.Equal("", opts.TLSConfig.Hostname)
				req.Equal(30000, opts.ManagerPort)
				req.Equal(true, opts.Headless)
				req.Equal(false, opts.AllowIgnoreHostPreflights)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			opts, err := buildAPIOptions(tt.flags, tt.installCfg, tt.rc, tt.ki, tt.metricsReporter)

			if tt.wantErr {
				req.Error(err)
				return
			}

			req.NoError(err)

			if tt.validate != nil {
				tt.validate(t, opts)
			}
		})
	}
}

func Test_buildHelmClientOptions(t *testing.T) {
	dataDir := t.TempDir()

	tests := []struct {
		name       string
		installCfg *installConfig
		rc         runtimeconfig.RuntimeConfig
		validate   func(*testing.T, runtimeconfig.RuntimeConfig, helm.HelmOptions)
	}{
		{
			name: "airgap mode",
			installCfg: &installConfig{
				isAirgap: true,
			},
			rc: func() runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(nil)
				rc.SetDataDir(dataDir)
				return rc
			}(),
			validate: func(t *testing.T, rc runtimeconfig.RuntimeConfig, opts helm.HelmOptions) {
				req := require.New(t)
				req.Equal(rc.PathToEmbeddedClusterBinary("helm"), opts.HelmPath)
				req.NotNil(opts.KubernetesEnvSettings)
				req.NotEmpty(opts.AirgapPath)
			},
		},
		{
			name: "non-airgap mode",
			installCfg: &installConfig{
				isAirgap: false,
			},
			rc: func() runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(nil)
				rc.SetDataDir(dataDir)
				return rc
			}(),
			validate: func(t *testing.T, rc runtimeconfig.RuntimeConfig, opts helm.HelmOptions) {
				req := require.New(t)
				req.Equal(rc.PathToEmbeddedClusterBinary("helm"), opts.HelmPath)
				req.NotNil(opts.KubernetesEnvSettings)
				req.Empty(opts.AirgapPath)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildHelmClientOptions(tt.installCfg, tt.rc)

			if tt.validate != nil {
				tt.validate(t, tt.rc, opts)
			}
		})
	}
}

func Test_buildKotsInstallOptions(t *testing.T) {
	tests := []struct {
		name             string
		installCfg       *installConfig
		flags            installFlags
		kotsadmNamespace string
		loading          *spinner.MessageWriter
		validate         func(*testing.T, kotscli.InstallOptions)
	}{
		{
			name: "all options set",
			installCfg: &installConfig{
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						AppSlug: "my-app",
					},
				},
				licenseBytes: []byte("license-data"),
				clusterID:    "test-cluster-id",
			},
			flags: installFlags{
				airgapBundle:        "/path/to/bundle.airgap",
				configValues:        "/path/to/config.yaml",
				ignoreAppPreflights: true,
			},
			kotsadmNamespace: "kotsadm",
			loading:          &spinner.MessageWriter{},
			validate: func(t *testing.T, opts kotscli.InstallOptions) {
				req := require.New(t)
				req.Equal("my-app", opts.AppSlug)
				req.Equal([]byte("license-data"), opts.License)
				req.Equal("kotsadm", opts.Namespace)
				req.Equal("test-cluster-id", opts.ClusterID)
				req.Equal("/path/to/bundle.airgap", opts.AirgapBundle)
				req.Equal("/path/to/config.yaml", opts.ConfigValuesFile)
				req.Equal(true, opts.SkipPreflights)
				req.NotEmpty(opts.ReplicatedAppEndpoint)
				req.NotNil(opts.Stdout)
			},
		},
		{
			name: "minimal options",
			installCfg: &installConfig{
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						AppSlug: "simple-app",
					},
				},
				licenseBytes: []byte("license-data"),
				clusterID:    "cluster-123",
			},
			flags: installFlags{
				airgapBundle:        "",
				configValues:        "",
				ignoreAppPreflights: false,
			},
			kotsadmNamespace: "default",
			loading:          nil,
			validate: func(t *testing.T, opts kotscli.InstallOptions) {
				req := require.New(t)
				req.Equal("simple-app", opts.AppSlug)
				req.Equal("default", opts.Namespace)
				req.Equal("", opts.AirgapBundle)
				req.Equal("", opts.ConfigValuesFile)
				req.Equal(false, opts.SkipPreflights)
				req.Nil(opts.Stdout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildKotsInstallOptions(tt.installCfg, tt.flags, tt.kotsadmNamespace, tt.loading)

			if tt.validate != nil {
				tt.validate(t, opts)
			}
		})
	}
}

func Test_buildAddonInstallOpts(t *testing.T) {
	// Set up release data with embedded cluster config for testing
	err := release.SetReleaseDataForTests(map[string][]byte{
		"embedded-cluster-config.yaml": []byte(`
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
metadata:
  name: "testconfig"
spec:
  roles:
    controller:
      name: controller-test
`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		release.SetReleaseDataForTests(nil)
	})

	tests := []struct {
		name             string
		flags            installFlags
		installCfg       *installConfig
		rc               runtimeconfig.RuntimeConfig
		kotsadmNamespace string
		loading          **spinner.MessageWriter
		validate         func(*testing.T, *addons.InstallOptions, runtimeconfig.RuntimeConfig, *installConfig)
	}{
		{
			name: "all features enabled",
			flags: installFlags{
				adminConsolePassword: "password123",
				airgapBundle:         "/path/to/bundle.airgap",
				hostname:             "example.com",
			},
			installCfg: &installConfig{
				clusterID: "cluster-123",
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						IsDisasterRecoverySupported:       true,
						IsEmbeddedClusterMultiNodeEnabled: true,
					},
				},
				tlsCertBytes: []byte("cert-data"),
				tlsKeyBytes:  []byte("key-data"),
				endUserConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{},
				},
			},
			rc: func(t *testing.T) runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(nil)
				tmpDir := t.TempDir()
				rc.SetDataDir(tmpDir)
				rc.SetAdminConsolePort(8800)
				rc.SetProxySpec(&ecv1beta1.ProxySpec{
					HTTPProxy:  "http://proxy.example.com:8080",
					HTTPSProxy: "https://proxy.example.com:8080",
				})
				rc.SetHostCABundlePath("/etc/ssl/certs/ca-bundle.crt")
				rc.SetNetworkSpec(ecv1beta1.NetworkSpec{
					ServiceCIDR: "10.96.0.0/12",
				})
				return rc
			}(t),
			kotsadmNamespace: "kotsadm",
			loading: func() **spinner.MessageWriter {
				loading := &spinner.MessageWriter{}
				return &loading
			}(),
			validate: func(t *testing.T, opts *addons.InstallOptions, rc runtimeconfig.RuntimeConfig, installCfg *installConfig) {
				req := require.New(t)
				req.Equal("cluster-123", opts.ClusterID)
				req.Equal("password123", opts.AdminConsolePwd)
				req.Equal(8800, opts.AdminConsolePort)
				req.Equal(true, opts.IsAirgap)
				req.Equal("example.com", opts.Hostname)
				req.Equal([]byte("cert-data"), opts.TLSCertBytes)
				req.Equal([]byte("key-data"), opts.TLSKeyBytes)
				req.Equal(true, opts.DisasterRecoveryEnabled)
				req.Equal(true, opts.IsMultiNodeEnabled)
				req.Equal("kotsadm", opts.KotsadmNamespace)
				req.Equal(rc.EmbeddedClusterHomeDirectory(), opts.DataDir)
				req.Equal(rc.EmbeddedClusterK0sSubDir(), opts.K0sDataDir)
				req.Equal(rc.EmbeddedClusterOpenEBSLocalSubDir(), opts.OpenEBSDataDir)
				req.Equal("10.96.0.0/12", opts.ServiceCIDR)
				req.Equal("/etc/ssl/certs/ca-bundle.crt", opts.HostCABundlePath)
				proxySpec := rc.ProxySpec()
				req.Equal(proxySpec, opts.ProxySpec)
				req.Equal(installCfg.license, opts.License)
				req.Equal(&installCfg.endUserConfig.Spec, opts.EndUserConfigSpec)
				expectedEmbeddedCfg := release.GetEmbeddedClusterConfig()
				req.NotNil(expectedEmbeddedCfg)
				req.Equal(&expectedEmbeddedCfg.Spec, opts.EmbeddedConfigSpec)
				req.NotNil(opts.KotsInstaller)
			},
		},
		{
			name: "minimal configuration",
			flags: installFlags{
				adminConsolePassword: "pass",
				airgapBundle:         "",
				hostname:             "",
			},
			installCfg: &installConfig{
				clusterID: "cluster-456",
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						IsDisasterRecoverySupported:       false,
						IsEmbeddedClusterMultiNodeEnabled: false,
					},
				},
				tlsCertBytes: []byte("cert-data"),
				tlsKeyBytes:  []byte("key-data"),
			},
			rc: func(t *testing.T) runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(nil)
				tmpDir := t.TempDir()
				rc.SetDataDir(tmpDir)
				rc.SetAdminConsolePort(30000)
				rc.SetHostCABundlePath("/etc/ssl/certs/ca-bundle.crt")
				rc.SetNetworkSpec(ecv1beta1.NetworkSpec{
					ServiceCIDR: "10.96.0.0/12",
				})
				return rc
			}(t),
			kotsadmNamespace: "kotsadm",
			loading: func() **spinner.MessageWriter {
				loading := &spinner.MessageWriter{}
				return &loading
			}(),
			validate: func(t *testing.T, opts *addons.InstallOptions, rc runtimeconfig.RuntimeConfig, installCfg *installConfig) {
				req := require.New(t)
				req.Equal("cluster-456", opts.ClusterID)
				req.Equal("pass", opts.AdminConsolePwd)
				req.Equal(30000, opts.AdminConsolePort)
				req.Equal(false, opts.IsAirgap)
				req.Equal("", opts.Hostname)
				req.Equal([]byte("cert-data"), opts.TLSCertBytes)
				req.Equal([]byte("key-data"), opts.TLSKeyBytes)
				req.Equal(false, opts.DisasterRecoveryEnabled)
				req.Equal(false, opts.IsMultiNodeEnabled)
				req.Equal("kotsadm", opts.KotsadmNamespace)
				req.Equal(rc.EmbeddedClusterHomeDirectory(), opts.DataDir)
				req.Equal(rc.EmbeddedClusterK0sSubDir(), opts.K0sDataDir)
				req.Equal(rc.EmbeddedClusterOpenEBSLocalSubDir(), opts.OpenEBSDataDir)
				req.Equal("10.96.0.0/12", opts.ServiceCIDR)
				req.Equal("/etc/ssl/certs/ca-bundle.crt", opts.HostCABundlePath)
				req.Nil(opts.ProxySpec)
				req.Equal(installCfg.license, opts.License)
				req.Nil(opts.EndUserConfigSpec)
				expectedEmbeddedCfg := release.GetEmbeddedClusterConfig()
				req.NotNil(expectedEmbeddedCfg)
				req.Equal(&expectedEmbeddedCfg.Spec, opts.EmbeddedConfigSpec)
				req.NotNil(opts.KotsInstaller)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildAddonInstallOpts(tt.flags, tt.installCfg, tt.rc, tt.kotsadmNamespace, tt.loading)

			if tt.validate != nil {
				tt.validate(t, opts, tt.rc, tt.installCfg)
			}
		})
	}
}

func Test_buildK0sConfig(t *testing.T) {
	tests := []struct {
		name       string
		flags      *installFlags
		installCfg *installConfig
		wantErr    bool
		validate   func(*testing.T, *k0sv1beta1.ClusterConfig)
	}{
		{
			name: "pod and service CIDRs set",
			flags: &installFlags{
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "10.0.0.0/24",
					ServiceCIDR: "10.1.0.0/24",
					GlobalCIDR:  nil,
				},
			},
			installCfg: &installConfig{},
			wantErr:    false,
			validate: func(t *testing.T, cfg *k0sv1beta1.ClusterConfig) {
				req := require.New(t)
				req.Equal("10.0.0.0/24", cfg.Spec.Network.PodCIDR)
				req.Equal("10.1.0.0/24", cfg.Spec.Network.ServiceCIDR)
			},
		},
		{
			name: "custom pod and service CIDRs",
			flags: &installFlags{
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "192.168.0.0/16",
					ServiceCIDR: "10.96.0.0/12",
					GlobalCIDR:  nil,
				},
			},
			installCfg: &installConfig{},
			wantErr:    false,
			validate: func(t *testing.T, cfg *k0sv1beta1.ClusterConfig) {
				req := require.New(t)
				req.Equal("192.168.0.0/16", cfg.Spec.Network.PodCIDR)
				req.Equal("10.96.0.0/12", cfg.Spec.Network.ServiceCIDR)
			},
		},
		{
			name: "global CIDR should not affect k0s config",
			flags: &installFlags{
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "10.0.0.0/25",
					ServiceCIDR: "10.0.0.128/25",
					GlobalCIDR:  stringPtr("10.0.0.0/24"),
				},
			},
			installCfg: &installConfig{},
			wantErr:    false,
			validate: func(t *testing.T, cfg *k0sv1beta1.ClusterConfig) {
				req := require.New(t)
				req.Equal("10.0.0.0/25", cfg.Spec.Network.PodCIDR)
				req.Equal("10.0.0.128/25", cfg.Spec.Network.ServiceCIDR)
			},
		},
		{
			name: "IPv4 CIDRs with different masks",
			flags: &installFlags{
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "172.16.0.0/20",
					ServiceCIDR: "172.17.0.0/20",
					GlobalCIDR:  nil,
				},
			},
			installCfg: &installConfig{},
			wantErr:    false,
			validate: func(t *testing.T, cfg *k0sv1beta1.ClusterConfig) {
				req := require.New(t)
				req.Equal("172.16.0.0/20", cfg.Spec.Network.PodCIDR)
				req.Equal("172.17.0.0/20", cfg.Spec.Network.ServiceCIDR)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			cfg, err := buildK0sConfig(tt.flags, tt.installCfg)

			if tt.wantErr {
				req.Error(err)
				return
			}

			req.NoError(err)
			req.NotNil(cfg)
			req.NotNil(cfg.Spec)
			req.NotNil(cfg.Spec.Network)

			if tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func Test_buildRecordInstallationOptions(t *testing.T) {
	// Set up release data with embedded cluster config for testing
	err := release.SetReleaseDataForTests(map[string][]byte{
		"embedded-cluster-config.yaml": []byte(`
apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
metadata:
  name: "testconfig"
spec:
  version: "1.0.0"
  roles:
    controller:
      name: controller-test
`),
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		release.SetReleaseDataForTests(nil)
	})

	tests := []struct {
		name       string
		installCfg *installConfig
		rc         runtimeconfig.RuntimeConfig
		validate   func(*testing.T, kubeutils.RecordInstallationOptions)
	}{
		{
			name: "airgap with metadata and info",
			installCfg: &installConfig{
				clusterID: "cluster-123",
				isAirgap:  true,
				license: &kotsv1beta1.License{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-license",
					},
				},
				airgapMetadata: &airgap.AirgapMetadata{
					AirgapInfo: &kotsv1beta1.Airgap{
						Spec: kotsv1beta1.AirgapSpec{
							UncompressedSize: 1024 * 1024 * 1024,
						},
					},
				},
			},
			rc: func(t *testing.T) runtimeconfig.RuntimeConfig {
				rc := runtimeconfig.New(nil)
				tmpDir := t.TempDir()
				rc.SetDataDir(tmpDir)
				rc.SetAdminConsolePort(8800)
				rc.SetManagerPort(8801)
				return rc
			}(t),
			validate: func(t *testing.T, opts kubeutils.RecordInstallationOptions) {
				req := require.New(t)
				req.Equal("cluster-123", opts.ClusterID)
				req.True(opts.IsAirgap)
				req.NotNil(opts.License)
				req.NotEmpty(opts.MetricsBaseURL)
				req.NotNil(opts.RuntimeConfig)
				req.NotEmpty(opts.RuntimeConfig.DataDir)
				req.Equal(8800, opts.RuntimeConfig.AdminConsole.Port)
				req.Equal(8801, opts.RuntimeConfig.Manager.Port)
				req.NotNil(opts.ConfigSpec)
				req.Equal("1.0.0", opts.ConfigSpec.Version)
				req.Equal("controller-test", opts.ConfigSpec.Roles.Controller.Name)
				req.Equal(int64(1024*1024*1024), opts.AirgapUncompressedSize)
				req.Nil(opts.EndUserConfig)
			},
		},
		{
			name: "airgap with metadata but no info",
			installCfg: &installConfig{
				clusterID: "cluster-456",
				isAirgap:  true,
				license: &kotsv1beta1.License{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-license",
					},
				},
				airgapMetadata: &airgap.AirgapMetadata{
					AirgapInfo: nil,
				},
			},
			rc: runtimeconfig.New(nil),
			validate: func(t *testing.T, opts kubeutils.RecordInstallationOptions) {
				req := require.New(t)
				req.Equal(int64(0), opts.AirgapUncompressedSize)
			},
		},
		{
			name: "non-airgap with end user config",
			installCfg: &installConfig{
				clusterID: "cluster-789",
				isAirgap:  false,
				license: &kotsv1beta1.License{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-license",
					},
				},
				endUserConfig: &ecv1beta1.Config{
					Spec: ecv1beta1.ConfigSpec{},
				},
			},
			rc: runtimeconfig.New(nil),
			validate: func(t *testing.T, opts kubeutils.RecordInstallationOptions) {
				req := require.New(t)
				req.False(opts.IsAirgap)
				req.NotNil(opts.EndUserConfig)
			},
		},
		{
			name: "minimal installation",
			installCfg: &installConfig{
				clusterID: "cluster-abc",
				isAirgap:  false,
				license: &kotsv1beta1.License{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-license",
					},
				},
			},
			rc: runtimeconfig.New(nil),
			validate: func(t *testing.T, opts kubeutils.RecordInstallationOptions) {
				req := require.New(t)
				req.Equal("cluster-abc", opts.ClusterID)
				req.False(opts.IsAirgap)
				req.Nil(opts.EndUserConfig)
				req.Equal(int64(0), opts.AirgapUncompressedSize)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := buildRecordInstallationOptions(tt.installCfg, tt.rc)

			if tt.validate != nil {
				tt.validate(t, opts)
			}
		})
	}
}

func Test_validateAdminConsolePassword(t *testing.T) {
	tests := []struct {
		name          string
		password      string
		passwordCheck string
		wantSuccess   bool
	}{
		{
			name:          "passwords match, with 3 characters length",
			password:      "123",
			passwordCheck: "123",
			wantSuccess:   false,
		},
		{
			name:          "passwords don't match, with 3 characters length",
			password:      "123",
			passwordCheck: "nop",
			wantSuccess:   false,
		},
		{
			name:          "passwords don't match, with 6 characters length",
			password:      "123456",
			passwordCheck: "nmatch",
			wantSuccess:   false,
		},
		{
			name:          "passwords match, with 6 characters length",
			password:      "123456",
			passwordCheck: "123456",
			wantSuccess:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			success := validateAdminConsolePassword(tt.password, tt.passwordCheck)
			if tt.wantSuccess {
				req.True(success)
			} else {
				req.False(success)
			}
		})
	}
}

func Test_ensureAdminConsolePassword(t *testing.T) {

	tests := []struct {
		name         string
		userPassword string
		noPrompt     bool
		wantPassword string
		wantError    bool
	}{
		{
			name:         "no user provided password, no-prompt true",
			userPassword: "",
			noPrompt:     true,
			wantPassword: "password",
			wantError:    false,
		},
		{
			name:         "invalid user provided password, no-prompt false",
			userPassword: "123",
			noPrompt:     false,
			wantPassword: "",
			wantError:    true,
		},
		{
			name:         "user provided password, no-prompt true",
			userPassword: "123456",
			noPrompt:     true,
			wantPassword: "123456",
			wantError:    false,
		},
		{
			name:         "user provided password, no-prompt false",
			userPassword: "123456",
			noPrompt:     false,
			wantPassword: "123456",
			wantError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			flags := &installFlags{
				assumeYes:            tt.noPrompt,
				adminConsolePassword: tt.userPassword,
			}

			err := ensureAdminConsolePassword(flags)
			if tt.wantError {
				req.Error(err)
			} else {
				req.NoError(err)
				req.Equal(tt.wantPassword, flags.adminConsolePassword)
			}
		})
	}
}

func Test_maybePromptForAppUpdate(t *testing.T) {
	tests := []struct {
		name                  string
		channelRelease        *release.ChannelRelease
		apiHandler            func(http.ResponseWriter, *http.Request)
		assumeYes             bool
		answerYes             bool
		wantPrompt            bool
		wantErr               bool
		isErrNothingElseToAdd bool
	}{
		{
			name:           "no channel release",
			channelRelease: nil,
			wantPrompt:     false,
			wantErr:        false,
		},
		{
			name: "no license",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			wantPrompt: false,
			wantErr:    true, // will fail during the test because license is required
		},
		{
			name: "version matches current release",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := `{"channelReleases":[{"channelId":"test-channel","versionLabel":"v1.0.0"}]}`
				w.Write([]byte(response))
			},
			wantPrompt: false,
			wantErr:    false,
		},
		{
			name: "newer version available, assumeYes true",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := `{"channelReleases":[{"channelId":"test-channel","versionLabel":"v2.0.0"}]}`
				w.Write([]byte(response))
			},
			assumeYes:  true,
			wantPrompt: false,
			wantErr:    false,
		},
		{
			name: "newer version available, user confirms",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := `{"channelReleases":[{"channelId":"test-channel","versionLabel":"v2.0.0"}]}`
				w.Write([]byte(response))
			},
			answerYes:  true,
			wantPrompt: true,
			wantErr:    false,
		},
		{
			name: "newer version available, user declines",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := `{"channelReleases":[{"channelId":"test-channel","versionLabel":"v2.0.0"}]}`
				w.Write([]byte(response))
			},
			answerYes:             false,
			wantPrompt:            true,
			wantErr:               true,
			isErrNothingElseToAdd: true,
		},
		{
			name: "API returns 404",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantPrompt: false,
			wantErr:    true,
		},
		{
			name: "API returns empty releases",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := `{"channelReleases":[]}`
				w.Write([]byte(response))
			},
			wantPrompt: false,
			wantErr:    true,
		},
		{
			name: "API returns invalid JSON",
			channelRelease: &release.ChannelRelease{
				ChannelID:    "test-channel",
				ChannelSlug:  "test",
				AppSlug:      "app-slug",
				VersionLabel: "v1.0.0",
			},
			apiHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`invalid json`))
			},
			wantPrompt: false,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var license *kotsv1beta1.License
			releaseDataMap := map[string][]byte{}
			if tt.channelRelease != nil {
				handler := getReleasesHandler(t, tt.channelRelease.ChannelID, tt.apiHandler)
				ts := httptest.NewServer(handler)
				t.Cleanup(ts.Close)

				license = &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						LicenseID: "license-id",
						AppSlug:   "app-slug",
					},
				}

				embedStr := "# channel release object\nchannelID: %s\nchannelSlug: %s\nappSlug: %s\nversionLabel: %s\ndefaultDomains:\n  replicatedAppDomain: %s"
				releaseDataMap["release.yaml"] = []byte(fmt.Sprintf(
					embedStr,
					tt.channelRelease.ChannelID,
					tt.channelRelease.ChannelSlug,
					tt.channelRelease.AppSlug,
					tt.channelRelease.VersionLabel,
					ts.URL,
				))
			}

			err := release.SetReleaseDataForTests(releaseDataMap)
			require.NoError(t, err)

			t.Cleanup(func() {
				release.SetReleaseDataForTests(nil)
			})

			var in *bytes.Buffer
			if tt.answerYes {
				in = bytes.NewBuffer([]byte("y\n"))
			} else {
				in = bytes.NewBuffer([]byte("n\n"))
			}
			out := bytes.NewBuffer([]byte{})
			prompt := plain.New(plain.WithIn(in), plain.WithOut(out))

			prompts.SetTerminal(true)
			t.Cleanup(func() { prompts.SetTerminal(false) })

			// Wrap the license for the new API
			wrappedLicense := &licensewrapper.LicenseWrapper{V1: license}
			err = maybePromptForAppUpdate(context.Background(), prompt, wrappedLicense, tt.assumeYes)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tt.wantPrompt {
				assert.Contains(t, out.String(), "Do you want to continue installing", "Prompt should have been printed")
			} else {
				assert.Empty(t, out.String(), "Prompt should not have been printed")
			}

			if tt.isErrNothingElseToAdd {
				assert.ErrorAs(t, err, &ErrorNothingElseToAdd{})
			} else {
				assert.NotErrorAs(t, err, &ErrorNothingElseToAdd{})
			}
		})
	}
}

func getReleasesHandler(t *testing.T, channelID string, apiHandler http.HandlerFunc) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/release/app-slug/pending" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if r.URL.Query().Get("selectedChannelId") != channelID {
			t.Fatalf("unexpected selectedChannelId %s", r.URL.Query().Get("selectedChannelId"))
		}
		if r.URL.Query().Get("channelSequence") != "" {
			t.Fatalf("unexpected channelSequence %s", r.URL.Query().Get("channelSequence"))
		}
		if r.URL.Query().Get("isSemverSupported") != "true" {
			t.Fatalf("unexpected isSemverSupported %s", r.URL.Query().Get("isSemverSupported"))
		}

		apiHandler(w, r)
	}
}

func Test_verifyLicensePresence(t *testing.T) {
	testRelease := &release.ChannelRelease{
		AppSlug:     "embedded-cluster-smoke-test-staging-app",
		ChannelID:   "2cHXb1RCttzpR0xvnNWyaZCgDBP",
		ChannelSlug: "CI",
	}

	tests := []struct {
		name    string
		license *licensewrapper.LicenseWrapper
		release *release.ChannelRelease
		wantErr string
	}{
		{
			name:    "no license, no release",
			wantErr: "",
		},
		{
			name:    "no license, with release",
			release: testRelease,
			wantErr: `no license was provided for embedded-cluster-smoke-test-staging-app and one is required, please rerun with '--license <path to license file>'`,
		},
		{
			name: "valid license, no release",
			license: &licensewrapper.LicenseWrapper{
				V1: &kotsv1beta1.License{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "kots.io/v1beta1",
						Kind:       "License",
					},
					Spec: kotsv1beta1.LicenseSpec{
						LicenseID:                        "test-license-no-release",
						AppSlug:                          "embedded-cluster-smoke-test-staging-app",
						ChannelID:                        "2cHXb1RCttzpR0xvnNWyaZCgDBP",
						IsEmbeddedClusterDownloadEnabled: true,
					},
				},
			},
			wantErr: "a license was provided but no release was found in binary, please rerun without the license flag",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			err := verifyLicensePresence(tt.license, tt.release)
			if tt.wantErr != "" {
				req.EqualError(err, tt.wantErr)
			} else {
				req.NoError(err)
			}
		})
	}
}

func Test_verifyLicenseFields(t *testing.T) {
	testRelease := &release.ChannelRelease{
		AppSlug:     "embedded-cluster-smoke-test-staging-app",
		ChannelID:   "2cHXb1RCttzpR0xvnNWyaZCgDBP",
		ChannelSlug: "CI",
	}

	tests := []struct {
		name    string
		license *licensewrapper.LicenseWrapper
		release *release.ChannelRelease
		wantErr string
	}{
		{
			name:    "valid license (v2), with release",
			release: testRelease,
			license: &licensewrapper.LicenseWrapper{
				V2: &kotsv1beta2.License{
					Spec: kotsv1beta2.LicenseSpec{
						AppSlug:                          "embedded-cluster-smoke-test-staging-app",
						ChannelID:                        "2cHXb1RCttzpR0xvnNWyaZCgDBP",
						IsEmbeddedClusterDownloadEnabled: true,
					},
				},
			},
		},
		{
			name:    "valid license (v1), with release",
			release: testRelease,
			license: &licensewrapper.LicenseWrapper{
				V1: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						AppSlug:                          "embedded-cluster-smoke-test-staging-app",
						ChannelID:                        "2cHXb1RCttzpR0xvnNWyaZCgDBP",
						IsEmbeddedClusterDownloadEnabled: true,
					},
				},
			},
		},
		{
			name:    "valid multi-channel license, with release",
			release: testRelease,
			license: &licensewrapper.LicenseWrapper{
				V1: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						AppSlug:                          "embedded-cluster-smoke-test-staging-app",
						ChannelID:                        "OtherChannelID",
						IsEmbeddedClusterDownloadEnabled: true,
						Channels: []kotsv1beta1.Channel{
							{
								ChannelID:   "OtherChannelID",
								ChannelName: "OtherChannel",
								ChannelSlug: "other-channel",
								IsDefault:   true,
							},
							{
								ChannelID:   "2cHXb1RCttzpR0xvnNWyaZCgDBP",
								ChannelName: "ExpectedChannel",
								ChannelSlug: "expected-channel",
								IsDefault:   false,
							},
						},
					},
				},
			},
		},
		{
			name:    "expired license, with release",
			release: testRelease,
			license: &licensewrapper.LicenseWrapper{
				V1: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						AppSlug:                          "embedded-cluster-smoke-test-staging-app",
						ChannelID:                        "2cHXb1RCttzpR0xvnNWyaZCgDBP",
						IsEmbeddedClusterDownloadEnabled: true,
						Entitlements: map[string]kotsv1beta1.EntitlementField{
							"expires_at": {
								Value: kotsv1beta1.EntitlementValue{
									Type:   kotsv1beta1.String,
									StrVal: "2024-06-03T00:00:00Z",
								},
							},
						},
					},
				},
			},
			wantErr: "license expired on 2024-06-03 00:00:00 +0000 UTC, please provide a valid license",
		},
		{
			name:    "license with no expiration, with release",
			release: testRelease,
			license: &licensewrapper.LicenseWrapper{
				V1: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						AppSlug:                          "embedded-cluster-smoke-test-staging-app",
						ChannelID:                        "2cHXb1RCttzpR0xvnNWyaZCgDBP",
						IsEmbeddedClusterDownloadEnabled: true,
						Entitlements: map[string]kotsv1beta1.EntitlementField{
							"expires_at": {
								Value: kotsv1beta1.EntitlementValue{
									Type:   kotsv1beta1.String,
									StrVal: "",
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "license with 100 year expiration, with release",
			release: testRelease,
			license: &licensewrapper.LicenseWrapper{
				V1: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						AppSlug:                          "embedded-cluster-smoke-test-staging-app",
						ChannelID:                        "2cHXb1RCttzpR0xvnNWyaZCgDBP",
						IsEmbeddedClusterDownloadEnabled: true,
						Entitlements: map[string]kotsv1beta1.EntitlementField{
							"expires_at": {
								Value: kotsv1beta1.EntitlementValue{
									Type:   kotsv1beta1.String,
									StrVal: "2124-06-03T00:00:00Z",
								},
							},
						},
					},
				},
			},
		},
		{
			name:    "embedded cluster not enabled, with release",
			release: testRelease,
			license: &licensewrapper.LicenseWrapper{
				V1: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						AppSlug:                          "embedded-cluster-smoke-test-staging-app",
						ChannelID:                        "2cHXb1RCttzpR0xvnNWyaZCgDBP",
						IsEmbeddedClusterDownloadEnabled: false,
					},
				},
			},
			wantErr: "license does not have embedded cluster enabled, please provide a valid license",
		},
		{
			name:    "incorrect license (multichan license)",
			release: testRelease,
			license: &licensewrapper.LicenseWrapper{
				V1: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						AppSlug:                          "embedded-cluster-smoke-test-staging-app",
						ChannelID:                        "2i9fCbxTNIhuAOaC6MoKMVeGzuK",
						IsEmbeddedClusterDownloadEnabled: false,
						Channels: []kotsv1beta1.Channel{
							{
								ChannelID:   "2i9fCbxTNIhuAOaC6MoKMVeGzuK",
								ChannelName: "Stable",
								ChannelSlug: "stable",
								IsDefault:   true,
							},
							{
								ChannelID:   "4l9fCbxTNIhuAOaC6MoKMVeV3K",
								ChannelName: "Alternate",
								ChannelSlug: "alternate",
								IsDefault:   false,
							},
						},
					},
				},
			},
			wantErr: "binary channel 2cHXb1RCttzpR0xvnNWyaZCgDBP (CI) not present in license, channels allowed by license are: stable (2i9fCbxTNIhuAOaC6MoKMVeGzuK), alternate (4l9fCbxTNIhuAOaC6MoKMVeV3K)",
		},
		{
			name:    "incorrect license (pre-multichan license)",
			release: testRelease,
			license: &licensewrapper.LicenseWrapper{
				V1: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						AppSlug:                          "embedded-cluster-smoke-test-staging-app",
						ChannelID:                        "2i9fCbxTNIhuAOaC6MoKMVeGzuK",
						ChannelName:                      "Stable",
						IsEmbeddedClusterDownloadEnabled: false,
					},
				},
			},
			wantErr: "binary channel 2cHXb1RCttzpR0xvnNWyaZCgDBP (CI) not present in license, channels allowed by license are: Stable (2i9fCbxTNIhuAOaC6MoKMVeGzuK)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			err := verifyLicenseFields(tt.license, tt.release)
			if tt.wantErr != "" {
				req.EqualError(err, tt.wantErr)
			} else {
				req.NoError(err)
			}
		})
	}
}

func Test_verifyProxyConfig(t *testing.T) {
	tests := []struct {
		name                  string
		proxy                 *ecv1beta1.ProxySpec
		confirm               bool
		assumeYes             bool
		wantErr               bool
		isErrNothingElseToAdd bool
	}{
		{
			name:    "no proxy set",
			proxy:   nil,
			wantErr: false,
		},
		{
			name: "http proxy set without https proxy and user confirms",
			proxy: &ecv1beta1.ProxySpec{
				HTTPProxy: "http://proxy:8080",
			},
			confirm: true,
			wantErr: false,
		},
		{
			name: "http proxy set without https proxy and user declines",
			proxy: &ecv1beta1.ProxySpec{
				HTTPProxy: "http://proxy:8080",
			},
			confirm:               false,
			wantErr:               true,
			isErrNothingElseToAdd: true,
		},
		{
			name: "http proxy set without https proxy and assumeYes is true",
			proxy: &ecv1beta1.ProxySpec{
				HTTPProxy: "http://proxy:8080",
			},
			assumeYes: true,
			wantErr:   false,
		},
		{
			name: "both proxies set",
			proxy: &ecv1beta1.ProxySpec{
				HTTPProxy:  "http://proxy:8080",
				HTTPSProxy: "https://proxy:8080",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var in *bytes.Buffer
			if tt.confirm {
				in = bytes.NewBuffer([]byte("y\n"))
			} else {
				in = bytes.NewBuffer([]byte("n\n"))
			}
			out := bytes.NewBuffer([]byte{})
			mockPrompt := plain.New(plain.WithIn(in), plain.WithOut(out))

			prompts.SetTerminal(true)
			t.Cleanup(func() { prompts.SetTerminal(false) })

			err := verifyProxyConfig(tt.proxy, mockPrompt, tt.assumeYes)
			if tt.wantErr {
				require.Error(t, err)
				if tt.isErrNothingElseToAdd {
					assert.ErrorAs(t, err, &ErrorNothingElseToAdd{})
				}
				if tt.proxy != nil && tt.proxy.HTTPProxy != "" && tt.proxy.HTTPSProxy == "" && !tt.assumeYes {
					assert.Contains(t, out.String(), "Typically --https-proxy should be set if --http-proxy is set")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_ignoreAppPreflights_FlagVisibility(t *testing.T) {
	tests := []struct {
		name                        string
		enableV3EnvVar              string
		expectedFlagShouldBeVisible bool
	}{
		{
			name:                        "ENABLE_V3 not set - flag should be visible",
			enableV3EnvVar:              "",
			expectedFlagShouldBeVisible: true,
		},
		{
			name:                        "ENABLE_V3 set to 1 - flag should be hidden",
			enableV3EnvVar:              "1",
			expectedFlagShouldBeVisible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment
			os.Unsetenv("ENABLE_V3")

			// Set environment variable if specified
			if tt.enableV3EnvVar != "" {
				t.Setenv("ENABLE_V3", tt.enableV3EnvVar)
			}

			flags := &installFlags{}
			enableV3 := isV3Enabled()
			flagSet := newLinuxInstallFlags(flags, enableV3)

			// Check if the flag exists
			flag := flagSet.Lookup("ignore-app-preflights")
			flagExists := flag != nil

			assert.Equal(t, tt.expectedFlagShouldBeVisible, flagExists, "Flag visibility should match expected")

			if flagExists {
				// Test flag properties
				assert.Equal(t, "ignore-app-preflights", flag.Name)
				assert.Equal(t, "false", flag.DefValue) // Default should be false
				assert.Equal(t, "Allow bypassing app preflight failures", flag.Usage)
				assert.Equal(t, "bool", flag.Value.Type())

				// Test flag targets - should be Linux only
				targetAnnotation := flag.Annotations[flagAnnotationTarget]
				require.NotNil(t, targetAnnotation, "Flag should have target annotation")
				assert.Contains(t, targetAnnotation, flagAnnotationTargetValueLinux)
			}
		})
	}
}

func Test_ignoreAppPreflights_FlagParsing(t *testing.T) {
	tests := []struct {
		name                     string
		args                     []string
		enableV3                 bool
		expectedIgnorePreflights bool
		expectError              bool
	}{
		{
			name:                     "flag not provided, V3 disabled",
			args:                     []string{},
			enableV3:                 false,
			expectedIgnorePreflights: false,
			expectError:              false,
		},
		{
			name:                     "flag set to true, V3 disabled",
			args:                     []string{"--ignore-app-preflights"},
			enableV3:                 false,
			expectedIgnorePreflights: true,
			expectError:              false,
		},
		{
			name:                     "flag set but V3 enabled - should error",
			args:                     []string{"--ignore-app-preflights"},
			enableV3:                 true,
			expectedIgnorePreflights: false,
			expectError:              true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable for V3 testing
			if tt.enableV3 {
				t.Setenv("ENABLE_V3", "1")
			}

			// Create a flagset similar to how newLinuxInstallFlags works
			flags := &installFlags{}
			flagSet := newLinuxInstallFlags(flags, tt.enableV3)

			// Create a command to test flag parsing
			cmd := &cobra.Command{
				Use: "test",
				Run: func(cmd *cobra.Command, args []string) {},
			}
			cmd.Flags().AddFlagSet(flagSet)

			// Try to parse the arguments
			err := cmd.Flags().Parse(tt.args)
			if tt.expectError {
				assert.Error(t, err, "Flag parsing should fail when flag doesn't exist")
			} else {
				assert.NoError(t, err, "Flag parsing should succeed")
				// Check the flag value only if parsing succeeded
				assert.Equal(t, tt.expectedIgnorePreflights, flags.ignoreAppPreflights)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
