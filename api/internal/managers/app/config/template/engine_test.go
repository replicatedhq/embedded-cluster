package template

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/release"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_BasicTemplating(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "database",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "database_host",
							Default: multitype.BoolOrString{StrVal: "localhost"},
						},
						{
							Name:    "database_port",
							Default: multitype.BoolOrString{StrVal: "5432"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	// Test basic sprig function
	result, err := engine.ProcessTemplate("{{  upper \"hello world\"  }}", nil)
	require.NoError(t, err)
	assert.Equal(t, "HELLO WORLD", result)

	// Test ConfigOption with default values
	result, err = engine.ProcessTemplate("{{  ConfigOption \"database_host\"  }}", nil)
	require.NoError(t, err)
	assert.Equal(t, "localhost", result)
}

func TestEngine_ConfigOptionResolution(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "database",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "database_host",
							Default: multitype.BoolOrString{StrVal: "localhost"},
						},
						{
							Name:    "database_port",
							Default: multitype.BoolOrString{StrVal: "5432"},
						},
						{
							Name:    "database_url",
							Default: multitype.BoolOrString{StrVal: "postgres://{{  ConfigOption \"database_host\" }}:{{  ConfigOption \"database_port\" }}/app"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	// Test with user-provided values
	configValues := types.AppConfigValues{
		"database_host": {Value: "db.example.com"},
		"database_port": {Value: "5433"},
	}

	result, err := engine.ProcessTemplate("{{  ConfigOption \"database_url\"  }}", configValues)
	require.NoError(t, err)
	assert.Equal(t, "postgres://db.example.com:5433/app", result)
}

func TestEngine_ConfigOptionEquals(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "storage",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "storage_type",
							Default: multitype.BoolOrString{StrVal: "local"},
						},
						{
							Name:    "s3_bucket",
							Default: multitype.BoolOrString{StrVal: "my-app-bucket"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	configValues := types.AppConfigValues{
		"storage_type": {Value: "s3"},
	}

	// Test ConfigOptionEquals - true case
	result, err := engine.ProcessTemplate("{{  if ConfigOptionEquals \"storage_type\" \"s3\" }}S3 Storage{{  else }}Local Storage{{  end }}", configValues)
	require.NoError(t, err)
	assert.Equal(t, "S3 Storage", result)

	// Test ConfigOptionEquals - false case
	result, err = engine.ProcessTemplate("{{  if ConfigOptionEquals \"storage_type\" \"local\" }}Local Storage{{  else }}S3 Storage{{  end }}", configValues)
	require.NoError(t, err)
	assert.Equal(t, "S3 Storage", result)
}

func TestEngine_ConfigOptionData(t *testing.T) {
	testData := "Hello, World!"
	encodedData := base64.StdEncoding.EncodeToString([]byte(testData))

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "files",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "ssl_cert",
							Default: multitype.BoolOrString{StrVal: encodedData},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	result, err := engine.ProcessTemplate("{{  ConfigOptionData \"ssl_cert\" }}", nil)
	require.NoError(t, err)
	assert.Equal(t, testData, result)
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
			result, err := engine.ProcessTemplate(fmt.Sprintf("{{ LicenseFieldValue \"%s\" }}", tc.field), nil)
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

	result, err := engine.ProcessTemplate("{{  LicenseFieldValue \"customerName\" }}", nil)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestEngine_CircularDependency(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "test",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "item_a",
							Default: multitype.BoolOrString{StrVal: "{{  ConfigOption \"item_b\" }}"},
						},
						{
							Name:    "item_b",
							Default: multitype.BoolOrString{StrVal: "{{  ConfigOption \"item_a\" }}"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	_, err := engine.ProcessTemplate("{{  ConfigOption \"item_a\" }}", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency detected for item_a")
}

func TestEngine_DeepDependencyChain(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "test",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "env",
							Default: multitype.BoolOrString{StrVal: "production"},
						},
						{
							Name:    "region",
							Default: multitype.BoolOrString{StrVal: "{{  if ConfigOptionEquals \"env\" \"production\" }}us-east-1{{  else }}us-west-2{{  end }}"},
						},
						{
							Name:    "cluster_name",
							Default: multitype.BoolOrString{StrVal: "{{  ConfigOption \"env\" }}-{{  ConfigOption \"region\" }}"},
						},
						{
							Name:    "database_host",
							Default: multitype.BoolOrString{StrVal: "{{  ConfigOption \"cluster_name\" }}.db.internal"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	result, err := engine.ProcessTemplate("{{  ConfigOption \"database_host\" }}", nil)
	require.NoError(t, err)
	assert.Equal(t, "production-us-east-1.db.internal", result)
}

func TestEngine_ComplexTemplate(t *testing.T) {
	license := &kotsv1beta1.License{
		Spec: kotsv1beta1.LicenseSpec{
			CustomerName: "Acme Corp",
			LicenseID:    "license-123",
		},
	}

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "database",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "database_host",
							Default: multitype.BoolOrString{StrVal: "localhost"},
						},
						{
							Name:    "database_port",
							Default: multitype.BoolOrString{StrVal: "5432"},
						},
						{
							Name:    "database_url",
							Default: multitype.BoolOrString{StrVal: "postgres://{{  ConfigOption \"database_host\" }}:{{  ConfigOption \"database_port\" }}/app"},
						},
						{
							Name:    "database_enabled",
							Default: multitype.BoolOrString{StrVal: "true"},
						},
					},
				},
				{
					Name: "storage",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "storage_type",
							Default: multitype.BoolOrString{StrVal: "local"},
						},
						{
							Name:    "s3_bucket",
							Default: multitype.BoolOrString{StrVal: "my-app-bucket"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config, WithLicense(license))

	configValues := types.AppConfigValues{
		"database_host": {Value: "db.example.com"},
		"database_port": {Value: "5432"},
		"storage_type":  {Value: "s3"},
		"s3_bucket":     {Value: "production-bucket"},
	}

	template := `database:
  enabled: {{  ConfigOption "database_enabled" }}
  {{  if ConfigOptionEquals "database_enabled" "true" }}
  url: {{  ConfigOption "database_url" }}
  {{  end }}
storage:
  type: {{  ConfigOption "storage_type" }}
  {{  if ConfigOptionEquals "storage_type" "s3" }}
  bucket: {{  ConfigOption "s3_bucket" }}
  {{  end }}
license:
  customer: {{  LicenseFieldValue "customerName" }}
  id: {{  LicenseFieldValue "licenseID" }}`

	result, err := engine.ProcessTemplate(template, configValues)
	require.NoError(t, err)

	expected := `database:
  enabled: true
  
  url: postgres://db.example.com:5432/app
  
storage:
  type: s3
  
  bucket: production-bucket
  
license:
  customer: Acme Corp
  id: license-123`

	assert.Equal(t, expected, result)
}

func TestEngine_ParseAndExecuteSeparately(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "database",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "database_host",
							Default: multitype.BoolOrString{StrVal: "localhost"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	// Parse once
	tmpl, err := engine.Parse("Host: {{  ConfigOption \"database_host\" }}")
	require.NoError(t, err)
	require.NotNil(t, tmpl)

	// Execute with different config values
	configValues1 := types.AppConfigValues{
		"database_host": {Value: "db1.example.com"},
	}
	result1, err := engine.Execute(tmpl, configValues1)
	require.NoError(t, err)
	assert.Equal(t, "Host: db1.example.com", result1)

	configValues2 := types.AppConfigValues{
		"database_host": {Value: "db2.example.com"},
	}
	result2, err := engine.Execute(tmpl, configValues2)
	require.NoError(t, err)
	assert.Equal(t, "Host: db2.example.com", result2)
}

func TestEngine_EmptyTemplate(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)

	// Test empty template string
	tmpl, err := engine.Parse("")
	require.NoError(t, err)
	assert.Nil(t, tmpl)

	result, err := engine.Execute(nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestEngine_UnknownConfigItem(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)

	_, err := engine.ProcessTemplate("{{  ConfigOption \"nonexistent\" }}", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config item nonexistent not found")
}

func TestEngine_ValuePriority(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "test",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "test_item",
							Value:   multitype.BoolOrString{StrVal: "config_value"},
							Default: multitype.BoolOrString{StrVal: "default_value"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	// Test with no user value - should use config value
	result, err := engine.ProcessTemplate("{{  ConfigOption \"test_item\" }}", nil)
	require.NoError(t, err)
	assert.Equal(t, "config_value", result)

	// Test with user value - should override config value
	configValues := types.AppConfigValues{
		"test_item": {Value: "user_value"},
	}
	result, err = engine.ProcessTemplate("{{  ConfigOption \"test_item\" }}", configValues)
	require.NoError(t, err)
	assert.Equal(t, "user_value", result)
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

	result, err := engine.ProcessTemplate("{{ LicenseFieldValue \"endpoint\" }}", nil)
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

	result, err := engine.ProcessTemplate("{{ LicenseFieldValue \"endpoint\" }}", nil)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}
