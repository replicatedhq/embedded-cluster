package config

import (
	"testing"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConfigTemplateProcessing(t *testing.T) {
	// Create a comprehensive config with various template scenarios
	config := kotsv1beta1.Config{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kots.io/v1beta1",
			Kind:       "Config",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-config",
		},
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name:  "basic_templates",
					Title: `{{ print "Basic Template Tests" }}`,
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:     "simple_print",
							Title:    `{{ print "Simple Print" }}`,
							Type:     "text",
							Default:  multitype.BoolOrString{StrVal: `{{ print "default_value" }}`},
							Value:    multitype.BoolOrString{StrVal: `{{ print "actual_value" }}`},
							HelpText: `{{ print "This is help text" }}`,
						},
						{
							Name:    "printf_test",
							Title:   `{{ printf "Port: %d" 8080 }}`,
							Type:    "text",
							Default: multitype.BoolOrString{StrVal: `{{ printf "%d" 9000 }}`},
							Value:   multitype.BoolOrString{StrVal: `{{ printf "%d" 3000 }}`},
						},
					},
				},
				{
					Name:  "sprig_functions",
					Title: `{{ upper "sprig function tests" }}`,
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "upper_lower",
							Title:   `{{ upper "http port" }}`,
							Type:    "text",
							Default: multitype.BoolOrString{StrVal: `{{ lower "DEFAULT_VALUE" }}`},
							Value:   multitype.BoolOrString{StrVal: `{{ upper "value_text" }}`},
						},
						{
							Name:    "default_function",
							Title:   `{{ print "Default Function" }}`,
							Type:    "text",
							Default: multitype.BoolOrString{StrVal: `{{ default "fallback" "" }}`},
							Value:   multitype.BoolOrString{StrVal: `{{ default "main_value" "" }}`},
						},
						{
							Name:    "quote_function",
							Title:   `{{ print "Quote Function" }}`,
							Type:    "text",
							Default: multitype.BoolOrString{StrVal: `{{ quote "quoted_value" }}`},
							Value:   multitype.BoolOrString{StrVal: `{{ quote "actual_quoted" }}`},
						},
					},
				},
				{
					Name:  "edge_cases",
					Title: `{{ print "Edge Cases" }}`,
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "undefined_field",
							Title:   `{{ .NonExistentField }}`, // This will render as "<no value>" - Go template's default for undefined fields
							Type:    "text",
							Default: multitype.BoolOrString{StrVal: `{{ .AnotherUndefinedField }}`}, // This will also render as "<no value>"
							Value:   multitype.BoolOrString{StrVal: `{{ .YetAnotherUndefinedField }}`}, // This will also render as "<no value>"
						},
						{
							Name:    "empty_template",
							Title:   `Regular Title`,
							Type:    "text",
							Default: multitype.BoolOrString{StrVal: `regular_value`},
							Value:   multitype.BoolOrString{StrVal: `regular_actual_value`},
						},
					},
				},
			},
		},
	}

	// Test successful template processing
	t.Run("successful_template_processing", func(t *testing.T) {
		manager, err := NewAppConfigManager(config)
		require.NoError(t, err)
		require.NotNil(t, manager)

		result, err := manager.executeConfigTemplate()
		require.NoError(t, err)
		require.NotEmpty(t, result)

		// Verify the entire resulted config string
		expectedYAML := `apiVersion: kots.io/v1beta1
kind: Config
metadata:
  creationTimestamp: null
  name: test-config
spec:
  groups:
  - items:
    - default: 'default_value'
      help_text: 'This is help text'
      name: simple_print
      title: 'Simple Print'
      type: text
      value: 'actual_value'
    - default: '9000'
      name: printf_test
      title: 'Port: 8080'
      type: text
      value: '3000'
    name: basic_templates
    title: 'Basic Template Tests'
  - items:
    - default: 'default_value'
      name: upper_lower
      title: 'HTTP PORT'
      type: text
      value: 'VALUE_TEXT'
    - default: 'fallback'
      name: default_function
      title: 'Default Function'
      type: text
      value: 'main_value'
    - default: '"quoted_value"'
      name: quote_function
      title: 'Quote Function'
      type: text
      value: '"actual_quoted"'
    name: sprig_functions
    title: 'SPRIG FUNCTION TESTS'
  - items:
    - default: '<no value>'
      name: undefined_field
      title: '<no value>'
      type: text
      value: '<no value>'
    - default: regular_value
      name: empty_template
      title: Regular Title
      type: text
      value: regular_actual_value
    name: edge_cases
    title: 'Edge Cases'
status: {}
`

		assert.Equal(t, expectedYAML, result)
	})

	// Test invalid template syntax
	t.Run("invalid_template_syntax", func(t *testing.T) {
		invalidConfig := kotsv1beta1.Config{
			Spec: kotsv1beta1.ConfigSpec{
				Groups: []kotsv1beta1.ConfigGroup{
					{
						Name:  "invalid_group",
						Title: `{{ invalid template syntax`,
					},
				},
			},
		}

		manager, err := NewAppConfigManager(invalidConfig)
		assert.Error(t, err)
		assert.Nil(t, manager)
		assert.Contains(t, err.Error(), "template parse error")
	})

	// Test empty config
	t.Run("empty_config", func(t *testing.T) {
		emptyConfig := kotsv1beta1.Config{}

		manager, err := NewAppConfigManager(emptyConfig)
		require.NoError(t, err)
		require.NotNil(t, manager)

		result, err := manager.executeConfigTemplate()
		require.NoError(t, err)
		require.NotEmpty(t, result)

		// Even empty configs should produce valid YAML structure
		assert.Contains(t, result, "metadata:")
		assert.Contains(t, result, "spec:")
		assert.Contains(t, result, "groups: null")
	})

	// Test complex nested template scenario
	t.Run("complex_nested_templates", func(t *testing.T) {
		complexConfig := kotsv1beta1.Config{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "kots.io/v1beta1",
				Kind:       "Config",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "complex-config",
			},
			Spec: kotsv1beta1.ConfigSpec{
				Groups: []kotsv1beta1.ConfigGroup{
					{
						Name:  "complex_group",
						Title: `{{ printf "Complex %s Configuration" (upper "nested") }}`,
						Items: []kotsv1beta1.ConfigItem{
							{
								Name:     "complex_item",
								Title:    `{{ printf "%s: %s" (upper "database") (lower "CONNECTION") }}`,
								Type:     "text",
								Default:  multitype.BoolOrString{StrVal: `{{ printf "host:%s,port:%d" (default "localhost" "") 5432 }}`},
								Value:    multitype.BoolOrString{StrVal: `{{ printf "host:%s,port:%d" (default "prod-db" "") 5433 }}`},
								HelpText: `{{ printf "Configure %s settings for %s" (lower "DATABASE") (upper "application") }}`,
							},
							{
								Name:    "conditional_item",
								Title:   `{{ if true }}Enabled Feature{{ else }}Disabled Feature{{ end }}`,
								Type:    "bool",
								Default: multitype.BoolOrString{StrVal: `{{ if true }}true{{ else }}false{{ end }}`},
								Value:   multitype.BoolOrString{StrVal: `{{ if false }}true{{ else }}false{{ end }}`},
							},
						},
					},
				},
			},
		}

		manager, err := NewAppConfigManager(complexConfig)
		require.NoError(t, err)
		require.NotNil(t, manager)

		result, err := manager.executeConfigTemplate()
		require.NoError(t, err)
		require.NotEmpty(t, result)

		// Verify the entire resulted config string
		expectedYAML := `apiVersion: kots.io/v1beta1
kind: Config
metadata:
  creationTimestamp: null
  name: complex-config
spec:
  groups:
  - items:
    - default: 'host:localhost,port:5432'
      help_text: 'Configure database settings for APPLICATION'
      name: complex_item
      title: 'DATABASE: connection'
      type: text
      value: 'host:prod-db,port:5433'
    - default: 'true'
      name: conditional_item
      title: 'Enabled Feature'
      type: bool
      value: 'false'
    name: complex_group
    title: 'Complex NESTED Configuration'
status: {}
`

		assert.Equal(t, expectedYAML, result)
	})

	// Test that templates are processed only once during initialization
	t.Run("template_initialization_only", func(t *testing.T) {
		manager, err := NewAppConfigManager(config)
		require.NoError(t, err)
		require.NotNil(t, manager)

		// Execute template multiple times
		result1, err1 := manager.executeConfigTemplate()
		require.NoError(t, err1)

		result2, err2 := manager.executeConfigTemplate()
		require.NoError(t, err2)

		// Results should be identical (template was parsed once)
		assert.Equal(t, result1, result2)
		assert.NotEmpty(t, result1)
		assert.NotEmpty(t, result2)
	})
}
