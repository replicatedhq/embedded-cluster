package template

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_LicenseFieldValue(t *testing.T) {
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			// Basic license fields
			CustomerName:    "Acme Corp",
			LicenseID:       "license-123",
			LicenseType:     "prod",
			LicenseSequence: 456,
			Signature:       []byte("signature-data"),
			AppSlug:         "my-app",
			ChannelID:       "channel-456",
			ChannelName:     "Stable",

			// Boolean feature flags
			IsSnapshotSupported:               true,
			IsDisasterRecoverySupported:       false,
			IsGitOpsSupported:                 true,
			IsSupportBundleUploadSupported:    false,
			IsEmbeddedClusterMultiNodeEnabled: true,
			IsIdentityServiceSupported:        false,
			IsGeoaxisSupported:                true,
			IsAirgapSupported:                 false,
			IsSemverRequired:                  true,

			// Custom entitlements
			Entitlements: map[string]kotsv1beta1.EntitlementField{
				"maxNodes": {
					Value: kotsv1beta1.EntitlementValue{
						Type:   kotsv1beta1.String,
						StrVal: "10",
					},
				},
				"storageLimit": {
					Value: kotsv1beta1.EntitlementValue{
						Type:   kotsv1beta1.Int,
						IntVal: 100,
					},
				},
				"isFeatureEnabled": {
					Value: kotsv1beta1.EntitlementValue{
						Type:    kotsv1beta1.Bool,
						BoolVal: true,
					},
				},
			},
		},
	}

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config, WithLicense(license))

	// Test basic license fields
	testCases := []struct {
		field    string
		expected string
	}{
		{"customerName", "Acme Corp"},
		{"licenseID", "license-123"},
		{"licenseId", "license-123"}, // Test alias
		{"licenseType", "prod"},
		{"licenseSequence", "456"},
		{"signature", "signature-data"},
		{"appSlug", "my-app"},
		{"channelID", "channel-456"},
		{"channelName", "Stable"},

		// Boolean feature flags
		{"isSnapshotSupported", "true"},
		{"IsDisasterRecoverySupported", "false"},
		{"isGitOpsSupported", "true"},
		{"isSupportBundleUploadSupported", "false"},
		{"isEmbeddedClusterMultiNodeEnabled", "true"},
		{"isIdentityServiceSupported", "false"},
		{"isGeoaxisSupported", "true"},
		{"isAirgapSupported", "false"},
		{"isSemverRequired", "true"},

		// Custom entitlements
		{"maxNodes", "10"},
		{"storageLimit", "100"},
		{"isFeatureEnabled", "true"},

		// Endpoint field (should be empty without releaseData)
		{"endpoint", ""},

		// Unknown field
		{"unknownField", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.field, func(t *testing.T) {
			err := engine.Parse(fmt.Sprintf("{{repl LicenseFieldValue \"%s\" }}", tc.field))
			require.NoError(t, err)
			result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result, "Field %s should return %s", tc.field, tc.expected)
		})
	}
}

func TestEngine_LicenseFieldValueWithoutLicense(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)

	err := engine.Parse("{{repl LicenseFieldValue \"customerName\" }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestEngine_LicenseFieldValue_Endpoint(t *testing.T) {
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			CustomerName: "Acme Corp",
			LicenseID:    "license-123",
		},
	}

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	// Mock release data with embedded cluster config
	releaseData := &release.ReleaseData{
		EmbeddedClusterConfig: &ecv1beta1.Config{
			Spec: ecv1beta1.ConfigSpec{},
		},
		ChannelRelease: &release.ChannelRelease{
			DefaultDomains: release.Domains{
				ReplicatedAppDomain: "my-app.example.com",
			},
		},
	}

	engine := NewEngine(config, WithLicense(license), WithReleaseData(releaseData))

	err := engine.Parse("{{repl LicenseFieldValue \"endpoint\" }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "https://my-app.example.com", result)
}

func TestEngine_LicenseFieldValue_EndpointWithoutReleaseData(t *testing.T) {
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			CustomerName: "Acme Corp",
			LicenseID:    "license-123",
		},
	}

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config, WithLicense(license))

	err := engine.Parse("{{repl LicenseFieldValue \"endpoint\" }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestEngine_LicenseDockerCfg(t *testing.T) {
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			LicenseID: "license-123",
		},
	}

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	// Mock release data with embedded cluster config
	releaseData := &release.ReleaseData{
		EmbeddedClusterConfig: &ecv1beta1.Config{
			Spec: ecv1beta1.ConfigSpec{},
		},
		ChannelRelease: &release.ChannelRelease{
			DefaultDomains: release.Domains{
				ReplicatedAppDomain:      "my-app.example.com",
				ReplicatedRegistryDomain: "registry.my-app.example.com",
			},
		},
	}

	engine := NewEngine(config, WithLicense(license), WithReleaseData(releaseData))

	err := engine.Parse("{{repl LicenseDockerCfg }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)

	// Decode the base64 result to verify the JSON structure
	decoded, err := base64.StdEncoding.DecodeString(result)
	require.NoError(t, err)

	// Verify the JSON structure
	var dockercfg map[string]interface{}
	err = json.Unmarshal(decoded, &dockercfg)
	require.NoError(t, err)

	// Check that auths key exists
	auths, ok := dockercfg["auths"].(map[string]interface{})
	require.True(t, ok, "auths should be a map")

	// Check that both proxy and registry domains are present
	proxyAuth, ok := auths["my-app.example.com"].(map[string]interface{})
	require.True(t, ok, "proxy auth should exist")
	require.Contains(t, proxyAuth, "auth")

	registryAuth, ok := auths["registry.my-app.example.com"].(map[string]interface{})
	require.True(t, ok, "registry auth should exist")
	require.Contains(t, registryAuth, "auth")

	// Verify the auth value is base64 encoded license:license
	expectedAuth := base64.StdEncoding.EncodeToString([]byte("license-123:license-123"))
	assert.Equal(t, expectedAuth, proxyAuth["auth"])
	assert.Equal(t, expectedAuth, registryAuth["auth"])
}

func TestEngine_LicenseDockerCfgWithoutLicense(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)

	err := engine.Parse("{{repl LicenseDockerCfg }}")
	require.NoError(t, err)
	_, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "license is nil")
}

func TestEngine_LicenseDockerCfgWithoutReleaseData(t *testing.T) {
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			LicenseID: "license-456",
		},
	}

	engine := NewEngine(nil, WithLicense(license))

	err := engine.Parse("{{repl LicenseDockerCfg }}")
	require.NoError(t, err)
	_, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "release data is nil")
}

func TestEngine_LicenseDockerCfgStagingEndpoint(t *testing.T) {
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			LicenseID: "license-456",
			Endpoint:  "https://staging.replicated.app",
		},
	}

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	// Mock release data with embedded cluster config
	releaseData := &release.ReleaseData{}

	engine := NewEngine(config, WithLicense(license), WithReleaseData(releaseData))

	err := engine.Parse("{{repl LicenseDockerCfg }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)

	// Decode the base64 result to verify the JSON structure
	decoded, err := base64.StdEncoding.DecodeString(result)
	require.NoError(t, err)

	// Verify the JSON structure
	var dockercfg map[string]interface{}
	err = json.Unmarshal(decoded, &dockercfg)
	require.NoError(t, err)

	// Check that auths key exists
	auths, ok := dockercfg["auths"].(map[string]interface{})
	require.True(t, ok, "auths should be a map")

	// With staging endpoint, should use staging domains:
	// - Proxy: ReplicatedAppDomain (default: "replicated.app") - not affected by staging
	// - Registry: ReplicatedRegistryDomain (default: "registry.replicated.com") - not affected by staging
	// Note: The staging endpoint only affects the getRegistryProxyInfoFromLicense function,
	// but when there's no release data, utils.GetDomains returns default domains
	proxyAuth, ok := auths["replicated.app"].(map[string]interface{})
	require.True(t, ok, "staging proxy auth should exist")
	require.Contains(t, proxyAuth, "auth")

	registryAuth, ok := auths["registry.replicated.com"].(map[string]interface{})
	require.True(t, ok, "staging registry auth should exist")
	require.Contains(t, registryAuth, "auth")

	// Verify the auth value is base64 encoded license:license
	expectedAuth := base64.StdEncoding.EncodeToString([]byte("license-456:license-456"))
	assert.Equal(t, expectedAuth, proxyAuth["auth"])
	assert.Equal(t, expectedAuth, registryAuth["auth"])
}

func TestEngine_LicenseDockerCfgStagingEndpointWithReleaseData(t *testing.T) {
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			LicenseID: "license-789",
			Endpoint:  "https://staging.replicated.app",
		},
	}

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	// Mock release data with embedded cluster config
	releaseData := &release.ReleaseData{
		EmbeddedClusterConfig: &ecv1beta1.Config{
			Spec: ecv1beta1.ConfigSpec{},
		},
		ChannelRelease: &release.ChannelRelease{
			DefaultDomains: release.Domains{
				ReplicatedAppDomain:      "staging-app.example.com",
				ReplicatedRegistryDomain: "staging-registry.example.com",
			},
		},
	}

	engine := NewEngine(config, WithLicense(license), WithReleaseData(releaseData))

	err := engine.Parse("{{repl LicenseDockerCfg }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)

	// Decode the base64 result to verify the JSON structure
	decoded, err := base64.StdEncoding.DecodeString(result)
	require.NoError(t, err)

	// Verify the JSON structure
	var dockercfg map[string]interface{}
	err = json.Unmarshal(decoded, &dockercfg)
	require.NoError(t, err)

	// Check that auths key exists
	auths, ok := dockercfg["auths"].(map[string]interface{})
	require.True(t, ok, "auths should be a map")

	// With staging endpoint and release data, should use release data domains:
	// - Proxy: ReplicatedAppDomain from release data
	// - Registry: ReplicatedRegistryDomain from release data
	proxyAuth, ok := auths["staging-app.example.com"].(map[string]interface{})
	require.True(t, ok, "staging proxy auth should exist")
	require.Contains(t, proxyAuth, "auth")

	registryAuth, ok := auths["staging-registry.example.com"].(map[string]interface{})
	require.True(t, ok, "staging registry auth should exist")
	require.Contains(t, registryAuth, "auth")

	// Verify the auth value is base64 encoded license:license
	expectedAuth := base64.StdEncoding.EncodeToString([]byte("license-789:license-789"))
	assert.Equal(t, expectedAuth, proxyAuth["auth"])
	assert.Equal(t, expectedAuth, registryAuth["auth"])
}
