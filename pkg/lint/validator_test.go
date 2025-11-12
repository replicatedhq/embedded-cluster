package lint

import (
	"os"
	"path/filepath"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePorts(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		expectWarning bool
		warningCount  int
		warningMsg    string
	}{
		{
			name: "valid port outside supported range",
			yamlContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  unsupportedOverrides:
    builtInExtensions:
    - name: adminconsole
      values: |
        service:
          nodePort: 50000`,
			expectWarning: false,
		},
		{
			name: "port within supported range - should warn",
			yamlContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  unsupportedOverrides:
    builtInExtensions:
    - name: adminconsole
      values: |
        service:
          nodePort: 8080`,
			expectWarning: true,
			warningCount:  1,
			warningMsg:    "port 8080 is already supported",
		},
		{
			name: "multiple ports mixed valid and invalid",
			yamlContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  unsupportedOverrides:
    builtInExtensions:
    - name: openebs
      values: |
        apiServer:
          service:
            nodePort: 5000
    - name: adminconsole
      values: |
        service:
          nodePort: 50000`,
			expectWarning: true,
			warningCount:  1,
			warningMsg:    "port 5000 is already supported",
		},
		{
			name: "port at boundary - 80",
			yamlContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  unsupportedOverrides:
    builtInExtensions:
    - name: adminconsole
      values: |
        service:
          nodePort: 80`,
			expectWarning: true,
			warningCount:  1,
			warningMsg:    "port 80 is already supported",
		},
		{
			name: "port at boundary - 32767",
			yamlContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  unsupportedOverrides:
    builtInExtensions:
    - name: adminconsole
      values: |
        service:
          nodePort: 32767`,
			expectWarning: true,
			warningCount:  1,
			warningMsg:    "port 32767 is already supported",
		},
		{
			name: "port outside boundary - 79",
			yamlContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  unsupportedOverrides:
    builtInExtensions:
    - name: adminconsole
      values: |
        service:
          nodePort: 79`,
			expectWarning: false,
		},
		{
			name: "port outside boundary - 32768",
			yamlContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  unsupportedOverrides:
    builtInExtensions:
    - name: adminconsole
      values: |
        service:
          nodePort: 32768`,
			expectWarning: false,
		},
		{
			name: "nested nodePort in complex structure",
			yamlContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  unsupportedOverrides:
    builtInExtensions:
    - name: monitoring
      values: |
        prometheus:
          server:
            service:
              nodePort: 30090
        grafana:
          service:
            nodePort: 30091`,
			expectWarning: true,
			warningCount:  2,
			warningMsg:    "already supported",
		},
		{
			name: "empty values",
			yamlContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  unsupportedOverrides:
    builtInExtensions:
    - name: adminconsole
      values: ""`,
			expectWarning: false,
		},
		{
			name: "no unsupportedOverrides",
			yamlContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec: {}`,
			expectWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "config.yaml")
			err := os.WriteFile(tmpFile, []byte(tt.yamlContent), 0644)
			require.NoError(t, err)

			// Create validator without API client
			validator := NewValidator("", "", "")

			// Validate the file
			result, err := validator.ValidateFile(tmpFile)
			require.NoError(t, err)

			if tt.expectWarning {
				assert.Len(t, result.Warnings, tt.warningCount, "Expected %d warnings but got %d", tt.warningCount, len(result.Warnings))
				if tt.warningMsg != "" && len(result.Warnings) > 0 {
					assert.Contains(t, result.Warnings[0].String(), tt.warningMsg)
				}
			} else {
				assert.Empty(t, result.Warnings, "Expected no warnings but got: %v", result.Warnings)
			}
			// Port validation should never produce errors, only warnings
			assert.Empty(t, result.Errors, "Port validation should not produce errors")
		})
	}
}

func TestExtractNodePorts(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		expected []int
	}{
		{
			name: "simple nodePort",
			data: map[interface{}]interface{}{
				"nodePort": 8080,
			},
			expected: []int{8080},
		},
		{
			name: "nested nodePort",
			data: map[string]interface{}{
				"service": map[string]interface{}{
					"type":     "NodePort",
					"nodePort": 30000,
				},
			},
			expected: []int{30000},
		},
		{
			name: "multiple nodePorts",
			data: map[string]interface{}{
				"service1": map[string]interface{}{
					"nodePort": 30001,
				},
				"service2": map[string]interface{}{
					"nodePort": 30002,
				},
			},
			expected: []int{30001, 30002},
		},
		{
			name: "nodePort as string",
			data: map[string]interface{}{
				"nodePort": "8080",
			},
			expected: []int{8080},
		},
		{
			name: "nodePort as float",
			data: map[string]interface{}{
				"nodePort": float64(8080),
			},
			expected: []int{8080},
		},
		{
			name: "invalid nodePort string",
			data: map[string]interface{}{
				"nodePort": "not-a-number",
			},
			expected: []int{},
		},
		{
			name: "array with nodePorts",
			data: []interface{}{
				map[string]interface{}{
					"nodePort": 30001,
				},
				map[string]interface{}{
					"nodePort": 30002,
				},
			},
			expected: []int{30001, 30002},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator("", "", "")
			portInfos := validator.extractNodePorts(tt.data, []string{})

			var ports []int
			for _, pi := range portInfos {
				ports = append(ports, pi.port)
			}

			assert.ElementsMatch(t, tt.expected, ports, "Expected ports %v but got %v", tt.expected, ports)
		})
	}
}

func TestExtractPortValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected int
	}{
		{
			name:     "int value",
			value:    8080,
			expected: 8080,
		},
		{
			name:     "float64 value",
			value:    float64(8080),
			expected: 8080,
		},
		{
			name:     "string value",
			value:    "8080",
			expected: 8080,
		},
		{
			name:     "invalid string",
			value:    "not-a-port",
			expected: 0,
		},
		{
			name:     "nil value",
			value:    nil,
			expected: 0,
		},
		{
			name:     "boolean value",
			value:    true,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator("", "", "")
			result := validator.extractPortValue(tt.value)
			assert.Equal(t, tt.expected, result, "Expected %d but got %d", tt.expected, result)
		})
	}
}

func TestValidateYAMLSyntax(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid yaml",
			yamlContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  version: "1.0.0"`,
			expectError: false,
		},
		{
			name: "valid multi-document yaml",
			yamlContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  version: "1.0.0"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test`,
			expectError: false,
		},
		{
			name: "duplicate key - strict mode",
			yamlContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  version: "1.0.0"
  version: "2.0.0"`,
			expectError: true,
			errorMsg:    "line 5",
		},
		{
			name:        "tab character mixed with spaces",
			yamlContent: "apiVersion: embeddedcluster.replicated.com/v1beta1\nkind: Config\nspec:\n\tversion: \"1.0.0\"",
			expectError: true,
			errorMsg:    "line",
		},
		{
			name: "unclosed quote",
			yamlContent: `apiVersion: embeddedcluster.replicated.com/v1beta1
kind: Config
spec:
  version: "1.0.0`,
			expectError: true,
			errorMsg:    "line",
		},
		{
			name:        "empty yaml",
			yamlContent: ``,
			expectError: false,
		},
		{
			name: "only comments",
			yamlContent: `# This is a comment
# Another comment`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator("", "", "")
			err := validator.validateYAMLSyntax([]byte(tt.yamlContent))

			if tt.expectError {
				require.Error(t, err, "Expected YAML syntax error but got none")
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg, "Error message should contain: %s", tt.errorMsg)
				}
			} else {
				assert.NoError(t, err, "Expected no error but got: %v", err)
			}
		})
	}
}

func TestExtractLineNumber(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected int
	}{
		{
			name:     "yaml error with line number",
			errMsg:   "yaml: line 15: mapping values are not allowed in this context",
			expected: 15,
		},
		{
			name:     "yaml error with different line",
			errMsg:   "yaml: line 42: found unexpected end of stream",
			expected: 42,
		},
		{
			name:     "error without line number",
			errMsg:   "yaml: invalid yaml",
			expected: 0,
		},
		{
			name:     "empty error message",
			errMsg:   "",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLineNumber(tt.errMsg)
			assert.Equal(t, tt.expected, result, "Expected line %d but got %d", tt.expected, result)
		})
	}
}

func TestValidateDomains(t *testing.T) {
	tests := []struct {
		name           string
		configDomains  ecv1beta1.Domains
		allowedDomains []string
		expectErrors   int
		errorFields    []string
	}{
		{
			name: "valid custom domain",
			configDomains: ecv1beta1.Domains{
				ReplicatedAppDomain: "custom.example.com",
			},
			allowedDomains: []string{"custom.example.com"},
			expectErrors:   0,
		},
		{
			name: "invalid custom domain",
			configDomains: ecv1beta1.Domains{
				ReplicatedAppDomain: "invalid.example.com",
			},
			allowedDomains: []string{"custom.example.com"},
			expectErrors:   1,
			errorFields:    []string{"domains.replicatedAppDomain"},
		},
		{
			name: "default domains always allowed",
			configDomains: ecv1beta1.Domains{
				ReplicatedAppDomain:      "replicated.app",
				ProxyRegistryDomain:      "proxy.replicated.com",
				ReplicatedRegistryDomain: "registry.replicated.com",
			},
			allowedDomains: []string{}, // Empty list, but defaults should still be allowed
			expectErrors:   0,
		},
		{
			name: "multiple invalid domains",
			configDomains: ecv1beta1.Domains{
				ReplicatedAppDomain:      "bad1.example.com",
				ProxyRegistryDomain:      "bad2.example.com",
				ReplicatedRegistryDomain: "bad3.example.com",
			},
			allowedDomains: []string{},
			expectErrors:   3,
			errorFields:    []string{"domains.replicatedAppDomain", "domains.proxyRegistryDomain", "domains.replicatedRegistryDomain"},
		},
		{
			name: "mixed valid and invalid domains",
			configDomains: ecv1beta1.Domains{
				ReplicatedAppDomain:      "custom.example.com",
				ProxyRegistryDomain:      "invalid.example.com",
				ReplicatedRegistryDomain: "registry.replicated.com", // Default
			},
			allowedDomains: []string{"custom.example.com"},
			expectErrors:   1,
			errorFields:    []string{"domains.proxyRegistryDomain"},
		},
		{
			name: "empty domain configuration",
			configDomains: ecv1beta1.Domains{
				ReplicatedAppDomain:      "",
				ProxyRegistryDomain:      "",
				ReplicatedRegistryDomain: "",
			},
			allowedDomains: []string{"any.example.com"},
			expectErrors:   0, // No domains to validate
		},
		{
			name: "custom domain in allowed list",
			configDomains: ecv1beta1.Domains{
				ReplicatedAppDomain: "app.custom.io",
				ProxyRegistryDomain: "proxy.custom.io",
			},
			allowedDomains: []string{"app.custom.io", "proxy.custom.io", "registry.custom.io"},
			expectErrors:   0,
		},
		{
			name: "one valid, two invalid",
			configDomains: ecv1beta1.Domains{
				ReplicatedAppDomain:      "replicated.app", // Default - valid
				ProxyRegistryDomain:      "bad.com",
				ReplicatedRegistryDomain: "alsobad.com",
			},
			allowedDomains: []string{},
			expectErrors:   2,
			errorFields:    []string{"domains.proxyRegistryDomain", "domains.replicatedRegistryDomain"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator("", "", "")

			errors := validator.validateDomains(tt.configDomains, tt.allowedDomains)

			assert.Len(t, errors, tt.expectErrors, "Expected %d errors but got %d", tt.expectErrors, len(errors))

			// Check that expected error fields are present
			if tt.expectErrors > 0 && len(tt.errorFields) > 0 {
				for i, expectedField := range tt.errorFields {
					if i < len(errors) {
						if ve, ok := errors[i].(ValidationError); ok {
							assert.Equal(t, expectedField, ve.Field, "Error field mismatch at index %d", i)
						}
					}
				}
			}
		})
	}
}

func TestValidateVeleroPlugins(t *testing.T) {
	tests := []struct {
		name        string
		veleroExt   ecv1beta1.VeleroExtensions
		expectError bool
		errorCount  int
		errorFields []string
		errorMsgs   []string
	}{
		{
			name: "valid single plugin",
			veleroExt: ecv1beta1.VeleroExtensions{
				Plugins: []ecv1beta1.VeleroPlugin{
					{Name: "velero-postgresql", Image: "myvendor/velero-postgresql:v1.0.0"},
				},
			},
			expectError: false,
		},
		{
			name: "valid multiple plugins",
			veleroExt: ecv1beta1.VeleroExtensions{
				Plugins: []ecv1beta1.VeleroPlugin{
					{Name: "velero-postgresql", Image: "myvendor/velero-postgresql:v1.0.0"},
					{Name: "velero-mongodb", Image: "myvendor/velero-mongodb:v2.1.0"},
				},
			},
			expectError: false,
		},
		{
			name: "empty name - should error",
			veleroExt: ecv1beta1.VeleroExtensions{
				Plugins: []ecv1beta1.VeleroPlugin{
					{Name: "", Image: "myvendor/velero-postgresql:v1.0.0"},
				},
			},
			expectError: true,
			errorCount:  1,
			errorFields: []string{"extensions.velero.plugins[0].name"},
			errorMsgs:   []string{"plugin name is required"},
		},
		{
			name: "empty image - should error",
			veleroExt: ecv1beta1.VeleroExtensions{
				Plugins: []ecv1beta1.VeleroPlugin{
					{Name: "velero-postgresql", Image: ""},
				},
			},
			expectError: true,
			errorCount:  1,
			errorFields: []string{"extensions.velero.plugins[0].image"},
			errorMsgs:   []string{"plugin image is required"},
		},
		{
			name: "duplicate plugin names - should error",
			veleroExt: ecv1beta1.VeleroExtensions{
				Plugins: []ecv1beta1.VeleroPlugin{
					{Name: "velero-postgresql", Image: "myvendor/velero-postgresql:v1.0.0"},
					{Name: "velero-postgresql", Image: "myvendor/velero-postgresql:v2.0.0"},
				},
			},
			expectError: true,
			errorCount:  1,
			errorFields: []string{"extensions.velero.plugins[1].name"},
			errorMsgs:   []string{"duplicate plugin name"},
		},
		{
			name: "duplicate plugin images - should error",
			veleroExt: ecv1beta1.VeleroExtensions{
				Plugins: []ecv1beta1.VeleroPlugin{
					{Name: "velero-postgresql", Image: "myvendor/velero-postgresql:v1.0.0"},
					{Name: "velero-postgresql-alt", Image: "myvendor/velero-postgresql:v1.0.0"},
				},
			},
			expectError: true,
			errorCount:  1,
			errorFields: []string{"extensions.velero.plugins[1].image"},
			errorMsgs:   []string{"duplicate plugin image"},
		},
		{
			name: "image with invalid characters - should error",
			veleroExt: ecv1beta1.VeleroExtensions{
				Plugins: []ecv1beta1.VeleroPlugin{
					{Name: "velero-postgresql", Image: "myvendor/velero postgresql:v1.0.0"},
				},
			},
			expectError: true,
			errorCount:  1,
			errorFields: []string{"extensions.velero.plugins[0].image"},
			errorMsgs:   []string{"invalid repository"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator("", "", "")
			errors := validator.validateVeleroPlugins(tt.veleroExt)

			if tt.expectError {
				require.Greater(t, len(errors), 0, "Expected errors but got none")
				if tt.errorCount > 0 {
					assert.Len(t, errors, tt.errorCount, "Expected %d errors but got %d", tt.errorCount, len(errors))
				}
				if len(tt.errorFields) > 0 {
					for i, expectedField := range tt.errorFields {
						if i < len(errors) {
							if ve, ok := errors[i].(ValidationError); ok {
								assert.Equal(t, expectedField, ve.Field, "Error field mismatch at index %d", i)
							}
						}
					}
				}
				if len(tt.errorMsgs) > 0 {
					for i, expectedMsg := range tt.errorMsgs {
						if i < len(errors) {
							assert.Contains(t, errors[i].Error(), expectedMsg, "Error message should contain: %s", expectedMsg)
						}
					}
				}
			} else {
				assert.Empty(t, errors, "Expected no errors but got: %v", errors)
			}
		})
	}
}

func TestValidateImageFormat(t *testing.T) {
	tests := []struct {
		name        string
		image       string
		expectError bool
		errorMsg    string
	}{
		{
			name:  "valid full image reference",
			image: "myvendor/velero-postgresql:v1.0.0",
		},
		{
			name:  "valid full image reference with registry, tag and digest",
			image: "registry.io:5000/repo/image:v1.0.0@sha256:3b9d51de8dab574f77f29c55119b7bb6943c9439c99e9945d76ea322ff5a192a",
		},
		{
			name:  "valid image with digest",
			image: "myvendor/velero-postgresql@sha256:3b9d51de8dab574f77f29c55119b7bb6943c9439c99e9945d76ea322ff5a192a",
		},
		{
			name:  "valid short image name with tag",
			image: "velero-plugin-postgres:v1.0.0",
		},
		{
			name:  "valid short image name",
			image: "velero-plugin-postgres",
		},
		{
			name:        "empty image",
			image:       "",
			expectError: true,
			errorMsg:    "cannot be empty",
		},
		{
			name:        "image with space",
			image:       "myvendor/velero postgresql:v1.0.0",
			expectError: true,
			errorMsg:    "invalid image reference",
		},
		{
			name:        "image starting with slash",
			image:       "/myvendor/velero-postgresql:v1.0.0",
			expectError: true,
			errorMsg:    "invalid image reference",
		},
		{
			name:  "image with registry but no tag",
			image: "myvendor/velero-postgresql",
		},
		{
			name:        "invalid digest",
			image:       "registry.io:5000/repo/image@sha256:3b9d51d",
			expectError: true,
			errorMsg:    "invalid image reference",
		},
		{
			name:  "registry with port and tag - should pass",
			image: "registry.io:5000/repo/image:v1.0.0",
		},
		{
			name:  "registry with port and digest - should pass",
			image: "registry.io:5000/repo/image@sha256:3b9d51de8dab574f77f29c55119b7bb6943c9439c99e9945d76ea322ff5a192a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator("", "", "")
			err := validator.validateImageFormat(tt.image)

			if tt.expectError {
				require.Error(t, err, "Expected error but got none")
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg, "Error message should contain: %s", tt.errorMsg)
				}
			} else {
				assert.NoError(t, err, "Expected no error but got: %v", err)
			}
		})
	}
}
