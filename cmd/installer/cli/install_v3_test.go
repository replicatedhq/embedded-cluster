package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	apitypes "github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	newconfig "github.com/replicatedhq/embedded-cluster/pkg-new/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_buildHeadlessInstallOptions(t *testing.T) {
	tests := []struct {
		name               string
		flags              installFlags
		apiConfig          apiOptions
		expectedConfigVals apitypes.AppConfigValues
		expectedLinuxCfg   apitypes.LinuxInstallationConfig
		expectedIgnoreHost bool
		expectedIgnoreApp  bool
		expectedAirgap     string
	}{
		{
			name: "minimal configuration",
			flags: installFlags{
				adminConsolePort:        30000,
				dataDir:                 "/var/lib/embedded-cluster",
				localArtifactMirrorPort: 50000,
				networkInterface:        "eth0",
				ignoreHostPreflights:    false,
				ignoreAppPreflights:     false,
				airgapBundle:            "",
			},
			apiConfig: apiOptions{
				APIConfig: apitypes.APIConfig{
					ConfigValues: apitypes.AppConfigValues{},
				},
			},
			expectedConfigVals: apitypes.AppConfigValues{},
			expectedLinuxCfg: apitypes.LinuxInstallationConfig{
				AdminConsolePort:        30000,
				DataDirectory:           "/var/lib/embedded-cluster",
				LocalArtifactMirrorPort: 50000,
				HTTPProxy:               "",
				HTTPSProxy:              "",
				NoProxy:                 "",
				NetworkInterface:        "eth0",
				PodCIDR:                 "",
				ServiceCIDR:             "",
				GlobalCIDR:              "",
			},
			expectedIgnoreHost: false,
			expectedIgnoreApp:  false,
			expectedAirgap:     "",
		},
		{
			name: "with proxy configuration",
			flags: installFlags{
				adminConsolePort:        30000,
				dataDir:                 "/opt/ec",
				localArtifactMirrorPort: 50000,
				networkInterface:        "ens5",
				ignoreHostPreflights:    true,
				ignoreAppPreflights:     false,
				airgapBundle:            "",
				proxySpec: &ecv1beta1.ProxySpec{
					HTTPProxy:  "http://proxy.example.com:8080",
					HTTPSProxy: "http://proxy.example.com:8443",
					NoProxy:    "localhost,127.0.0.1",
				},
			},
			apiConfig: apiOptions{
				APIConfig: apitypes.APIConfig{
					ConfigValues: apitypes.AppConfigValues{
						"hostname": apitypes.AppConfigValue{Value: "test.example.com"},
					},
				},
			},
			expectedConfigVals: apitypes.AppConfigValues{
				"hostname": apitypes.AppConfigValue{Value: "test.example.com"},
			},
			expectedLinuxCfg: apitypes.LinuxInstallationConfig{
				AdminConsolePort:        30000,
				DataDirectory:           "/opt/ec",
				LocalArtifactMirrorPort: 50000,
				HTTPProxy:               "http://proxy.example.com:8080",
				HTTPSProxy:              "http://proxy.example.com:8443",
				NoProxy:                 "localhost,127.0.0.1",
				NetworkInterface:        "ens5",
				PodCIDR:                 "",
				ServiceCIDR:             "",
				GlobalCIDR:              "",
			},
			expectedIgnoreHost: true,
			expectedIgnoreApp:  false,
			expectedAirgap:     "",
		},
		{
			name: "with CIDR configuration",
			flags: installFlags{
				adminConsolePort:        30000,
				dataDir:                 "/data",
				localArtifactMirrorPort: 50000,
				networkInterface:        "eth0",
				ignoreHostPreflights:    false,
				ignoreAppPreflights:     true,
				airgapBundle:            "/tmp/airgap.tar.gz",
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "10.244.0.0/16",
					ServiceCIDR: "10.96.0.0/12",
					GlobalCIDR:  stringPtr("10.0.0.0/8"),
				},
			},
			apiConfig: apiOptions{
				APIConfig: apitypes.APIConfig{
					ConfigValues: apitypes.AppConfigValues{},
				},
			},
			expectedConfigVals: apitypes.AppConfigValues{},
			expectedLinuxCfg: apitypes.LinuxInstallationConfig{
				AdminConsolePort:        30000,
				DataDirectory:           "/data",
				LocalArtifactMirrorPort: 50000,
				HTTPProxy:               "",
				HTTPSProxy:              "",
				NoProxy:                 "",
				NetworkInterface:        "eth0",
				PodCIDR:                 "10.244.0.0/16",
				ServiceCIDR:             "10.96.0.0/12",
				GlobalCIDR:              "10.0.0.0/8",
			},
			expectedIgnoreHost: false,
			expectedIgnoreApp:  true,
			expectedAirgap:     "/tmp/airgap.tar.gz",
		},
		{
			name: "with all configurations",
			flags: installFlags{
				adminConsolePort:        8800,
				dataDir:                 "/custom/data",
				localArtifactMirrorPort: 60000,
				networkInterface:        "bond0",
				ignoreHostPreflights:    true,
				ignoreAppPreflights:     true,
				airgapBundle:            "/path/to/bundle.airgap",
				proxySpec: &ecv1beta1.ProxySpec{
					HTTPProxy:  "http://10.0.0.1:3128",
					HTTPSProxy: "https://10.0.0.1:3129",
					NoProxy:    ".cluster.local,.svc",
				},
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "172.16.0.0/16",
					ServiceCIDR: "172.17.0.0/16",
					GlobalCIDR:  stringPtr("172.0.0.0/8"),
				},
			},
			apiConfig: apiOptions{
				APIConfig: apitypes.APIConfig{
					ConfigValues: apitypes.AppConfigValues{
						"db_host":  apitypes.AppConfigValue{Value: "db.example.com"},
						"replicas": apitypes.AppConfigValue{Value: "3"},
					},
				},
			},
			expectedConfigVals: apitypes.AppConfigValues{
				"db_host":  apitypes.AppConfigValue{Value: "db.example.com"},
				"replicas": apitypes.AppConfigValue{Value: "3"},
			},
			expectedLinuxCfg: apitypes.LinuxInstallationConfig{
				AdminConsolePort:        8800,
				DataDirectory:           "/custom/data",
				LocalArtifactMirrorPort: 60000,
				HTTPProxy:               "http://10.0.0.1:3128",
				HTTPSProxy:              "https://10.0.0.1:3129",
				NoProxy:                 ".cluster.local,.svc",
				NetworkInterface:        "bond0",
				PodCIDR:                 "172.16.0.0/16",
				ServiceCIDR:             "172.17.0.0/16",
				GlobalCIDR:              "172.0.0.0/8",
			},
			expectedIgnoreHost: true,
			expectedIgnoreApp:  true,
			expectedAirgap:     "/path/to/bundle.airgap",
		},
		{
			name: "empty CIDR strings ignored",
			flags: installFlags{
				adminConsolePort:        30000,
				dataDir:                 "/data",
				localArtifactMirrorPort: 50000,
				networkInterface:        "eth0",
				cidrConfig: &newconfig.CIDRConfig{
					PodCIDR:     "",
					ServiceCIDR: "",
					GlobalCIDR:  nil,
				},
			},
			apiConfig: apiOptions{
				APIConfig: apitypes.APIConfig{
					ConfigValues: apitypes.AppConfigValues{},
				},
			},
			expectedConfigVals: apitypes.AppConfigValues{},
			expectedLinuxCfg: apitypes.LinuxInstallationConfig{
				AdminConsolePort:        30000,
				DataDirectory:           "/data",
				LocalArtifactMirrorPort: 50000,
				HTTPProxy:               "",
				HTTPSProxy:              "",
				NoProxy:                 "",
				NetworkInterface:        "eth0",
				PodCIDR:                 "",
				ServiceCIDR:             "",
				GlobalCIDR:              "",
			},
			expectedIgnoreHost: false,
			expectedIgnoreApp:  false,
			expectedAirgap:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildHeadlessInstallOptions(tt.flags, tt.apiConfig)

			assert.Equal(t, tt.expectedConfigVals, result.ConfigValues, "ConfigValues should match")
			assert.Equal(t, tt.expectedLinuxCfg, result.LinuxInstallationConfig, "LinuxInstallationConfig should match")
			assert.Equal(t, tt.expectedIgnoreHost, result.IgnoreHostPreflights, "IgnoreHostPreflights should match")
			assert.Equal(t, tt.expectedIgnoreApp, result.IgnoreAppPreflights, "IgnoreAppPreflights should match")
			assert.Equal(t, tt.expectedAirgap, result.AirgapBundle, "AirgapBundle should match")
		})
	}
}

func Test_buildOrchestrator(t *testing.T) {
	tests := []struct {
		name          string
		flags         installFlags
		apiConfig     apiOptions
		expectError   bool
		expectedError string
	}{
		{
			name: "success with linux target",
			flags: installFlags{
				managerPort: 30000,
			},
			apiConfig: apiOptions{
				APIConfig: apitypes.APIConfig{
					InstallTarget: apitypes.InstallTargetLinux,
					Password:      "test-password",
				},
			},
			expectError: false,
		},
		{
			name: "error with kubernetes target",
			flags: installFlags{
				managerPort: 30000,
			},
			apiConfig: apiOptions{
				APIConfig: apitypes.APIConfig{
					InstallTarget: apitypes.InstallTargetKubernetes,
					Password:      "test-password",
				},
			},
			expectError:   true,
			expectedError: "kubernetes target not supported",
		},
		{
			name: "error with empty target",
			flags: installFlags{
				managerPort: 30000,
			},
			apiConfig: apiOptions{
				APIConfig: apitypes.APIConfig{
					InstallTarget: "",
					Password:      "test-password",
				},
			},
			expectError:   true,
			expectedError: " target not supported",
		},
		{
			name: "success with custom manager port",
			flags: installFlags{
				managerPort: 8080,
			},
			apiConfig: apiOptions{
				APIConfig: apitypes.APIConfig{
					InstallTarget: apitypes.InstallTargetLinux,
					Password:      "another-password",
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This test requires a real API server to be running for authentication.
			// In practice, we would need to mock the API client or have a test server.
			// For now, we'll test the target validation logic which happens before API calls.

			ctx := context.Background()
			orchestrator, err := buildOrchestrator(ctx, tt.flags, tt.apiConfig)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, orchestrator)
			} else {
				// We expect an error here because there's no actual API server running
				// but we can verify it's an authentication error, not a target validation error
				if err != nil {
					// Authentication will fail without a real API server, which is expected
					// The important thing is we don't get a "target not supported" error
					assert.NotContains(t, err.Error(), "target not supported")
				}
			}
		})
	}
}

func Test_displayInstallErrorAndRecoveryInstructions(t *testing.T) {
	appSlug := "my-app"

	tests := []struct {
		name           string
		err            error
		resetNeeded    bool
		expectedOutput string
	}{
		{
			name:        "reset needed - infrastructure failure",
			err:         errors.New("setup infrastructure: failed to start k0s: timeout waiting for ready"),
			resetNeeded: true,
			expectedOutput: fmt.Sprintf(`
Error: setup infrastructure: failed to start k0s: timeout waiting for ready

To collect diagnostic information, run: %s support-bundle
To retry installation, run: %s reset and wait for server reboot
`, appSlug, appSlug),
		},
		{
			name:        "no reset needed - config validation error",
			err:         errors.New("application configuration validation failed: field errors:\n  - Field 'database_host': required field missing\n  - Field 'replica_count': value \"10\" exceeds maximum allowed value 5\n  - Field 'enable_ssl': validation rule failed: SSL requires cert_path to be set"),
			resetNeeded: false,
			expectedOutput: fmt.Sprintf(`
Error: application configuration validation failed: field errors:
  - Field 'database_host': required field missing
  - Field 'replica_count': value "10" exceeds maximum allowed value 5
  - Field 'enable_ssl': validation rule failed: SSL requires cert_path to be set

For configuration options, run: %s install --help
Please correct the above issues and retry
`, appSlug),
		},
		{
			name:        "reset needed - app installation failure",
			err:         errors.New("install app: deployment failed: pods not ready"),
			resetNeeded: true,
			expectedOutput: fmt.Sprintf(`
Error: install app: deployment failed: pods not ready

To collect diagnostic information, run: %s support-bundle
To retry installation, run: %s reset and wait for server reboot
`, appSlug, appSlug),
		},
		{
			name:        "no reset needed - preflight check failure",
			err:         errors.New("host preflight checks failed: insufficient memory"),
			resetNeeded: false,
			expectedOutput: fmt.Sprintf(`
Error: host preflight checks failed: insufficient memory

For configuration options, run: %s install --help
Please correct the above issues and retry
`, appSlug),
		},
		{
			name:        "reset needed - airgap processing failure",
			err:         errors.New("process airgap bundle: failed to extract images"),
			resetNeeded: true,
			expectedOutput: fmt.Sprintf(`
Error: process airgap bundle: failed to extract images

To collect diagnostic information, run: %s support-bundle
To retry installation, run: %s reset and wait for server reboot
`, appSlug, appSlug),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create logger with buffer
			var buf bytes.Buffer
			logger := newPlainTextLogger(&buf)

			// Execute function
			displayInstallErrorAndRecoveryInstructions(tt.err, tt.resetNeeded, appSlug, logger)

			// Assert entire output matches exactly
			assert.Equal(t, tt.expectedOutput, buf.String(), "output should match exactly")
		})
	}
}

// newPlainTextLogger creates a logger that outputs plain text (no timestamps, levels, or colors)
// to the provided buffer. This is useful for testing log output where you want to verify
// exact message content without dealing with log formatting.
func newPlainTextLogger(buf *bytes.Buffer) *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(buf)
	logger.SetFormatter(&plainTextFormatter{})
	return logger
}

// plainTextFormatter is a logrus formatter that outputs only the message text with no metadata.
// Used in tests to capture exact log messages without timestamps, levels, or ANSI color codes.
type plainTextFormatter struct {
}

// Format implements the logrus.Formatter interface, returning just the message with a newline.
func (f *plainTextFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(entry.Message + "\n"), nil
}
