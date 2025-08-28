package template

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
							Default: multitype.FromString("localhost"),
						},
						{
							Name:    "database_port",
							Default: multitype.FromString("5432"),
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
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "HELLO WORLD", result)

	// Test ConfigOption with default values using repl{{ format
	err = engine.Parse("repl{{ ConfigOption \"database_host\"  }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "localhost", result)

	// Test mixing both delimiter formats in one template
	err = engine.Parse("Host: repl{{ ConfigOption \"database_host\" }} Port: {{repl ConfigOption \"database_port\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
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
							Value:   multitype.FromString("db-internal.company.com"),
							Default: multitype.FromString("localhost"),
						},
						{
							Name:    "database_port",
							Default: multitype.FromString("5432"),
						},
						{
							Name:    "redis_host",
							Default: multitype.FromString("redis.company.com"),
						},
						{
							Name: "metrics_endpoint",
							// No Value and no Default - should return empty string
						},
						{
							Name:    "database_url",
							Default: multitype.FromString("postgres://repl{{ ConfigOption \"database_host\" }}:{{repl ConfigOption \"database_port\" }}/app"),
						},
						{
							Name:    "empty_template_value",
							Value:   multitype.FromString("repl{{ if false }}never_shown{{repl end }}"),
							Default: multitype.FromString("fallback_default"),
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
	result, err := engine.Execute(configValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "postgres://db.example.com:5433/app", result)

	// Test with partial user values (should use config value for database_host, user value for database_port)
	partialConfigValues := types.AppConfigValues{
		"database_port": {Value: "5433"},
	}

	err = engine.Parse("{{repl ConfigOption \"database_url\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(partialConfigValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "postgres://db-internal.company.com:5433/app", result)

	// Test with no user values (should use config value for database_host, default for database_port)
	err = engine.Parse("{{repl ConfigOption \"database_url\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "postgres://db-internal.company.com:5432/app", result)

	// Test item with only default value (redis_host) - should use default
	err = engine.Parse("{{repl ConfigOption \"redis_host\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "redis.company.com", result)

	// Test item with only default value but user override - should use user value
	configValues = types.AppConfigValues{
		"redis_host": {Value: "redis.production.com"},
	}
	err = engine.Parse("{{repl ConfigOption \"redis_host\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(configValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "redis.production.com", result)

	// Test item with no value and no default - should return empty string
	err = engine.Parse("{{repl ConfigOption \"metrics_endpoint\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "", result)

	// Test item with no value and no default but user override - should use user value
	configValues = types.AppConfigValues{
		"metrics_endpoint": {Value: "https://metrics.company.com"},
	}
	err = engine.Parse("{{repl ConfigOption \"metrics_endpoint\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(configValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "https://metrics.company.com", result)

	// Test item with template value that evaluates to empty - should fall back to default
	err = engine.Parse("{{repl ConfigOption \"empty_template_value\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "fallback_default", result)

	// Test with empty user value (should use empty string, not fall back to config value)
	emptyConfigValues := types.AppConfigValues{
		"database_host": {Value: ""}, // Empty user value should be used as-is
		"database_port": {Value: "5433"},
	}
	err = engine.Parse("{{repl ConfigOption \"database_url\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(emptyConfigValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
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
							Value:   multitype.FromString("filesystem"),
							Default: multitype.FromString("local"),
						},
						{
							Name:    "backup_type",
							Default: multitype.FromString("snapshot"),
						},
						{
							Name:    "s3_bucket",
							Default: multitype.FromString("my-app-backups"),
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
	result, err := engine.Execute(configValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "S3 Storage", result)

	// Test with no user value - should use config value "filesystem"
	err = engine.Parse("{{repl if ConfigOptionEquals \"storage_type\" \"filesystem\" }}Filesystem Storage{{repl else }}Other Storage{{repl end }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "Filesystem Storage", result)

	// Test with item that has only default value - should use default "snapshot"
	err = engine.Parse("repl{{ if ConfigOptionEquals \"backup_type\" \"snapshot\" }}Snapshot Backup{{repl else }}Other Backup{{repl end }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "Snapshot Backup", result)

	// Test with an unknown item - an error is returned and false is returned
	err = engine.Parse("{{repl if ConfigOptionEquals \"notfound\" \"filesystem\" }}Filesystem Storage{{repl else }}Other Storage{{repl end }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.Error(t, err)
	assert.Equal(t, "", result)
}

func TestEngine_ConfigOptionNotEquals(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "storage",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "storage_type",
							Value:   multitype.FromString("filesystem"),
							Default: multitype.FromString("local"),
						},
						{
							Name:    "backup_type",
							Default: multitype.FromString("snapshot"),
						},
						{
							Name:    "s3_bucket",
							Default: multitype.FromString("my-app-backups"),
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

	err := engine.Parse("repl{{ if ConfigOptionNotEquals \"storage_type\" \"s3\" }}S3 Storage{{repl else }}Other Storage{{repl end }}")
	require.NoError(t, err)
	result, err := engine.Execute(configValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "Other Storage", result)

	// Test with no user value - should use config value "filesystem"
	err = engine.Parse("{{repl if ConfigOptionNotEquals \"storage_type\" \"filesystem\" }}Filesystem Storage{{repl else }}Other Storage{{repl end }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "Other Storage", result)

	// Test with item that has only default value - should use default "snapshot"
	err = engine.Parse("repl{{ if ConfigOptionNotEquals \"backup_type\" \"snapshot\" }}Snapshot Backup{{repl else }}Other Backup{{repl end }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "Other Backup", result)

	// Test with an unknown item - an error is returned and false is returned
	err = engine.Parse("{{repl if ConfigOptionNotEquals \"notfound\" \"filesystem\" }}Filesystem Storage{{repl else }}Other Storage{{repl end }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.Error(t, err)
	assert.Equal(t, "", result)
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
							Type:    "file",
							Value:   multitype.FromString(configCertEncoded),
							Default: multitype.FromString(defaultCertEncoded),
						},
						{
							Name:    "ca_cert",
							Type:    "file",
							Default: multitype.FromString(defaultCertEncoded),
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
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
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
	result, err = engine.Execute(configValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, userCertContent, result)

	// Test with item that has only default value - should use default
	err = engine.Parse("{{repl ConfigOptionData \"ca_cert\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, defaultCertContent, result)
}

func TestEngine_ConfigOptionFilename(t *testing.T) {
	content := "default content"
	contentEncoded := base64.StdEncoding.EncodeToString([]byte(content))

	userContent := "user content"
	userContentEncoded := base64.StdEncoding.EncodeToString([]byte(userContent))

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "a_file_group",
					Items: []kotsv1beta1.ConfigItem{
						{
							Type:     "file",
							Name:     "a_file",
							Value:    multitype.FromString(contentEncoded),
							Filename: "a_file.txt",
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	err := engine.Parse("{{repl ConfigOptionFilename \"a_file\" }}")
	require.NoError(t, err)

	// Test with no user value - should be empty
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "", result)

	// Test with user value - should be user value
	result, err = engine.Execute(types.AppConfigValues{
		"a_file": {Value: userContentEncoded, Filename: "user_file.txt"},
	}, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "user_file.txt", result)

	// Test with an unknown item - an error is returned and empty string is returned
	err = engine.Parse("{{repl ConfigOptionFilename \"notfound\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.Error(t, err)
	assert.Equal(t, "", result)
}

func TestEngine_ConfigOptionFilenameAndDataAndValue(t *testing.T) {
	content := "default content"
	contentEncoded := base64.StdEncoding.EncodeToString([]byte(content))

	userContent := "user content"
	userContentEncoded := base64.StdEncoding.EncodeToString([]byte(userContent))

	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "a_file_group",
					Items: []kotsv1beta1.ConfigItem{
						{
							Type:     "file",
							Name:     "a_file",
							Value:    multitype.FromString(contentEncoded),
							Filename: "a_file.txt",
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	err := engine.Parse("{{repl ConfigOptionFilename \"a_file\" }} {{repl ConfigOptionData \"a_file\" }}")
	require.NoError(t, err)

	// Test with no user value - should be default value but not use the config's filename
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, " default content", result)

	// Test with user value - should be user value
	result, err = engine.Execute(types.AppConfigValues{
		"a_file": {Value: userContentEncoded, Filename: "user_file.txt"},
	}, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "user_file.txt user content", result)
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
							Default: multitype.FromString("{{repl ConfigOption \"item_b\" }}"),
						},
						{
							Name:    "item_b",
							Default: multitype.FromString("{{repl ConfigOption \"item_a\" }}"),
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	err := engine.Parse("{{repl ConfigOption \"item_a\" }}")
	require.NoError(t, err)
	_, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
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
							Value:   multitype.FromString("staging"),
							Default: multitype.FromString("development"),
						},
						{
							Name:    "aws_region",
							Value:   multitype.FromString("repl{{ if ConfigOptionEquals \"environment\" \"production\" }}us-east-1{{repl else }}us-west-2{{repl end }}"),
							Default: multitype.FromString("us-central-1"),
						},
						{
							Name:    "cluster_name",
							Default: multitype.FromString("{{repl ConfigOption \"environment\" }}-repl{{ ConfigOption \"aws_region\" }}"),
						},
						{
							Name:    "database_host",
							Default: multitype.FromString("{{repl ConfigOption \"cluster_name\" }}.rds.amazonaws.com"),
						},
						{
							Name:    "redis_host",
							Value:   multitype.FromString("{{repl ConfigOption \"cluster_name\" }}.elasticache.amazonaws.com"),
							Default: multitype.FromString("localhost"),
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
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "staging-us-west-2.rds.amazonaws.com", result)

	// Test another item with config value that depends on the chain
	err = engine.Parse("{{repl ConfigOption \"redis_host\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "staging-us-west-2.elasticache.amazonaws.com", result)

	// Test with user override - should change the entire chain
	configValues := types.AppConfigValues{
		"environment": {Value: "production"},
	}
	err = engine.Parse("{{repl ConfigOption \"database_host\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(configValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
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
							Value:   multitype.FromString("db-internal.company.com"),
							Default: multitype.FromString("localhost"),
						},
						{
							Name:    "database_port",
							Default: multitype.FromString("5432"),
						},
						{
							Name:    "database_url",
							Default: multitype.FromString("postgres://repl{{ ConfigOption \"database_host\" }}:{{repl ConfigOption \"database_port\" }}/app"),
						},
						{
							Name:    "database_enabled",
							Value:   multitype.FromString("true"),
							Default: multitype.FromString("false"),
						},
					},
				},
				{
					Name: "storage",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "storage_type",
							Value:   multitype.FromString("filesystem"),
							Default: multitype.FromString("memory"),
						},
						{
							Name:    "s3_bucket",
							Default: multitype.FromString("company-app-backups"),
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
	result, err := engine.Execute(configValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
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
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
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
							Value:   multitype.FromString("db-internal.company.com"),
							Default: multitype.FromString("localhost"),
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
	result1, err := engine.Execute(configValues1, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "Host: db1.production.com", result1)

	configValues2 := types.AppConfigValues{
		"database_host": {Value: "db2.staging.com"},
	}
	result2, err := engine.Execute(configValues2, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "Host: db2.staging.com", result2)

	// Execute with no user values - should use config value
	result3, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
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

	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestEngine_ExecuteWithoutParsing(t *testing.T) {
	engine := NewEngine(nil)
	_, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
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
	_, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config item nonexistent not found")
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
							Value:   multitype.FromString("staging"),
							Default: multitype.FromString("development"),
						},
						{
							Name:    "region",
							Value:   multitype.FromString("{{repl ConfigOption \"environment\" }}-region"),
							Default: multitype.FromString("default-region"),
						},
						{
							Name:    "database_url",
							Default: multitype.FromString("postgres://{{repl ConfigOption \"environment\" }}:{{repl ConfigOption \"region\" }}/app"),
						},
						{
							Name:    "redis_url",
							Value:   multitype.FromString("redis://{{repl ConfigOption \"database_url\" }}/0"),
							Default: multitype.FromString("redis://localhost/0"),
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
	result, err := engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://staging:staging-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Verify cache was populated
	assert.Equal(t, "staging", engine.cache["environment"].Effective)
	assert.Equal(t, "staging-region", engine.cache["region"].Effective)
	assert.Equal(t, "postgres://staging:staging-region/app", engine.cache["database_url"].Effective)
	assert.Equal(t, "redis://postgres://staging:staging-region/app/0", engine.cache["redis_url"].Effective)

	// Test 2: Second execution with no changes - should use cache
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://staging:staging-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Test 3: Change a user value - should invalidate dependent items
	configValues := types.AppConfigValues{
		"environment": {Value: "production"},
	}
	result, err = engine.Execute(configValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://production:production-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Verify that items dependent on 'environment' were recomputed
	assert.Equal(t, "production", engine.cache["environment"].Effective)
	assert.Equal(t, "production-region", engine.cache["region"].Effective)
	assert.Equal(t, "postgres://production:production-region/app", engine.cache["database_url"].Effective)
	assert.Equal(t, "redis://postgres://production:production-region/app/0", engine.cache["redis_url"].Effective)

	// Test 4: Execute again with same user values - should use cache
	result, err = engine.Execute(configValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://production:production-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Test 5: Change user value again - should detect change and invalidate
	configValues = types.AppConfigValues{
		"environment": {Value: "development"},
	}
	result, err = engine.Execute(configValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://development:development-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Verify all dependent items were updated
	assert.Equal(t, "development", engine.cache["environment"].Effective)
	assert.Equal(t, "development-region", engine.cache["region"].Effective)
	assert.Equal(t, "postgres://development:development-region/app", engine.cache["database_url"].Effective)
	assert.Equal(t, "redis://postgres://development:development-region/app/0", engine.cache["redis_url"].Effective)

	// Test 6: Remove user value (go back to config value) - should invalidate
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://staging:staging-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Should be back to original config values
	assert.Equal(t, "staging", engine.cache["environment"].Effective)
	assert.Equal(t, "staging-region", engine.cache["region"].Effective)
	assert.Equal(t, "postgres://staging:staging-region/app", engine.cache["database_url"].Effective)
	assert.Equal(t, "redis://postgres://staging:staging-region/app/0", engine.cache["redis_url"].Effective)

	// Test 7: Change top-level item (redis_url) directly - should only affect itself
	configValues = types.AppConfigValues{
		"redis_url": {Value: "redis://custom-url/0"},
	}
	err = engine.Parse("{{repl ConfigOption \"redis_url\" }}")
	require.NoError(t, err)
	result, err = engine.Execute(configValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "redis://custom-url/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Only redis_url should have user value, others should remain from config
	assert.Equal(t, "redis://custom-url/0", engine.cache["redis_url"].Effective)
	assert.Equal(t, "staging", engine.cache["environment"].Effective)                                // unchanged
	assert.Equal(t, "staging-region", engine.cache["region"].Effective)                              // unchanged
	assert.Equal(t, "postgres://staging:staging-region/app", engine.cache["database_url"].Effective) // unchanged

	// Test 8: Remove redis_url user value (go back to config value) - should invalidate
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://staging:staging-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Should be back to original config values
	assert.Equal(t, "staging", engine.cache["environment"].Effective)
	assert.Equal(t, "staging-region", engine.cache["region"].Effective)
	assert.Equal(t, "postgres://staging:staging-region/app", engine.cache["database_url"].Effective)
	assert.Equal(t, "redis://postgres://staging:staging-region/app/0", engine.cache["redis_url"].Effective)

	// Test 9: Change middle item (region) - should invalidate dependents but not dependencies
	configValues = types.AppConfigValues{
		"region": {Value: "custom-region"},
	}
	result, err = engine.Execute(configValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://staging:custom-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Verify dependencies vs dependents
	assert.Equal(t, "staging", engine.cache["environment"].Effective)                                      // unchanged (dependency)
	assert.Equal(t, "custom-region", engine.cache["region"].Effective)                                     // changed (middle item)
	assert.Equal(t, "postgres://staging:custom-region/app", engine.cache["database_url"].Effective)        // changed (dependent)
	assert.Equal(t, "redis://postgres://staging:custom-region/app/0", engine.cache["redis_url"].Effective) // changed (dependent)

	// Test 10: Reset to no user values, then change middle item (database_url) directly - should only affect itself and dependents
	result, err = engine.Execute(nil, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://staging:staging-region/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	configValues = types.AppConfigValues{
		"database_url": {Value: "postgres://direct-override/app"},
	}
	result, err = engine.Execute(configValues, WithProxySpec(&ecv1beta1.ProxySpec{}))
	require.NoError(t, err)
	assert.Equal(t, "redis://postgres://direct-override/app/0", result)
	assert.Equal(t, expectedDepsTree, engine.depsTree)

	// Verify only database_url and its dependents changed
	assert.Equal(t, "staging", engine.cache["environment"].Effective)                                // unchanged (dependency)
	assert.Equal(t, "staging-region", engine.cache["region"].Effective)                              // unchanged (dependency) - back to config value
	assert.Equal(t, "postgres://direct-override/app", engine.cache["database_url"].Effective)        // changed (directly)
	assert.Equal(t, "redis://postgres://direct-override/app/0", engine.cache["redis_url"].Effective) // changed (dependent)
}

func TestEngine_RecordDependency(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "test",
					Items: []kotsv1beta1.ConfigItem{
						{Name: "item1"},
						{Name: "item2"},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	// Test 1: Record dependency when stack is empty - should not record anything
	engine.recordDependency("dependency1")
	assert.Empty(t, engine.depsTree)

	// Test 2: Record dependency when stack has one item
	engine.stack = []string{"item1"}
	engine.recordDependency("dependency1")
	assert.Equal(t, []string{"dependency1"}, engine.depsTree["item1"])

	// Test 3: Record multiple dependencies for same item
	engine.recordDependency("dependency2")
	assert.ElementsMatch(t, []string{"dependency1", "dependency2"}, engine.depsTree["item1"])

	// Test 4: Record duplicate dependency - should not add duplicates
	engine.recordDependency("dependency1")
	assert.ElementsMatch(t, []string{"dependency1", "dependency2"}, engine.depsTree["item1"])

	// Test 5: Record dependency with different item on stack
	engine.stack = []string{"item2"}
	engine.recordDependency("dependency3")
	assert.Equal(t, []string{"dependency3"}, engine.depsTree["item2"])
	assert.ElementsMatch(t, []string{"dependency1", "dependency2"}, engine.depsTree["item1"]) // item1 unchanged

	// Test 6: Record dependency with nested stack (should use current top)
	engine.stack = []string{"item1", "item2"}
	engine.recordDependency("dependency4")
	assert.ElementsMatch(t, []string{"dependency3", "dependency4"}, engine.depsTree["item2"])
	assert.ElementsMatch(t, []string{"dependency1", "dependency2"}, engine.depsTree["item1"]) // item1 unchanged
}

func TestEngine_ConfigMode_TLSGeneration(t *testing.T) {
	// Helper function to create config values for a hostname
	configValuesFor := func(hostname string) types.AppConfigValues {
		return types.AppConfigValues{
			"ingress_hostname": {Value: hostname},
		}
	}

	// Helper function to time an execution
	timeExecution := func(name string, fn func() (string, error)) (time.Duration, string) {
		start := time.Now()
		result, err := fn()
		duration := time.Since(start)
		require.NoError(t, err, "execution %s failed", name)
		return duration, result
	}

	// Create config with TLS certificate generation templates
	config := &kotsv1beta1.Config{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kots.io/v1beta1",
			Kind:       "Config",
		},
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "tls_settings",
					Title: "TLS Configuration",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:     "ingress_hostname",
							Title:    "Ingress Hostname",
							HelpText: "Enter a DNS hostname to use as the cert's CN.",
							Type:     "text",
						},
						{
							Name:   "tls_json",
							Title:  "TLS JSON",
							Type:   "textarea",
							Hidden: true,
							Default: multitype.FromString(`repl{{ $ca := genCA (ConfigOption "ingress_hostname") 365 }}
repl{{ $tls := dict "ca" $ca }}
repl{{ $cert := genSignedCert (ConfigOption "ingress_hostname") (list ) (list (ConfigOption "ingress_hostname")) 365 $ca }}
repl{{ $_ := set $tls "cert" $cert }}
repl{{ toJson $tls }}`),
						},
						{
							Name:    "tls_ca",
							Title:   "Signing Authority",
							Type:    "textarea",
							Default: multitype.FromString(`repl{{ fromJson (ConfigOption "tls_json") | dig "ca" "Cert" "" }}`),
						},
						{
							Name:    "tls_cert",
							Title:   "TLS Cert",
							Type:    "textarea",
							Default: multitype.FromString(`repl{{ fromJson (ConfigOption "tls_json") | dig "cert" "Cert" "" }}`),
						},
						{
							Name:    "tls_key",
							Title:   "TLS Key",
							Type:    "textarea",
							Default: multitype.FromString(`repl{{ fromJson (ConfigOption "tls_json") | dig "cert" "Key" "" }}`),
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config, WithMode(ModeConfig))

	// Test 1: First execution with hostname - should be slow (certificate generation)
	firstHostname := "example.com"
	firstDuration, firstResult := timeExecution("first", func() (string, error) {
		return engine.Execute(configValuesFor(firstHostname))
	})

	// Verify basic YAML structure
	assert.Contains(t, firstResult, "apiVersion: kots.io/v1beta1")
	assert.Contains(t, firstResult, "kind: Config")

	// Verify TLS config items are present
	expectedTLSItems := []string{"tls_json", "tls_ca", "tls_cert", "tls_key"}
	for _, item := range expectedTLSItems {
		assert.Contains(t, firstResult, fmt.Sprintf("name: %s", item))
	}

	// Test 2: First cached execution - should be fast
	firstCachedDuration, firstCachedResult := timeExecution("first cached", func() (string, error) {
		return engine.Execute(configValuesFor(firstHostname))
	})

	// Verify performance characteristics: non-cached should be in ms, cached much faster
	assert.Greater(t, firstDuration, time.Millisecond*50, "First execution should take at least 50ms (cert generation)")
	assert.Less(t, firstCachedDuration, time.Millisecond*20, "First cached execution should be under 20ms")

	// Verify caching provides significant speedup
	assert.True(t, firstCachedDuration < firstDuration/5,
		"Cached execution should be at least 5x faster. First: %v, Cached: %v",
		firstDuration, firstCachedDuration)

	// Verify cached result is identical to first execution
	assert.Equal(t, firstResult, firstCachedResult, "Cached execution should return identical result")

	// Test 3: Second execution with different hostname - should be slow again (new certificate generation)
	secondHostname := "test.example.com"
	secondDuration, secondResult := timeExecution("second", func() (string, error) {
		return engine.Execute(configValuesFor(secondHostname))
	})

	// Verify different certificates are generated for different hostnames
	assert.NotEqual(t, firstResult, secondResult, "Different hostnames should generate different certificates")

	// Test 4: Second cached execution - should be fast again
	secondCachedDuration, secondCachedResult := timeExecution("second cached", func() (string, error) {
		return engine.Execute(configValuesFor(secondHostname))
	})

	// Verify performance characteristics for second hostname
	assert.Greater(t, secondDuration, time.Millisecond*50, "Second execution should take at least 50ms (cert generation)")
	assert.Less(t, secondCachedDuration, time.Millisecond*20, "Second cached execution should be under 20ms")

	// Verify caching provides significant speedup
	assert.True(t, secondCachedDuration < secondDuration/5,
		"Cached execution should be at least 5x faster. Second: %v, Cached: %v",
		secondDuration, secondCachedDuration)

	// Verify second cached result is identical to second execution
	assert.Equal(t, secondResult, secondCachedResult, "Second cached execution should return identical result")

	// Log performance metrics
	t.Logf("TLS Generation Cache Performance:")
	t.Logf("  First execution (%s): %.1fms", firstHostname, float64(firstDuration)/float64(time.Millisecond))
	t.Logf("  First cached execution (%s): %.1fµs (%.1fx speedup)", firstHostname, float64(firstCachedDuration)/float64(time.Microsecond),
		float64(firstDuration)/float64(firstCachedDuration))
	t.Logf("  Second execution (%s): %.1fms", secondHostname, float64(secondDuration)/float64(time.Millisecond))
	t.Logf("  Second cached execution (%s): %.1fµs (%.1fx speedup)", secondHostname, float64(secondCachedDuration)/float64(time.Microsecond),
		float64(secondDuration)/float64(secondCachedDuration))
}

func TestEngine_ConfigMode_BasicTemplating(t *testing.T) {
	config := &kotsv1beta1.Config{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kots.io/v1beta1",
			Kind:       "Config",
		},
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "database_settings",
					Title: "Database Configuration",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "database_host",
							Title:   "Database Host",
							Type:    "text",
							Default: multitype.FromString("localhost"),
						},
						{
							Name:    "database_port",
							Title:   "Database Port",
							Type:    "text",
							Default: multitype.FromString("5432"),
						},
						{
							Name:    "database_url",
							Title:   "Database URL",
							Type:    "text",
							Default: multitype.FromString("postgres://repl{{ ConfigOption \"database_host\" }}:repl{{ ConfigOption \"database_port\" }}/app"),
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config, WithMode(ModeConfig))

	// Test basic config mode execution
	result, err := engine.Execute(nil)
	require.NoError(t, err)

	expectedYAML := `apiVersion: kots.io/v1beta1
kind: Config
metadata:
  creationTimestamp: null
spec:
  groups:
  - items:
    - default: localhost
      name: database_host
      title: Database Host
      type: text
      value: ""
    - default: "5432"
      name: database_port
      title: Database Port
      type: text
      value: ""
    - default: postgres://localhost:5432/app
      name: database_url
      title: Database URL
      type: text
      value: ""
    name: database_settings
    title: Database Configuration
status: {}
`

	assert.YAMLEq(t, expectedYAML, result)
}

func TestEngine_ConfigMode_ValuePriority(t *testing.T) {
	config := &kotsv1beta1.Config{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kots.io/v1beta1",
			Kind:       "Config",
		},
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "app_settings",
					Title: "Application Settings",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "app_name",
							Title:   "Application Name",
							Type:    "text",
							Value:   multitype.FromString("MyApp"),
							Default: multitype.FromString("DefaultApp"),
						},
						{
							Name:    "app_version",
							Title:   "Version",
							Type:    "text",
							Default: multitype.FromString("1.0.0"),
						},
						{
							Name:    "display_name",
							Title:   "Display Name",
							Type:    "text",
							Default: multitype.FromString("repl{{ ConfigOption \"app_name\" }} v repl{{ ConfigOption \"app_version\" }}"),
						},
						{
							Name:    "app_title",
							Title:   "App Title",
							Type:    "text",
							Value:   multitype.FromString("Application: repl{{ ConfigOption \"app_name\" }}"),
							Default: multitype.FromString("Default Application"),
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config, WithMode(ModeConfig))

	// Test with user values (should override config values)
	configValues := types.AppConfigValues{
		"app_name":    {Value: "CustomApp"},
		"app_version": {Value: "2.0.0"},
	}

	result, err := engine.Execute(configValues)
	require.NoError(t, err)

	expectedYAMLWithUserValues := `apiVersion: kots.io/v1beta1
kind: Config
metadata:
  creationTimestamp: null
spec:
  groups:
  - items:
    - default: DefaultApp
      name: app_name
      title: Application Name
      type: text
      value: CustomApp
    - default: 1.0.0
      name: app_version
      title: Version
      type: text
      value: "2.0.0"
    - default: CustomApp v 2.0.0
      name: display_name
      title: Display Name
      type: text
      value: ""
    - default: Default Application
      name: app_title
      title: App Title
      type: text
      value: "Application: CustomApp"
    name: app_settings
    title: Application Settings
status: {}
`

	assert.YAMLEq(t, expectedYAMLWithUserValues, result)

	// Test without user values (should use config values and defaults)
	result2, err := engine.Execute(nil)
	require.NoError(t, err)

	expectedYAMLWithoutUserValues := `apiVersion: kots.io/v1beta1
kind: Config
metadata:
  creationTimestamp: null
spec:
  groups:
  - items:
    - default: DefaultApp
      name: app_name
      title: Application Name
      type: text
      value: MyApp
    - default: 1.0.0
      name: app_version
      title: Version
      type: text
      value: ""
    - default: MyApp v 1.0.0
      name: display_name
      title: Display Name
      type: text
      value: ""
    - default: Default Application
      name: app_title
      title: App Title
      type: text
      value: "Application: MyApp"
    name: app_settings
    title: Application Settings
status: {}
`

	assert.YAMLEq(t, expectedYAMLWithoutUserValues, result2)
}

func TestEngine_ConfigMode_CircularDependency(t *testing.T) {
	config := &kotsv1beta1.Config{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kots.io/v1beta1",
			Kind:       "Config",
		},
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "circular_test",
					Title: "Circular Dependency Test",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "item_a",
							Title:   "Item A",
							Type:    "text",
							Default: multitype.FromString("repl{{ ConfigOption \"item_b\" }}"),
						},
						{
							Name:    "item_b",
							Title:   "Item B",
							Type:    "text",
							Default: multitype.FromString("repl{{ ConfigOption \"item_a\" }}"),
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config, WithMode(ModeConfig))

	// Should detect circular dependency and return error
	_, err := engine.Execute(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency detected")
}

func TestEngine_ConfigMode_ComplexDependencyChain(t *testing.T) {
	config := &kotsv1beta1.Config{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kots.io/v1beta1",
			Kind:       "Config",
		},
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "complex_settings",
					Title: "Complex Dependency Chain",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "base_url",
							Title:   "Base URL",
							Type:    "text",
							Default: multitype.FromString("https://api.example.com"),
						},
						{
							Name:    "api_version",
							Title:   "API Version",
							Type:    "text",
							Default: multitype.FromString("v1"),
						},
						{
							Name:    "api_endpoint",
							Title:   "API Endpoint",
							Type:    "text",
							Default: multitype.FromString("repl{{ ConfigOption \"base_url\" }}/repl{{ ConfigOption \"api_version\" }}"),
						},
						{
							Name:    "full_config",
							Title:   "Full Configuration",
							Type:    "textarea",
							Default: multitype.FromString("endpoint: repl{{ ConfigOption \"api_endpoint\" }}\nversion: repl{{ ConfigOption \"api_version\" }}"),
						},
						{
							Name:    "service_url",
							Title:   "Service URL",
							Type:    "text",
							Value:   multitype.FromString("repl{{ ConfigOption \"base_url\" }}/service"),
							Default: multitype.FromString("http://localhost/service"),
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config, WithMode(ModeConfig))

	// Test with user values
	configValues := types.AppConfigValues{
		"base_url":    {Value: "https://custom.api.com"},
		"api_version": {Value: "v2"},
	}

	result, err := engine.Execute(configValues)
	require.NoError(t, err)

	expectedYAML := `apiVersion: kots.io/v1beta1
kind: Config
metadata:
  creationTimestamp: null
spec:
  groups:
  - items:
    - default: https://api.example.com
      name: base_url
      title: Base URL
      type: text
      value: "https://custom.api.com"
    - default: v1
      name: api_version
      title: API Version
      type: text
      value: "v2"
    - default: https://custom.api.com/v2
      name: api_endpoint
      title: API Endpoint
      type: text
      value: ""
    - default: |-
        endpoint: https://custom.api.com/v2
        version: v2
      name: full_config
      title: Full Configuration
      type: textarea
      value: ""
    - default: http://localhost/service
      name: service_url
      title: Service URL
      type: text
      value: "https://custom.api.com/service"
    name: complex_settings
    title: Complex Dependency Chain
status: {}
`

	assert.YAMLEq(t, expectedYAML, result)
}
