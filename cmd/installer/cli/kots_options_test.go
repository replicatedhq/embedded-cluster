package cli

import (
	"testing"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/require"
)

func Test_kotsInstallOptionsFromFlags(t *testing.T) {
	tests := []struct {
		name                   string
		configValuesFile       string
		airgapBundle           string
		ignoreAppPreflights    bool
		expectedConfigFile     string
		expectedAirgapBundle   string
		expectedSkipPreflights bool
	}{
		{
			name:                   "with config values file",
			configValuesFile:       "/tmp/config-values.yaml",
			airgapBundle:           "",
			ignoreAppPreflights:    false,
			expectedConfigFile:     "/tmp/config-values.yaml",
			expectedAirgapBundle:   "",
			expectedSkipPreflights: false,
		},
		{
			name:                   "without config values",
			configValuesFile:       "",
			airgapBundle:           "",
			ignoreAppPreflights:    false,
			expectedConfigFile:     "",
			expectedAirgapBundle:   "",
			expectedSkipPreflights: false,
		},
		{
			name:                   "with config values and airgap bundle",
			configValuesFile:       "/tmp/config-values.yaml",
			airgapBundle:           "/tmp/airgap.tar.gz",
			ignoreAppPreflights:    false,
			expectedConfigFile:     "/tmp/config-values.yaml",
			expectedAirgapBundle:   "/tmp/airgap.tar.gz",
			expectedSkipPreflights: false,
		},
		{
			name:                   "with all flags set",
			configValuesFile:       "/tmp/config-values.yaml",
			airgapBundle:           "/tmp/airgap.tar.gz",
			ignoreAppPreflights:    true,
			expectedConfigFile:     "/tmp/config-values.yaml",
			expectedAirgapBundle:   "/tmp/airgap.tar.gz",
			expectedSkipPreflights: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			// Create mock flags and install config
			flags := installFlags{
				configValues:        tt.configValuesFile,
				airgapBundle:        tt.airgapBundle,
				ignoreAppPreflights: tt.ignoreAppPreflights,
			}

			installCfg := &installConfig{
				clusterID:    "test-cluster-123",
				licenseBytes: []byte("license-data"),
				license: &kotsv1beta1.License{
					Spec: kotsv1beta1.LicenseSpec{
						AppSlug: "test-app",
					},
				},
			}

			// Call function
			opts := kotsInstallOptionsFromFlags(flags, installCfg, "kotsadm")

			// Validate all fields
			req.Equal(tt.expectedConfigFile, opts.ConfigValuesFile,
				"ConfigValuesFile should match")
			req.Equal(tt.expectedAirgapBundle, opts.AirgapBundle,
				"AirgapBundle should match")
			req.Equal(tt.expectedSkipPreflights, opts.SkipPreflights,
				"SkipPreflights should match")

			// Validate fields from installConfig
			req.Equal("test-app", opts.AppSlug,
				"AppSlug should be set from license")
			req.Equal("kotsadm", opts.Namespace,
				"Namespace should be set from parameter")
			req.Equal("test-cluster-123", opts.ClusterID,
				"ClusterID should be set from installConfig")
			req.Equal([]byte("license-data"), opts.License,
				"License should be set from installConfig")
		})
	}
}
