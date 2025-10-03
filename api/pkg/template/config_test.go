package template

import (
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_templateConfigItems(t *testing.T) {
	tests := []struct {
		name         string
		config       *kotsv1beta1.Config
		configValues types.AppConfigValues
		expected     *kotsv1beta1.Config
	}{
		{
			name: "templates in value and default fields",
			config: &kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "settings",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "hostname",
									Value:   multitype.FromString("repl{{ upper \"app.example.com\" }}"),
									Default: multitype.FromString("repl{{ lower \"LOCALHOST\" }}"),
								},
								{
									Name:    "port",
									Default: multitype.FromString("repl{{ add 8000 80 }}"),
								},
							},
						},
					},
				},
			},
			expected: &kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "settings",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "hostname",
									Value:   multitype.FromString("APP.EXAMPLE.COM"),
									Default: multitype.FromString("localhost"),
								},
								{
									Name:    "port",
									Value:   multitype.FromString(""),
									Default: multitype.FromString("8080"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "template dependencies",
			config: &kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "dependent",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "base_url",
									Default: multitype.FromString("https://api.example.com"),
								},
								{
									Name:    "full_endpoint",
									Value:   multitype.FromString("repl{{ ConfigOption \"base_url\" }}/v1/health"),
									Default: multitype.FromString("https://localhost/health"),
								},
							},
						},
					},
				},
			},
			expected: &kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "dependent",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "base_url",
									Value:   multitype.FromString(""),
									Default: multitype.FromString("https://api.example.com"),
								},
								{
									Name:    "full_endpoint",
									Value:   multitype.FromString("https://api.example.com/v1/health"),
									Default: multitype.FromString("https://localhost/health"),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "file type with templates",
			config: &kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "files",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "cert_file",
									Type:    "file",
									Value:   multitype.FromString("repl{{ upper \"cert\" }}.pem"),
									Default: multitype.FromString("repl{{ lower \"DEFAULT\" }}-cert.pem"),
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"cert_file": {Filename: "uploaded-cert.pem", Value: "cert-content"},
			},
			expected: &kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "files",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "cert_file",
									Type:     "file",
									Value:    multitype.FromString("cert-content"),
									Default:  multitype.FromString("default-cert.pem"),
									Filename: "uploaded-cert.pem",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "templates in non-templated fields are not currently processed",
			config: &kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "repl{{ upper \"group\" }}",
							Title: "repl{{ title \"settings group\" }}",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:        "hostname",
									Title:       "repl{{ title \"hostname field\" }}",
									HelpText:    "repl{{ upper \"help text\" }}",
									Type:        "repl{{ lower \"TEXT\" }}",
									Value:       multitype.FromString("repl{{ upper \"app.example.com\" }}"),
									Default:     multitype.FromString("localhost"),
									Recommended: true,
									Required:    true,
								},
							},
						},
					},
				},
			},
			expected: &kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "repl{{ upper \"group\" }}",
							Title: "repl{{ title \"settings group\" }}",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:        "hostname",
									Title:       "repl{{ title \"hostname field\" }}",
									HelpText:    "repl{{ upper \"help text\" }}",
									Type:        "repl{{ lower \"TEXT\" }}",
									Value:       multitype.FromString("APP.EXAMPLE.COM"),
									Default:     multitype.FromString("localhost"),
									Recommended: true,
									Required:    true,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "filename preservation from user config values with config default and value",
			config: &kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "files",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "user_overridden",
									Type:     "file",
									Value:    multitype.FromString("user_overridden_content"),
									Default:  multitype.FromString("user_overridden_default"),
									Filename: "user_overridden.txt",
								},
								{
									Name:     "user_cleared",
									Type:     "file",
									Value:    multitype.FromString("user_cleared_content"),
									Default:  multitype.FromString("user_cleared_default"),
									Filename: "user_cleared.txt",
								},
								{
									Name:     "no_user_value",
									Type:     "file",
									Value:    multitype.FromString("no_user_value_content"),
									Default:  multitype.FromString("no_user_value_default"),
									Filename: "no_user_value.txt",
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"user_overridden": {Filename: "overridden.txt", Value: "overridden_content"}, // user overridden value
				"user_cleared":    {Filename: "", Value: ""},                                 // user cleared value
				// no user supplied value for no_user_value
			},
			expected: &kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "files",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "user_overridden",
									Type:     "file",
									Value:    multitype.FromString("overridden_content"),
									Default:  multitype.FromString("user_overridden_default"),
									Filename: "overridden.txt",
								},
								{
									Name:     "user_cleared",
									Type:     "file",
									Value:    multitype.FromString(""), // Empty differs from config value = user cleared it
									Default:  multitype.FromString("user_cleared_default"),
									Filename: "", // Empty differs from config filename = user cleared it
								},
								{
									Name:     "no_user_value",
									Type:     "file",
									Value:    multitype.FromString("no_user_value_content"),
									Default:  multitype.FromString("no_user_value_default"),
									Filename: "no_user_value.txt",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine(tt.config, WithMode(ModeConfig))
			if tt.configValues != nil {
				engine.configValues = tt.configValues
			}

			result, err := engine.templateConfigItems()
			require.NoError(t, err)
			assert.NotNil(t, result)

			// Compare the entire config structure
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEngine_ConfigValueChanged(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "test",
					Items: []kotsv1beta1.ConfigItem{
						{Name: "item1"},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	// Test 1: Both don't exist - no change
	engine.prevConfigValues = types.AppConfigValues{}
	engine.configValues = types.AppConfigValues{}
	assert.False(t, engine.configValueChanged("item1"))

	// Test 2: Previous exists, current doesn't exist - existence change always detected
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: "value1"},
	}
	engine.configValues = types.AppConfigValues{}
	assert.True(t, engine.configValueChanged("item1"), "should detect existence change (removal)")

	// Test 3: Previous exists with empty value, current doesn't exist - existence change always detected
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: ""},
	}
	engine.configValues = types.AppConfigValues{}
	assert.True(t, engine.configValueChanged("item1"), "should detect existence change (removal)")

	// Test 4: Previous doesn't exist, current exists - existence change always detected
	engine.prevConfigValues = types.AppConfigValues{}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: "value1"},
	}
	assert.True(t, engine.configValueChanged("item1"), "should detect existence change (addition)")

	// Test 5: Previous doesn't exist, current exists with empty value - existence change always detected
	engine.prevConfigValues = types.AppConfigValues{}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: ""},
	}
	assert.True(t, engine.configValueChanged("item1"), "should detect existence change (addition)")

	// Test 6: Both exist with same value - no change
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: "value1"},
	}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: "value1"},
	}
	assert.False(t, engine.configValueChanged("item1"), "should not detect change when values are same")

	// Test 7: Both exist with different values - change
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: "value1"},
	}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: "value2"},
	}
	assert.True(t, engine.configValueChanged("item1"), "should detect change when values differ")

	// Test 8: Both exist, previous empty, current non-empty - change
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: ""},
	}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: "value1"},
	}
	assert.True(t, engine.configValueChanged("item1"), "should detect change from empty to non-empty")

	// Test 9: Both exist, previous non-empty, current empty - change
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: "value1"},
	}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: ""},
	}
	assert.True(t, engine.configValueChanged("item1"), "should detect change from non-empty to empty")
}

func TestEngine_ShouldInvalidateItem(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "test",
					Items: []kotsv1beta1.ConfigItem{
						{Name: "item1"},
						{Name: "item2"},
						{Name: "item3"},
					},
				},
			},
		},
	}
	engine := NewEngine(config)

	// Test 1: Item has no dependency tree and no value change - should not invalidate
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: "value1"},
	}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: "value1"},
	}
	engine.depsTree = map[string][]string{}
	assert.False(t, engine.shouldInvalidateItem("item1"), "should not invalidate when no change and no dependencies")

	// Test 2: Item has value change - should invalidate
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: "value1"},
	}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: "value2"},
	}
	engine.depsTree = map[string][]string{}
	assert.True(t, engine.shouldInvalidateItem("item1"), "should invalidate when value changed")

	// Test 3: Item has no value change but dependency changed - should invalidate
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: "value1"},
		"item2": {Value: "dep_value1"},
	}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: "value1"},
		"item2": {Value: "dep_value2"},
	}
	engine.depsTree = map[string][]string{
		"item1": {"item2"},
	}
	assert.True(t, engine.shouldInvalidateItem("item1"), "should invalidate when dependency changed")

	// Test 4: Item has no value change and dependencies unchanged - should not invalidate
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: "value1"},
		"item2": {Value: "dep_value1"},
	}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: "value1"},
		"item2": {Value: "dep_value1"},
	}
	engine.depsTree = map[string][]string{
		"item1": {"item2"},
	}
	assert.False(t, engine.shouldInvalidateItem("item1"), "should not invalidate when no change in item or dependencies")

	// Test 5: Deep dependency chain with change at the bottom - should invalidate all up the chain
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: "value1"},
		"item2": {Value: "value2"},
		"item3": {Value: "value3_old"},
	}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: "value1"},
		"item2": {Value: "value2"},
		"item3": {Value: "value3_new"},
	}
	engine.depsTree = map[string][]string{
		"item1": {"item2"},
		"item2": {"item3"},
	}
	assert.True(t, engine.shouldInvalidateItem("item1"), "should invalidate when deep dependency changed")
	assert.True(t, engine.shouldInvalidateItem("item2"), "should invalidate when direct dependency changed")
	assert.True(t, engine.shouldInvalidateItem("item3"), "should invalidate when own value changed")

	// Test 6: Multiple dependencies, only one changed - should invalidate
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: "value1"},
		"item2": {Value: "dep1_value1"},
		"item3": {Value: "dep2_value1"},
	}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: "value1"},
		"item2": {Value: "dep1_value2"}, // changed
		"item3": {Value: "dep2_value1"}, // unchanged
	}
	engine.depsTree = map[string][]string{
		"item1": {"item2", "item3"},
	}
	assert.True(t, engine.shouldInvalidateItem("item1"), "should invalidate when one of multiple dependencies changed")

	// Test 7: Multiple dependencies, none changed - should not invalidate
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: "value1"},
		"item2": {Value: "dep1_value1"},
		"item3": {Value: "dep2_value1"},
	}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: "value1"},
		"item2": {Value: "dep1_value1"},
		"item3": {Value: "dep2_value1"},
	}
	engine.depsTree = map[string][]string{
		"item1": {"item2", "item3"},
	}
	assert.False(t, engine.shouldInvalidateItem("item1"), "should not invalidate when none of multiple dependencies changed")

	// Test 8: Item not in dependency tree and no value change - should not invalidate
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: "value1"},
	}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: "value1"},
	}
	engine.depsTree = map[string][]string{
		"item2": {"item3"}, // item1 not in tree
	}
	assert.False(t, engine.shouldInvalidateItem("item1"), "should not invalidate when item not in tree and no value change")

	// Test 9: Middle dependency change should not invalidate its dependencies, only dependents
	// Chain: item1 -> item2 -> item3, change item2
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: "value1"},
		"item2": {Value: "value2_old"},
		"item3": {Value: "value3"},
	}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: "value1"},
		"item2": {Value: "value2_new"}, // changed
		"item3": {Value: "value3"},
	}
	engine.depsTree = map[string][]string{
		"item1": {"item2"},
		"item2": {"item3"},
	}
	assert.True(t, engine.shouldInvalidateItem("item1"), "should invalidate item1 (dependent of changed item2)")
	assert.True(t, engine.shouldInvalidateItem("item2"), "should invalidate item2 (changed directly)")
	assert.False(t, engine.shouldInvalidateItem("item3"), "should not invalidate item3 (dependency of changed item2)")

	// Test 10: Top level change should not invalidate its dependencies
	// Chain: item1 -> item2 -> item3, change item1
	engine.prevConfigValues = types.AppConfigValues{
		"item1": {Value: "value1_old"},
		"item2": {Value: "value2"},
		"item3": {Value: "value3"},
	}
	engine.configValues = types.AppConfigValues{
		"item1": {Value: "value1_new"}, // changed
		"item2": {Value: "value2"},
		"item3": {Value: "value3"},
	}
	engine.depsTree = map[string][]string{
		"item1": {"item2"},
		"item2": {"item3"},
	}
	assert.True(t, engine.shouldInvalidateItem("item1"), "should invalidate item1 (changed directly)")
	assert.False(t, engine.shouldInvalidateItem("item2"), "should not invalidate item2 (dependency of changed item1)")
	assert.False(t, engine.shouldInvalidateItem("item3"), "should not invalidate item3 (dependency of changed item1)")

	// Test 11: Item that doesn't exist in either config values should not invalidate
	engine.prevConfigValues = types.AppConfigValues{}
	engine.configValues = types.AppConfigValues{}
	engine.depsTree = map[string][]string{}
	assert.False(t, engine.shouldInvalidateItem("item1"), "should not invalidate item1 as it doesn't exist in either config values")
}

func TestEngine_GetItemFilename(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "test",
					Items: []kotsv1beta1.ConfigItem{
						{Name: "file_with_user_filename"},
						{Name: "file_without_user_filename"},
						{Name: "file_with_empty_user_filename"},
						{Name: "non_file_item"},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	// Test 1: User provides filename - should use user filename
	engine.configValues = types.AppConfigValues{
		"file_with_user_filename": {Filename: "user-provided.txt", Value: "content"},
	}
	filename := engine.getItemFilename(&kotsv1beta1.ConfigItem{Name: "file_with_user_filename"})
	assert.Equal(t, "user-provided.txt", filename, "should use user-provided filename")

	// Test 2: User doesn't provide filename - should return empty string
	engine.configValues = types.AppConfigValues{
		"file_without_user_filename": {Value: "content"},
	}
	filename = engine.getItemFilename(&kotsv1beta1.ConfigItem{Name: "file_without_user_filename"})
	assert.Equal(t, "", filename, "should return empty string when user doesn't provide filename")

	// Test 3: User provides empty filename - should return empty string
	engine.configValues = types.AppConfigValues{
		"file_with_empty_user_filename": {Filename: "", Value: "content"},
	}
	filename = engine.getItemFilename(&kotsv1beta1.ConfigItem{Name: "file_with_empty_user_filename"})
	assert.Equal(t, "", filename, "should return empty string when user provides empty filename")

	// Test 4: Item not in config values - should return empty string
	filename = engine.getItemFilename(&kotsv1beta1.ConfigItem{Name: "non_file_item"})
	assert.Equal(t, "", filename, "should return empty string for item not in config values")

	// Test 5: Empty config values - should return empty string
	engine.configValues = types.AppConfigValues{}
	filename = engine.getItemFilename(&kotsv1beta1.ConfigItem{Name: "file_with_user_filename"})
	assert.Equal(t, "", filename, "should return empty string when config values is empty")
}

func TestEngine_FilenamePreservationInTemplateConfigItems(t *testing.T) {
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "filename_test",
					Items: []kotsv1beta1.ConfigItem{
						{
							Name:    "preserved_file",
							Type:    "file",
							Value:   multitype.FromString(""),
							Default: multitype.FromString("default.txt"),
						},
						{
							Name:    "overwritten_file",
							Type:    "file",
							Value:   multitype.FromString(""),
							Default: multitype.FromString("default.txt"),
						},
						{
							Name:    "text_item",
							Type:    "text",
							Value:   multitype.FromString(""),
							Default: multitype.FromString("default"),
						},
					},
				},
			},
		},
	}

	engine := NewEngine(config, WithMode(ModeConfig))
	engine.configValues = types.AppConfigValues{
		"preserved_file":   {Filename: "preserved.txt", Value: "content1"},
		"overwritten_file": {Filename: "overwritten.txt", Value: "content2"},
		"text_item":        {Value: "text content"},
	}

	result, err := engine.templateConfigItems()
	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify filename is preserved for file items
	preservedItem := result.Spec.Groups[0].Items[0]
	assert.Equal(t, "preserved.txt", preservedItem.Filename, "filename should be preserved for preserved_file")

	overwrittenItem := result.Spec.Groups[0].Items[1]
	assert.Equal(t, "overwritten.txt", overwrittenItem.Filename, "filename should be preserved for overwritten_file")

	// Verify text items don't have filename field set
	textItem := result.Spec.Groups[0].Items[2]
	assert.Equal(t, "", textItem.Filename, "text items should not have filename field set")
}

func TestEngine_GetItemFilenameDirect(t *testing.T) {
	// Test the getItemFilename function directly
	config := &kotsv1beta1.Config{
		Spec: kotsv1beta1.ConfigSpec{
			Groups: []kotsv1beta1.ConfigGroup{
				{
					Name: "test",
					Items: []kotsv1beta1.ConfigItem{
						{Name: "test_file"},
					},
				},
			},
		},
	}

	engine := NewEngine(config)

	// Test with user filename
	engine.configValues = types.AppConfigValues{
		"test_file": {Filename: "test.txt", Value: "content"},
	}

	configItem := &kotsv1beta1.ConfigItem{Name: "test_file"}
	filename := engine.getItemFilename(configItem)

	t.Logf("User filename test: expected 'test.txt', got '%s'", filename)
	assert.Equal(t, "test.txt", filename, "should return user filename when provided")

	// Test without user filename
	engine.configValues = types.AppConfigValues{}
	filename = engine.getItemFilename(configItem)

	t.Logf("No user filename test: expected '', got '%s'", filename)
	assert.Equal(t, "", filename, "should return empty string when no user filename")
}
