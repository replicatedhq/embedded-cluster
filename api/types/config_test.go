package types

import (
	"testing"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/require"
)

func TestConvertConfigValue(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    kotsv1beta1.ConfigValue
		expected AppConfigValue
	}{
		{
			name: "regular config value",
			key:  "regular_item",
			value: kotsv1beta1.ConfigValue{
				Default:        "default_value",
				Value:          "actual_value",
				Data:           "data_content",
				ValuePlaintext: "plain_value",
				DataPlaintext:  "plain_data",
				Filename:       "config.txt",
				RepeatableItem: "repeat_value",
			},
			expected: AppConfigValue{
				Default:        "default_value",
				Value:          "actual_value",
				Data:           "data_content",
				ValuePlaintext: "plain_value",
				DataPlaintext:  "plain_data",
				Filename:       "config.txt",
				RepeatableItem: "repeat_value",
			},
		},
		{
			name: "json config value with content",
			key:  "api_config_json",
			value: kotsv1beta1.ConfigValue{
				Default: "",
				Value:   `{"key": "value"}`,
			},
			expected: AppConfigValue{
				Default:        "",
				Value:          `{"key": "value"}`,
				Data:           "",
				ValuePlaintext: "",
				DataPlaintext:  "",
				Filename:       "",
				RepeatableItem: "",
			},
		},
		{
			name: "empty json config value gets fixed",
			key:  "empty_config_json",
			value: kotsv1beta1.ConfigValue{
				Default: "default",
				Value:   "",
			},
			expected: AppConfigValue{
				Default:        "default",
				Value:          "{}",
				Data:           "",
				ValuePlaintext: "",
				DataPlaintext:  "",
				Filename:       "",
				RepeatableItem: "",
			},
		},
		{
			name: "non-json config value with empty value remains empty",
			key:  "regular_config",
			value: kotsv1beta1.ConfigValue{
				Value: "",
			},
			expected: AppConfigValue{
				Default:        "",
				Value:          "",
				Data:           "",
				ValuePlaintext: "",
				DataPlaintext:  "",
				Filename:       "",
				RepeatableItem: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertConfigValue(tt.key, tt.value)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertToAppConfigValues(t *testing.T) {
	tests := []struct {
		name     string
		input    *kotsv1beta1.ConfigValues
		expected AppConfigValues
	}{
		{
			name:     "nil input returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name: "empty config values",
			input: &kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{},
				},
			},
			expected: AppConfigValues{},
		},
		{
			name: "single config value",
			input: &kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"test_item": {
							Default: "default_val",
							Value:   "actual_val",
						},
					},
				},
			},
			expected: AppConfigValues{
				"test_item": AppConfigValue{
					Default:        "default_val",
					Value:          "actual_val",
					Data:           "",
					ValuePlaintext: "",
					DataPlaintext:  "",
					Filename:       "",
					RepeatableItem: "",
				},
			},
		},
		{
			name: "multiple config values including json fix",
			input: &kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"regular_item": {
							Value: "regular_value",
						},
						"empty_json_item": {
							Value: "",
						},
						"filled_json_item": {
							Value: `{"configured": true}`,
						},
						"config_json": {
							Value: "",
						},
					},
				},
			},
			expected: AppConfigValues{
				"regular_item": AppConfigValue{
					Value: "regular_value",
				},
				"empty_json_item": AppConfigValue{
					Value: "",
				},
				"filled_json_item": AppConfigValue{
					Value: `{"configured": true}`,
				},
				"config_json": AppConfigValue{
					Value: "{}",
				},
			},
		},
		{
			name: "comprehensive test with all fields",
			input: &kotsv1beta1.ConfigValues{
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"full_config": {
							Default:        "def",
							Value:          "val",
							Data:           "data",
							ValuePlaintext: "plain_val",
							DataPlaintext:  "plain_data",
							Filename:       "file.txt",
							RepeatableItem: "repeat",
						},
					},
				},
			},
			expected: AppConfigValues{
				"full_config": AppConfigValue{
					Default:        "def",
					Value:          "val",
					Data:           "data",
					ValuePlaintext: "plain_val",
					DataPlaintext:  "plain_data",
					Filename:       "file.txt",
					RepeatableItem: "repeat",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToAppConfigValues(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertToAppConfigValues_JSONFix(t *testing.T) {
	input := &kotsv1beta1.ConfigValues{
		Spec: kotsv1beta1.ConfigValuesSpec{
			Values: map[string]kotsv1beta1.ConfigValue{
				"api_config_json":  {Value: ""},
				"db_settings_json": {Value: ""},
				"tls_json":         {Value: ""},
				"regular_config":   {Value: ""},
				"not_json_suffix":  {Value: ""},
			},
		},
	}

	result := ConvertToAppConfigValues(input)

	// Verify JSON fix is applied only to keys ending with "_json"
	require.Equal(t, "{}", result["api_config_json"].Value)
	require.Equal(t, "{}", result["db_settings_json"].Value)
	require.Equal(t, "{}", result["tls_json"].Value)
	require.Equal(t, "", result["regular_config"].Value)
	require.Equal(t, "", result["not_json_suffix"].Value)
}
