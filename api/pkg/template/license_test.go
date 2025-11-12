package template

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	kotsv1beta2 "github.com/replicatedhq/kotskinds/apis/kots/v1beta2"
	"github.com/replicatedhq/kotskinds/pkg/licensewrapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to wrap v1beta1 license in LicenseWrapper for testing
func wrapLicense(license *kotsv1beta1.License) *licensewrapper.LicenseWrapper {
	return &licensewrapper.LicenseWrapper{
		V1: license,
	}
}

// Helper function to wrap v1beta2 license in LicenseWrapper for testing
func wrapLicenseV2(license *kotsv1beta2.License) *licensewrapper.LicenseWrapper {
	return &licensewrapper.LicenseWrapper{
		V2: license,
	}
}

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

	engine := NewEngine(config, WithLicense(wrapLicense(license)))

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
	_, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "license is nil")
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

	engine := NewEngine(config, WithLicense(wrapLicense(license)), WithReleaseData(releaseData))

	err := engine.Parse("{{repl LicenseFieldValue \"endpoint\" }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "https://my-app.example.com", result)
}

func TestEngine_LicenseFieldValue_EndpointWithoutReleaseData(t *testing.T) {
	license := &kotsv1beta2.License{
		Spec: kotsv1beta2.LicenseSpec{
			CustomerName: "Acme Corp",
			LicenseID:    "license-123",
		},
	}

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config, WithLicense(wrapLicenseV2(license)))

	err := engine.Parse("{{repl LicenseFieldValue \"endpoint\" }}")
	require.NoError(t, err)
	_, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "release data is nil")
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
				ProxyRegistryDomain:      "proxy.my-app.example.com",
			},
		},
	}

	engine := NewEngine(config, WithLicense(wrapLicense(license)), WithReleaseData(releaseData))

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
	proxyAuth, ok := auths["proxy.my-app.example.com"].(map[string]interface{})
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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "license is nil")
}

func TestEngine_LicenseDockerCfgWithoutReleaseData(t *testing.T) {
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			LicenseID: "license-456",
		},
	}

	engine := NewEngine(nil, WithLicense(wrapLicense(license)))

	err := engine.Parse("{{repl LicenseDockerCfg }}")
	require.NoError(t, err)
	_, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	assert.Error(t, err)
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
	releaseData := &release.ReleaseData{
		ChannelRelease: &release.ChannelRelease{
			ChannelID: "channel-456",
		},
	}

	engine := NewEngine(config, WithLicense(wrapLicense(license)), WithReleaseData(releaseData))

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

	// The (staging) endpoint in license should not affect the domains being used
	proxyAuth, ok := auths["proxy.replicated.com"].(map[string]interface{})
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
				ProxyRegistryDomain:      "staging-proxy.example.com",
			},
		},
	}

	engine := NewEngine(config, WithLicense(wrapLicense(license)), WithReleaseData(releaseData))

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

	// Domains should come from release data and not be affected by the staging endpoint in license:
	// - Proxy: ProxyRegistryDomain from release data
	// - Registry: ReplicatedRegistryDomain from release data
	proxyAuth, ok := auths["staging-proxy.example.com"].(map[string]interface{})
	require.True(t, ok, "custom staging proxy auth should exist")
	require.Contains(t, proxyAuth, "auth")

	registryAuth, ok := auths["staging-registry.example.com"].(map[string]interface{})
	require.True(t, ok, "custom staging registry auth should exist")
	require.Contains(t, registryAuth, "auth")

	// Verify the auth value is base64 encoded license:license
	expectedAuth := base64.StdEncoding.EncodeToString([]byte("license-789:license-789"))
	assert.Equal(t, expectedAuth, proxyAuth["auth"])
	assert.Equal(t, expectedAuth, registryAuth["auth"])
}

func TestEngine_ChannelName(t *testing.T) {
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			LicenseID:   "license-123",
			ChannelID:   "fallback-channel-id",
			ChannelName: "Fallback Channel",
			Channels: []kotsv1beta1.Channel{
				{
					ChannelID:   "channel-456",
					ChannelName: "Stable",
				},
				{
					ChannelID:   "channel-789",
					ChannelName: "Beta",
				},
			},
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
			ChannelID: "channel-456",
		},
	}

	engine := NewEngine(config, WithLicense(wrapLicense(license)), WithReleaseData(releaseData))

	err := engine.Parse("{{repl ChannelName }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "Stable", result)
}

func TestEngine_ChannelName_FallbackToLicenseChannel(t *testing.T) {
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			LicenseID:   "license-123",
			ChannelID:   "fallback-channel-id",
			ChannelName: "Fallback Channel",
			Channels: []kotsv1beta1.Channel{
				{
					ChannelID:   "channel-456",
					ChannelName: "Stable",
				},
			},
		},
	}

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	// Mock release data where ChannelID matches the fallback ChannelID
	releaseData := &release.ReleaseData{
		EmbeddedClusterConfig: &ecv1beta1.Config{
			Spec: ecv1beta1.ConfigSpec{},
		},
		ChannelRelease: &release.ChannelRelease{
			ChannelID: "fallback-channel-id",
		},
	}

	engine := NewEngine(config, WithLicense(wrapLicense(license)), WithReleaseData(releaseData))

	err := engine.Parse("{{repl ChannelName }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "Fallback Channel", result)
}

func TestEngine_ChannelName_WithoutLicense(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)

	err := engine.Parse("{{repl ChannelName }}")
	require.NoError(t, err)
	_, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "license is nil")
}

func TestEngine_ChannelName_WithoutReleaseData(t *testing.T) {
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			LicenseID:   "license-123",
			ChannelID:   "channel-456",
			ChannelName: "Stable",
		},
	}

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config, WithLicense(wrapLicense(license)))

	err := engine.Parse("{{repl ChannelName }}")
	require.NoError(t, err)
	_, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "release data is nil")
}

func TestEngine_ChannelName_WithoutChannelRelease(t *testing.T) {
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			LicenseID:   "license-123",
			ChannelID:   "channel-456",
			ChannelName: "Stable",
		},
	}

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	// Mock release data without ChannelRelease
	releaseData := &release.ReleaseData{
		EmbeddedClusterConfig: &ecv1beta1.Config{
			Spec: ecv1beta1.ConfigSpec{},
		},
	}

	engine := NewEngine(config, WithLicense(wrapLicense(license)), WithReleaseData(releaseData))

	err := engine.Parse("{{repl ChannelName }}")
	require.NoError(t, err)
	_, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "channel release is nil")
}

func TestEngine_ChannelName_ChannelNotFound(t *testing.T) {
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			LicenseID:   "license-123",
			ChannelID:   "fallback-channel-id",
			ChannelName: "Fallback Channel",
			Channels: []kotsv1beta1.Channel{
				{
					ChannelID:   "channel-456",
					ChannelName: "Stable",
				},
			},
		},
	}

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	// Mock release data with a channel that doesn't exist in the license
	releaseData := &release.ReleaseData{
		EmbeddedClusterConfig: &ecv1beta1.Config{
			Spec: ecv1beta1.ConfigSpec{},
		},
		ChannelRelease: &release.ChannelRelease{
			ChannelID: "unknown-channel-id",
		},
	}

	engine := NewEngine(config, WithLicense(wrapLicense(license)), WithReleaseData(releaseData))

	err := engine.Parse("{{repl ChannelName }}")
	require.NoError(t, err)
	_, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "channel unknown-channel-id not found in license")
}
