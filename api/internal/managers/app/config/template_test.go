package config

import (
	"testing"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/assert"
)

func TestProcessTemplate(t *testing.T) {
	// Create a manager using NewAppConfigManager
	manager := NewAppConfigManager(kotsv1beta1.Config{})

	tests := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{
			name:     "empty input",
			input:    "",
			expected: "",
			hasError: false,
		},
		{
			name:     "no template",
			input:    "title: HTTP Configuration",
			expected: "title: HTTP Configuration",
			hasError: false,
		},
		{
			name:     "simple print template",
			input:    "title: {{ print \"HTTP Configuration\" }}",
			expected: "title: HTTP Configuration",
			hasError: false,
		},
		{
			name:     "printf template",
			input:    "help: {{ printf \"Port number (default: %d)\" 8080 }}",
			expected: "help: Port number (default: 8080)",
			hasError: false,
		},
		{
			name:     "sprig upper function",
			input:    "title: {{ upper \"http enabled\" }}",
			expected: "title: HTTP ENABLED",
			hasError: false,
		},
		{
			name:     "sprig lower function",
			input:    "title: {{ lower \"HTTP ENABLED\" }}",
			expected: "title: http enabled",
			hasError: false,
		},
		{
			name:     "sprig default function",
			input:    "value: {{ default \"8080\" \"\" }}",
			expected: "value: 8080",
			hasError: false,
		},
		{
			name:     "multiple templates",
			input:    "title: {{ print \"HTTP\" }}\nhelp: {{ printf \"Port %d\" 8080 }}",
			expected: "title: HTTP\nhelp: Port 8080",
			hasError: false,
		},
		{
			name:     "invalid template syntax",
			input:    "title: {{ invalid syntax",
			expected: "",
			hasError: true,
		},
		{
			name:     "template execution error",
			input:    "title: {{ .UnknownField }}",
			expected: "title: <no value>",
			hasError: false,
		},
		{
			name: "complex YAML with multiple templates",
			input: `
apiVersion: kots.io/v1beta1
kind: Config
metadata:
  name: simple-config
spec:
  groups:
    - name: http_config
      title: {{ print "HTTP Configuration" }}
      items:
      - name: http_enabled
        title: {{ print "HTTP Enabled" }}
        type: bool
        default: {{ print "true" }}
      - name: http_port
        title: {{ upper "http port" }}
        type: text
        default: {{ print "8080" }}
        help: {{ printf "Port number (default: %d)" 8080 }}
`,
			expected: `
apiVersion: kots.io/v1beta1
kind: Config
metadata:
  name: simple-config
spec:
  groups:
    - name: http_config
      title: HTTP Configuration
      items:
      - name: http_enabled
        title: HTTP Enabled
        type: bool
        default: true
      - name: http_port
        title: HTTP PORT
        type: text
        default: 8080
        help: Port number (default: 8080)
`,
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := manager.processTemplate(tt.input)
			
			if tt.hasError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}