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

	// Test basic sprig function with {{repl format
	err := engine.Parse("{{repl upper \"hello world\"  }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "HELLO WORLD", result)

	// Test ConfigOption with default values using repl{{ format
	err = engine.Parse("repl{{ ConfigOption \"database_host\"  }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "localhost", result)

	// Test mixing both delimiter formats in one template
	err = engine.Parse("Host: repl{{ ConfigOption \"database_host\" }} Port: {{repl ConfigOption \"database_port\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "Host: localhost Port: 5432", result)
}

func TestEngine_ValuePriority(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "database",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "database_host",
							Value:   multitype.BoolOrString{StrVal: "db-internal.company.com"},
							Default: multitype.BoolOrString{StrVal: "localhost"},
						},
						{
							Name:    "database_port",
							Default: multitype.BoolOrString{StrVal: "5432"},
						},
						{
							Name:    "redis_host",
							Default: multitype.BoolOrString{StrVal: "redis.company.com"},
						},
						{
							Name: "metrics_endpoint",
							// No Value and no Default - should return empty string
						},
						{
							Name:    "database_url",
							Default: multitype.BoolOrString{StrVal: "postgres://repl{{ ConfigOption \"database_host\" }}:{{repl ConfigOption \"database_port\" }}/app"},
						},
						{
							Name:    "empty_template_value",
							Value:   multitype.BoolOrString{StrVal: "repl{{ if false }}never_shown{{repl end }}"},
							Default: multitype.BoolOrString{StrVal: "fallback_default"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	// Test with user-provided values (should override config values)
	configValues := types.AppConfigValues{
		"database_host": {Value: "db.example.com"},
		"database_port": {Value: "5433"},
	}

	err := engine.Parse("{{repl ConfigOption \"database_url\" }}")
	require.NoError(t, err)
	result, err := engine.Execute(configValues)
	require.NoError(t, err)
	assert.Equal(t, "postgres://db.example.com:5433/app", result)

	// Test with partial user values (should use config value for database_host, user value for database_port)
	partialConfigValues := types.AppConfigValues{
		"database_port": {Value: "5433"},
	}

	err = engine.Parse("{{repl ConfigOption \"database_url\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(partialConfigValues)
	require.NoError(t, err)
	assert.Equal(t, "postgres://db-internal.company.com:5433/app", result)

	// Test with no user values (should use config value for database_host, default for database_port)
	err = engine.Parse("{{repl ConfigOption \"database_url\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "postgres://db-internal.company.com:5432/app", result)

	// Test item with only default value (redis_host) - should use default
	err = engine.Parse("{{repl ConfigOption \"redis_host\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "redis.company.com", result)

	// Test item with only default value but user override - should use user value
	configValues = types.AppConfigValues{
		"redis_host": {Value: "redis.production.com"},
	}
	err = engine.Parse("{{repl ConfigOption \"redis_host\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(configValues)
	require.NoError(t, err)
	assert.Equal(t, "redis.production.com", result)

	// Test item with no value and no default - should return empty string
	err = engine.Parse("{{repl ConfigOption \"metrics_endpoint\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "", result)

	// Test item with no value and no default but user override - should use user value
	configValues = types.AppConfigValues{
		"metrics_endpoint": {Value: "https://metrics.company.com"},
	}
	err = engine.Parse("{{repl ConfigOption \"metrics_endpoint\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(configValues)
	require.NoError(t, err)
	assert.Equal(t, "https://metrics.company.com", result)

	// Test item with template value that evaluates to empty - should fall back to default
	err = engine.Parse("{{repl ConfigOption \"empty_template_value\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "fallback_default", result)

	// Test with empty user value (should use empty string, not fall back to config value)
	emptyConfigValues := types.AppConfigValues{
		"database_host": {Value: ""}, // Empty user value should be used as-is
		"database_port": {Value: "5433"},
	}
	err = engine.Parse("{{repl ConfigOption \"database_url\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(emptyConfigValues)
	require.NoError(t, err)
	assert.Equal(t, "postgres://:5433/app", result) // Empty host should result in empty string, not config fallback
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
							Value:   multitype.BoolOrString{StrVal: "filesystem"},
							Default: multitype.BoolOrString{StrVal: "local"},
						},
						{
							Name:    "backup_type",
							Default: multitype.BoolOrString{StrVal: "snapshot"},
						},
						{
							Name:    "s3_bucket",
							Default: multitype.BoolOrString{StrVal: "my-app-backups"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	// Test with user value override - should use user value "s3"
	configValues := types.AppConfigValues{
		"storage_type": {Value: "s3"},
	}

	err := engine.Parse("repl{{ if ConfigOptionEquals \"storage_type\" \"s3\" }}S3 Storage{{repl else }}Other Storage{{repl end }}")
	require.NoError(t, err)
	result, err := engine.Execute(configValues)
	require.NoError(t, err)
	assert.Equal(t, "S3 Storage", result)

	// Test with no user value - should use config value "filesystem"
	err = engine.Parse("{{repl if ConfigOptionEquals \"storage_type\" \"filesystem\" }}Filesystem Storage{{repl else }}Other Storage{{repl end }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "Filesystem Storage", result)

	// Test with item that has only default value - should use default "snapshot"
	err = engine.Parse("repl{{ if ConfigOptionEquals \"backup_type\" \"snapshot\" }}Snapshot Backup{{repl else }}Other Backup{{repl end }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "Snapshot Backup", result)
}

func TestEngine_ConfigOptionData(t *testing.T) {
	// Sample certificate content
	defaultCertContent := "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJANGt7tgH..."
	defaultCertEncoded := base64.StdEncoding.EncodeToString([]byte(defaultCertContent))

	// Config-provided certificate content
	configCertContent := "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJAConfig..."
	configCertEncoded := base64.StdEncoding.EncodeToString([]byte(configCertContent))

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "certificates",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "ssl_cert",
							Value:   multitype.BoolOrString{StrVal: configCertEncoded},
							Default: multitype.BoolOrString{StrVal: defaultCertEncoded},
						},
						{
							Name:    "ca_cert",
							Default: multitype.BoolOrString{StrVal: defaultCertEncoded},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	// Test with no user value - should use config value
	err := engine.Parse("{{repl ConfigOptionData \"ssl_cert\" }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, configCertContent, result)

	// Test with user value - should override config value
	userCertContent := "-----BEGIN CERTIFICATE-----\nMIIBkTCB+wIJAUser..."
	userCertEncoded := base64.StdEncoding.EncodeToString([]byte(userCertContent))
	configValues := types.AppConfigValues{
		"ssl_cert": {Value: userCertEncoded},
	}

	err = engine.Parse("{{repl ConfigOptionData \"ssl_cert\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(configValues)
	require.NoError(t, err)
	assert.Equal(t, userCertContent, result)

	// Test with item that has only default value - should use default
	err = engine.Parse("{{repl ConfigOptionData \"ca_cert\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, defaultCertContent, result)
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
			err := engine.Parse(fmt.Sprintf("{{repl LicenseFieldValue \"%s\" }}", tc.field))
			require.NoError(t, err)
			result, err := engine.Execute(nil)
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
	result, err := engine.Execute(nil)
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
							Default: multitype.BoolOrString{StrVal: "{{repl ConfigOption \"item_b\" }}"},
						},
						{
							Name:    "item_b",
							Default: multitype.BoolOrString{StrVal: "{{repl ConfigOption \"item_a\" }}"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	err := engine.Parse("{{repl ConfigOption \"item_a\" }}")
	require.NoError(t, err)
	_, err = engine.Execute(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency detected for item_a")
}

func TestEngine_DeepDependencyChain(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "deployment",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "environment",
							Value:   multitype.BoolOrString{StrVal: "staging"},
							Default: multitype.BoolOrString{StrVal: "development"},
						},
						{
							Name:    "aws_region",
							Value:   multitype.BoolOrString{StrVal: "repl{{ if ConfigOptionEquals \"environment\" \"production\" }}us-east-1{{repl else }}us-west-2{{repl end }}"},
							Default: multitype.BoolOrString{StrVal: "us-central-1"},
						},
						{
							Name:    "cluster_name",
							Default: multitype.BoolOrString{StrVal: "{{repl ConfigOption \"environment\" }}-repl{{ ConfigOption \"aws_region\" }}"},
						},
						{
							Name:    "database_host",
							Default: multitype.BoolOrString{StrVal: "{{repl ConfigOption \"cluster_name\" }}.rds.amazonaws.com"},
						},
						{
							Name:    "redis_host",
							Value:   multitype.BoolOrString{StrVal: "{{repl ConfigOption \"cluster_name\" }}.elasticache.amazonaws.com"},
							Default: multitype.BoolOrString{StrVal: "localhost"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	// Test with no user values - should use config values and resolve deep dependency chain
	err := engine.Parse("{{repl ConfigOption \"database_host\" }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "staging-us-west-2.rds.amazonaws.com", result)

	// Test another item with config value that depends on the chain
	err = engine.Parse("{{repl ConfigOption \"redis_host\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "staging-us-west-2.elasticache.amazonaws.com", result)

	// Test with user override - should change the entire chain
	configValues := types.AppConfigValues{
		"environment": {Value: "production"},
	}
	err = engine.Parse("{{repl ConfigOption \"database_host\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(configValues)
	require.NoError(t, err)
	assert.Equal(t, "production-us-east-1.rds.amazonaws.com", result)
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
							Value:   multitype.BoolOrString{StrVal: "db-internal.company.com"},
							Default: multitype.BoolOrString{StrVal: "localhost"},
						},
						{
							Name:    "database_port",
							Default: multitype.BoolOrString{StrVal: "5432"},
						},
						{
							Name:    "database_url",
							Default: multitype.BoolOrString{StrVal: "postgres://repl{{ ConfigOption \"database_host\" }}:{{repl ConfigOption \"database_port\" }}/app"},
						},
						{
							Name:    "database_enabled",
							Value:   multitype.BoolOrString{StrVal: "true"},
							Default: multitype.BoolOrString{StrVal: "false"},
						},
					},
				},
				{
					Name: "storage",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "storage_type",
							Value:   multitype.BoolOrString{StrVal: "filesystem"},
							Default: multitype.BoolOrString{StrVal: "memory"},
						},
						{
							Name:    "s3_bucket",
							Default: multitype.BoolOrString{StrVal: "company-app-backups"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config, WithLicense(license))

	// Test with user values overriding config values
	configValues := types.AppConfigValues{
		"database_host": {Value: "db.production.com"},
		"database_port": {Value: "5432"},
		"storage_type":  {Value: "s3"},
		"s3_bucket":     {Value: "production-backups"},
	}

	template := `database:
  enabled: repl{{ ConfigOption "database_enabled" }}
  {{repl if ConfigOptionEquals "database_enabled" "true" }}
  url: repl{{ ConfigOption "database_url" }}
  {{repl end }}
storage:
  type: {{repl ConfigOption "storage_type" }}
  repl{{ if ConfigOptionEquals "storage_type" "s3" }}
  bucket: {{repl ConfigOption "s3_bucket" }}
  repl{{ end }}
license:
  customer: repl{{ LicenseFieldValue "customerName" }}
  id: {{repl LicenseFieldValue "licenseID" }}`

	err := engine.Parse(template)
	require.NoError(t, err)
	result, err := engine.Execute(configValues)
	require.NoError(t, err)

	expected := `database:
  enabled: true
  
  url: postgres://db.production.com:5432/app
  
storage:
  type: s3
  
  bucket: production-backups
  
license:
  customer: Acme Corp
  id: license-123`

	assert.Equal(t, expected, result)

	// Test with no user values - should use config values
	err = engine.Parse(template)
	require.NoError(t, err)
	result, err = engine.Execute(nil)
	require.NoError(t, err)

	expectedWithConfigValues := `database:
  enabled: true
  
  url: postgres://db-internal.company.com:5432/app
  
storage:
  type: filesystem
  
license:
  customer: Acme Corp
  id: license-123`

	assert.Equal(t, expectedWithConfigValues, result)
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
							Value:   multitype.BoolOrString{StrVal: "db-internal.company.com"},
							Default: multitype.BoolOrString{StrVal: "localhost"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	// Parse once
	err := engine.Parse("Host: {{repl ConfigOption \"database_host\" }}")
	require.NoError(t, err)
	require.NotNil(t, engine.tmpl)

	// Execute with different config values
	configValues1 := types.AppConfigValues{
		"database_host": {Value: "db1.production.com"},
	}
	result1, err := engine.Execute(configValues1)
	require.NoError(t, err)
	assert.Equal(t, "Host: db1.production.com", result1)

	configValues2 := types.AppConfigValues{
		"database_host": {Value: "db2.staging.com"},
	}
	result2, err := engine.Execute(configValues2)
	require.NoError(t, err)
	assert.Equal(t, "Host: db2.staging.com", result2)

	// Execute with no user values - should use config value
	result3, err := engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "Host: db-internal.company.com", result3)
}

func TestEngine_EmptyTemplate(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)

	// Test empty template string
	err := engine.Parse("")
	require.NoError(t, err)
	assert.NotNil(t, engine.tmpl)

	result, err := engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestEngine_ExecuteWithoutParsing(t *testing.T) {
	engine := NewEngine(nil)
	_, err := engine.Execute(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "template not parsed")
}

func TestEngine_UnknownConfigItem(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{},
		},
	}

	engine := NewEngine(config)

	err := engine.Parse("{{repl ConfigOption \"nonexistent\" }}")
	require.NoError(t, err)
	_, err = engine.Execute(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config item nonexistent not found")
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
	result, err := engine.Execute(nil)
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
	result, err := engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestEngine_DependencyTreeAndCaching(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "app",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "environment",
							Value:   multitype.BoolOrString{StrVal: "staging"},
							Default: multitype.BoolOrString{StrVal: "development"},
						},
						{
							Name:    "region",
							Value:   multitype.BoolOrString{StrVal: "{{repl ConfigOption \"environment\" }}-region"},
							Default: multitype.BoolOrString{StrVal: "default-region"},
						},
						{
							Name:    "database_url",
							Default: multitype.BoolOrString{StrVal: "postgres://{{repl ConfigOption \"environment\" }}:{{repl ConfigOption \"region\" }}/app"},
						},
						{
							Name:    "redis_url",
							Value:   multitype.BoolOrString{StrVal: "redis://{{repl ConfigOption \"database_url\" }}/0"},
							Default: multitype.BoolOrString{StrVal: "redis://localhost/0"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	// Define the expected dependency tree (should remain constant throughout the test)
	expectedDepsTree := map[string][]string{
		"redis_url":    {"database_url"},
		"database_url": {"environment", "region"},
		"region":       {"environment"},
		// environment doesn't appear because it has no template dependencies
	}

	// Test 1: First execution - build dependency tree
	err := engine.Parse("{{repl ConfigOption \"redis_url\" }}")
	require.NoError(t, err)
	result, err := engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://staging:staging-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Verify cache was populated
	assert.Equal(t, "staging", engine.cache["environment"].Value)
	assert.Equal(t, "staging-region", engine.cache["region"].Value)
	assert.Equal(t, "postgres://staging:staging-region/app", engine.cache["database_url"].Value)
	assert.Equal(t, "redis://postgres://staging:staging-region/app/0", engine.cache["redis_url"].Value)

	// Test 2: Second execution with no changes - should use cache
	result, err = engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://staging:staging-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Test 3: Change a user value - should invalidate dependent items
	configValues := types.AppConfigValues{
		"environment": {Value: "production"},
	}
	result, err = engine.Execute(configValues)
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://production:production-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Verify that items dependent on 'environment' were recomputed
	assert.Equal(t, "production", engine.cache["environment"].Value)
	assert.Equal(t, "production-region", engine.cache["region"].Value)
	assert.Equal(t, "postgres://production:production-region/app", engine.cache["database_url"].Value)
	assert.Equal(t, "redis://postgres://production:production-region/app/0", engine.cache["redis_url"].Value)

	// Test 4: Execute again with same user values - should use cache
	result, err = engine.Execute(configValues)
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://production:production-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Test 5: Change user value again - should detect change and invalidate
	configValues = types.AppConfigValues{
		"environment": {Value: "development"},
	}
	result, err = engine.Execute(configValues)
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://development:development-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Verify all dependent items were updated
	assert.Equal(t, "development", engine.cache["environment"].Value)
	assert.Equal(t, "development-region", engine.cache["region"].Value)
	assert.Equal(t, "postgres://development:development-region/app", engine.cache["database_url"].Value)
	assert.Equal(t, "redis://postgres://development:development-region/app/0", engine.cache["redis_url"].Value)

	// Test 6: Remove user value (go back to config value) - should invalidate
	result, err = engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://staging:staging-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Should be back to original config values
	assert.Equal(t, "staging", engine.cache["environment"].Value)
	assert.Equal(t, "staging-region", engine.cache["region"].Value)
	assert.Equal(t, "postgres://staging:staging-region/app", engine.cache["database_url"].Value)
	assert.Equal(t, "redis://postgres://staging:staging-region/app/0", engine.cache["redis_url"].Value)

	// Test 7: Change top-level item (redis_url) directly - should only affect itself
	configValues = types.AppConfigValues{
		"redis_url": {Value: "redis://custom-url/0"},
	}
	err = engine.Parse("{{repl ConfigOption \"redis_url\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(configValues)
	require.NoError(t, err)
	assert.Equal(t, "redis://custom-url/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Only redis_url should have user value, others should remain from config
	assert.Equal(t, "redis://custom-url/0", engine.cache["redis_url"].Value)
	assert.Equal(t, "staging", engine.cache["environment"].Value)                                // unchanged
	assert.Equal(t, "staging-region", engine.cache["region"].Value)                              // unchanged
	assert.Equal(t, "postgres://staging:staging-region/app", engine.cache["database_url"].Value) // unchanged

	// Test 8: Remove redis_url user value (go back to config value) - should invalidate
	result, err = engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://staging:staging-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Should be back to original config values
	assert.Equal(t, "staging", engine.cache["environment"].Value)
	assert.Equal(t, "staging-region", engine.cache["region"].Value)
	assert.Equal(t, "postgres://staging:staging-region/app", engine.cache["database_url"].Value)
	assert.Equal(t, "redis://postgres://staging:staging-region/app/0", engine.cache["redis_url"].Value)

	// Test 9: Change middle item (region) - should invalidate dependents but not dependencies
	configValues = types.AppConfigValues{
		"region": {Value: "custom-region"},
	}
	result, err = engine.Execute(configValues)
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://staging:custom-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Verify dependencies vs dependents
	assert.Equal(t, "staging", engine.cache["environment"].Value)                                      // unchanged (dependency)
	assert.Equal(t, "custom-region", engine.cache["region"].Value)                                     // changed (middle item)
	assert.Equal(t, "postgres://staging:custom-region/app", engine.cache["database_url"].Value)        // changed (dependent)
	assert.Equal(t, "redis://postgres://staging:custom-region/app/0", engine.cache["redis_url"].Value) // changed (dependent)

	// Test 10: Reset to no user values, then change middle item (database_url) directly - should only affect itself and dependents
	result, err = engine.Execute(nil)
	require.NoError(t, err)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	configValues = types.AppConfigValues{
		"database_url": {Value: "postgres://direct-override/app"},
	}
	result, err = engine.Execute(configValues)
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://direct-override/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Verify only database_url and its dependents changed
	assert.Equal(t, "staging", engine.cache["environment"].Value)                                // unchanged (dependency)
	assert.Equal(t, "staging-region", engine.cache["region"].Value)                              // unchanged (dependency) - back to config value
	assert.Equal(t, "postgres://direct-override/app", engine.cache["database_url"].Value)        // changed (directly)
	assert.Equal(t, "redis://postgres://direct-override/app/0", engine.cache["redis_url"].Value) // changed (dependent)
}
