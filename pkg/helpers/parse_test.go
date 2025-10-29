package helpers

import (
	"os"
	"path/filepath"
	"testing"

	embeddedclusterv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseEndUserConfig(t *testing.T) {
	tests := []struct {
		name        string
		fpath       string
		fileContent string
		expected    *embeddedclusterv1beta1.Config
		wantErr     string
	}{
		{
			name:     "empty file path returns nil",
			fpath:    "",
			expected: nil,
		},
		{
			name:    "file does not exist",
			fpath:   "nonexistent.yaml",
			wantErr: "failed to read overrides file",
		},
		{
			name:  "invalid YAML",
			fpath: "invalid.yaml",
			fileContent: `invalid: yaml: content: [
			unclosed bracket`,
			wantErr: "failed to unmarshal overrides file",
		},
		{
			name:  "valid embedded cluster config",
			fpath: "valid-config.yaml",
			fileContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
metadata:
  name: test-config
spec:
  version: "1.0.0"
  roles:
    controller:
      name: "controller"`,
			expected: &embeddedclusterv1beta1.Config{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "embeddedcluster.replicated.com/v1beta1",
					Kind:       "Config",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Spec: embeddedclusterv1beta1.ConfigSpec{
					Version: "1.0.0",
					Roles: embeddedclusterv1beta1.Roles{
						Controller: embeddedclusterv1beta1.NodeRole{
							Name: "controller",
						},
					},
				},
			},
		},
		{
			name:  "minimal valid config",
			fpath: "minimal.yaml",
			fileContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config`,
			expected: &embeddedclusterv1beta1.Config{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "embeddedcluster.replicated.com/v1beta1",
					Kind:       "Config",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			var testFile string
			if tt.fpath != "" && tt.fileContent != "" {
				// Create temporary file
				tmpDir := t.TempDir()
				testFile = filepath.Join(tmpDir, tt.fpath)
				err := os.WriteFile(testFile, []byte(tt.fileContent), 0644)
				req.NoError(err)
			} else if tt.fpath != "" {
				// Use the fpath as-is for non-existent file tests
				testFile = tt.fpath
			}

			result, err := ParseEndUserConfig(testFile)

			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
				req.Nil(result)
			} else {
				req.NoError(err)
				req.Equal(tt.expected, result)
			}
		})
	}
}

func TestParseLicense_V1Beta1(t *testing.T) {
	licenseFile := "testdata/license-v1beta1.yaml"

	wrapper, err := ParseLicense(licenseFile)
	require.NoError(t, err)
	require.True(t, wrapper.IsV1())
	require.False(t, wrapper.IsV2())

	assert.Equal(t, "embedded-cluster-test", wrapper.GetAppSlug())
	assert.Equal(t, "test-license-id-v1", wrapper.GetLicenseID())
	assert.True(t, wrapper.IsEmbeddedClusterDownloadEnabled())
	assert.Equal(t, "Test Customer V1", wrapper.GetCustomerName())
}

func TestParseLicense_V1Beta2(t *testing.T) {
	licenseFile := "testdata/license-v1beta2.yaml"

	wrapper, err := ParseLicense(licenseFile)
	require.NoError(t, err)
	require.False(t, wrapper.IsV1())
	require.True(t, wrapper.IsV2())

	assert.Equal(t, "embedded-cluster-test", wrapper.GetAppSlug())
	assert.Equal(t, "test-license-id-v2", wrapper.GetLicenseID())
	assert.True(t, wrapper.IsEmbeddedClusterDownloadEnabled())
	assert.Equal(t, "Test Customer V2", wrapper.GetCustomerName())
}

func TestParseLicense_InvalidVersion(t *testing.T) {
	licenseFile := "testdata/license-invalid-version.yaml"

	_, err := ParseLicense(licenseFile)
	require.Error(t, err)
}

func TestParseLicense_FileNotFound(t *testing.T) {
	licenseFile := "testdata/nonexistent.yaml"

	_, err := ParseLicense(licenseFile)
	require.Error(t, err)
}

func TestParseLicenseFromBytes_V1Beta1(t *testing.T) {
	data, err := os.ReadFile("testdata/license-v1beta1.yaml")
	require.NoError(t, err)

	wrapper, err := ParseLicenseFromBytes(data)
	require.NoError(t, err)
	require.True(t, wrapper.IsV1())
	require.False(t, wrapper.IsV2())

	assert.Equal(t, "embedded-cluster-test", wrapper.GetAppSlug())
	assert.Equal(t, "test-license-id-v1", wrapper.GetLicenseID())
	assert.True(t, wrapper.IsEmbeddedClusterDownloadEnabled())
}

func TestParseLicenseFromBytes_V1Beta2(t *testing.T) {
	data, err := os.ReadFile("testdata/license-v1beta2.yaml")
	require.NoError(t, err)

	wrapper, err := ParseLicenseFromBytes(data)
	require.NoError(t, err)
	require.False(t, wrapper.IsV1())
	require.True(t, wrapper.IsV2())

	assert.Equal(t, "embedded-cluster-test", wrapper.GetAppSlug())
	assert.Equal(t, "test-license-id-v2", wrapper.GetLicenseID())
	assert.True(t, wrapper.IsEmbeddedClusterDownloadEnabled())
}

func TestParseLicenseFromBytes_InvalidVersion(t *testing.T) {
	data := []byte(`apiVersion: kots.io/v1beta3
kind: License`)

	_, err := ParseLicenseFromBytes(data)
	require.Error(t, err)
}

func TestParseLicenseFromBytes_InvalidYAML(t *testing.T) {
	data := []byte(`this is not valid yaml: [[[`)

	_, err := ParseLicenseFromBytes(data)
	require.Error(t, err)
}

func TestParseConfigValues(t *testing.T) {
	tests := []struct {
		name        string
		fpath       string
		fileContent string
		expected    *kotsv1beta1.ConfigValues
		wantErr     string
	}{
		{
			name:     "empty file path returns nil",
			fpath:    "",
			expected: nil,
		},
		{
			name:    "file does not exist",
			fpath:   "nonexistent.yaml",
			wantErr: "failed to read config values file",
		},
		{
			name:  "invalid YAML",
			fpath: "invalid.yaml",
			fileContent: `invalid: yaml: content: [
			unclosed bracket`,
			wantErr: "failed to unmarshal config values file",
		},
		{
			name:  "valid config values",
			fpath: "config-values.yaml",
			fileContent: `apiVersion: kots.io/v1beta1
kind: ConfigValues
metadata:
  name: test-config
spec:
  values:
    database_host:
      default: "localhost"
      value: "postgres.example.com"
    api_port:
      default: "8080"
      value: "3000"`,
			expected: &kotsv1beta1.ConfigValues{
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
							Default: "localhost",
							Value:   "postgres.example.com",
						},
						"api_port": {
							Default: "8080",
							Value:   "3000",
						},
					},
				},
			},
		},
		{
			name:  "empty config values",
			fpath: "empty-config.yaml",
			fileContent: `apiVersion: kots.io/v1beta1
kind: ConfigValues
spec:
  values: {}`,
			expected: &kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{},
				},
			},
		},
		{
			name:  "config values with all fields",
			fpath: "full-config.yaml",
			fileContent: `apiVersion: kots.io/v1beta1
kind: ConfigValues
spec:
  values:
    comprehensive_item:
      default: "default_val"
      value: "actual_val"
      data: "data_content"
      valuePlaintext: "plain_val"
      dataPlaintext: "plain_data"
      filename: "config.txt"
      repeatableItem: "repeat_val"`,
			expected: &kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"comprehensive_item": {
							Default:        "default_val",
							Value:          "actual_val",
							Data:           "data_content",
							ValuePlaintext: "plain_val",
							DataPlaintext:  "plain_data",
							Filename:       "config.txt",
							RepeatableItem: "repeat_val",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			var testFile string
			if tt.fpath != "" && tt.fileContent != "" {
				// Create temporary file
				tmpDir := t.TempDir()
				testFile = filepath.Join(tmpDir, tt.fpath)
				err := os.WriteFile(testFile, []byte(tt.fileContent), 0644)
				req.NoError(err)
			} else if tt.fpath != "" {
				// Use the fpath as-is for non-existent file tests
				testFile = tt.fpath
			}

			result, err := ParseConfigValues(testFile)

			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
				req.Nil(result)
			} else {
				req.NoError(err)
				req.Equal(tt.expected, result)
			}
		})
	}
}

func TestParseConfigValuesFromString(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		expected    *kotsv1beta1.ConfigValues
		wantErr     string
	}{
		{
			name:        "empty string",
			yamlContent: "",
			expected: &kotsv1beta1.ConfigValues{
				TypeMeta:   metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: nil,
				},
			},
		},
		{
			name: "invalid YAML",
			yamlContent: `invalid: yaml: content: [
			unclosed bracket`,
			wantErr: "failed to unmarshal config values YAML",
		},
		{
			name: "valid config values YAML",
			yamlContent: `apiVersion: kots.io/v1beta1
kind: ConfigValues
metadata:
  name: test-config
spec:
  values:
    database_url:
      default: "postgres://localhost/db"
      value: "postgres://prod.example.com/myapp"
    debug_mode:
      default: "false"
      value: "true"`,
			expected: &kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"database_url": {
							Default: "postgres://localhost/db",
							Value:   "postgres://prod.example.com/myapp",
						},
						"debug_mode": {
							Default: "false",
							Value:   "true",
						},
					},
				},
			},
		},
		{
			name: "minimal config values",
			yamlContent: `apiVersion: kots.io/v1beta1
kind: ConfigValues
spec:
  values: {}`,
			expected: &kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{},
				},
			},
		},
		{
			name: "config values with complex data",
			yamlContent: `apiVersion: kots.io/v1beta1
kind: ConfigValues
spec:
  values:
    tls_json:
      value: '{"cert": "...", "key": "..."}'
    empty_json:
      value: ""
    file_config:
      filename: "app.conf"
      data: "server_name=prod"
      dataPlaintext: "server_name=prod"`,
			expected: &kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"tls_json": {
							Value: `{"cert": "...", "key": "..."}`,
						},
						"empty_json": {
							Value: "",
						},
						"file_config": {
							Filename:      "app.conf",
							Data:          "server_name=prod",
							DataPlaintext: "server_name=prod",
						},
					},
				},
			},
		},
		{
			name: "whitespace only",
			yamlContent: `

			`,
			wantErr: "failed to unmarshal config values YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			result, err := ParseConfigValuesFromString(tt.yamlContent)

			if tt.wantErr != "" {
				req.Error(err)
				req.Contains(err.Error(), tt.wantErr)
				req.Nil(result)
			} else {
				req.NoError(err)
				req.Equal(tt.expected, result)
			}
		})
	}
}
