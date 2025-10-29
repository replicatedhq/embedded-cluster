package kotscli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_buildInstallArgs(t *testing.T) {
	tests := []struct {
		name             string
		opts             InstallOptions
		upstreamURI      string
		appVersionLabel  string
		licenseFile      string
		shouldContain    []string
		shouldNotContain []string
	}{
		{
			name: "with config values file",
			opts: InstallOptions{
				Namespace:        "kotsadm",
				ConfigValuesFile: "/tmp/config-values.yaml",
			},
			upstreamURI:     "test-app",
			appVersionLabel: "v1.0.0",
			licenseFile:     "/tmp/license",
			shouldContain: []string{
				"install",
				"test-app",
				"--namespace", "kotsadm",
				"--config-values", "/tmp/config-values.yaml",
				"--exclude-admin-console",
			},
			shouldNotContain: []string{},
		},
		{
			name: "without config values file",
			opts: InstallOptions{
				Namespace:        "kotsadm",
				ConfigValuesFile: "",
			},
			upstreamURI:     "test-app",
			appVersionLabel: "v1.0.0",
			licenseFile:     "/tmp/license",
			shouldContain: []string{
				"install",
				"test-app",
				"--namespace", "kotsadm",
			},
			shouldNotContain: []string{
				"--config-values",
			},
		},
		{
			name: "with config values and airgap bundle",
			opts: InstallOptions{
				Namespace:        "kotsadm",
				ConfigValuesFile: "/tmp/config-values.yaml",
				AirgapBundle:     "/tmp/airgap.tar.gz",
			},
			upstreamURI:     "test-app",
			appVersionLabel: "v1.0.0",
			licenseFile:     "/tmp/license",
			shouldContain: []string{
				"install",
				"--config-values", "/tmp/config-values.yaml",
				"--airgap-bundle", "/tmp/airgap.tar.gz",
			},
			shouldNotContain: []string{},
		},
		{
			name: "with all optional flags",
			opts: InstallOptions{
				Namespace:        "kotsadm",
				ConfigValuesFile: "/tmp/config-values.yaml",
				AirgapBundle:     "/tmp/airgap.tar.gz",
				SkipPreflights:   true,
				DisableImagePush: true,
			},
			upstreamURI:     "test-app",
			appVersionLabel: "v1.0.0",
			licenseFile:     "/tmp/license",
			shouldContain: []string{
				"--config-values", "/tmp/config-values.yaml",
				"--airgap-bundle", "/tmp/airgap.tar.gz",
				"--skip-preflights",
				"--disable-image-push",
			},
			shouldNotContain: []string{},
		},
		{
			name: "config values file with special path characters",
			opts: InstallOptions{
				Namespace:        "kotsadm",
				ConfigValuesFile: "/path/with-special_chars.123/config-values.yaml",
			},
			upstreamURI:     "test-app",
			appVersionLabel: "v1.0.0",
			licenseFile:     "/tmp/license",
			shouldContain: []string{
				"--config-values",
				"/path/with-special_chars.123/config-values.yaml",
			},
			shouldNotContain: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			args := buildInstallArgs(tt.opts, tt.upstreamURI, tt.appVersionLabel, tt.licenseFile)

			req.NotNil(args, "args should not be nil")
			req.NotEmpty(args, "args should not be empty")

			argsString := strings.Join(args, " ")

			for _, expected := range tt.shouldContain {
				req.Contains(argsString, expected,
					"args should contain %q, got: %s", expected, argsString)
			}

			for _, notExpected := range tt.shouldNotContain {
				req.NotContains(argsString, notExpected,
					"args should not contain %q, got: %s", notExpected, argsString)
			}
		})
	}
}
