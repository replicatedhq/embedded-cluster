package lint

import (
	"os"
	"path/filepath"
	"testing"

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
