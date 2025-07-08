package render

import (
	"testing"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateConfigValues(t *testing.T) {
	tests := []struct {
		name           string
		config         kotsv1beta1.Config
		expectedResult kotsv1beta1.ConfigValues
	}{
		{
			name: "successful conversion with boolean defaults",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "settings",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "enable_feature",
									Type:    "bool",
									Default: multitype.FromString("0"),
								},
								{
									Name:    "show_advanced",
									Type:    "bool",
									Default: multitype.FromString("1"),
								},
							},
						},
					},
				},
			},
			expectedResult: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"enable_feature": {
							Value:   "0",
							Default: "0",
						},
						"show_advanced": {
							Value:   "1",
							Default: "1",
						},
					},
				},
			},
		},
		{
			name: "handles missing defaults gracefully",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "settings",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name: "enable_feature",
									Type: "bool",
									// No default field
								},
							},
						},
					},
				},
			},
			expectedResult: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"enable_feature": {
							Value:   "",
							Default: "",
						},
					},
				},
			},
		},
		{
			name: "preserves user-set values over defaults",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "settings",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "enable_feature",
									Type:    "bool",
									Default: multitype.FromString("0"),
									Value:   multitype.FromString("1"), // User set to "1"
								},
							},
						},
					},
				},
			},
			expectedResult: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"enable_feature": {
							Value:   "1", // User's value takes precedence
							Default: "0", // Original default preserved
						},
					},
				},
			},
		},
		{
			name: "filters out non-boolean fields",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "settings",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "enable_feature",
									Type:    "bool",
									Default: multitype.FromString("0"),
								},
								{
									Name:    "some_text_field",
									Type:    "text",
									Default: multitype.FromString("default text"),
								},
								{
									Name:    "some_password_field",
									Type:    "password",
									Default: multitype.FromString("secret"),
								},
							},
						},
					},
				},
			},
			expectedResult: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"enable_feature": {
							Value:   "0",
							Default: "0",
						},
						// Non-boolean fields should be filtered out
					},
				},
			},
		},
		{
			name: "handles empty config gracefully",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{},
				},
			},
			expectedResult: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateConfigValues(tt.config)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestProcessBooleanConfigItem(t *testing.T) {
	tests := []struct {
		name              string
		item              kotsv1beta1.ConfigItem
		expectedValue     kotsv1beta1.ConfigValue
		expectedProcessed bool
	}{
		{
			name: "processes boolean item with default value",
			item: kotsv1beta1.ConfigItem{
				Name:    "enable_feature",
				Type:    "bool",
				Default: multitype.FromString("0"),
			},
			expectedValue: kotsv1beta1.ConfigValue{
				Value:   "0",
				Default: "0",
			},
			expectedProcessed: true,
		},
		{
			name: "processes boolean item with user value overriding default",
			item: kotsv1beta1.ConfigItem{
				Name:    "enable_feature",
				Type:    "bool",
				Default: multitype.FromString("0"),
				Value:   multitype.FromString("1"),
			},
			expectedValue: kotsv1beta1.ConfigValue{
				Value:   "1",
				Default: "0",
			},
			expectedProcessed: true,
		},
		{
			name: "processes boolean item with empty default",
			item: kotsv1beta1.ConfigItem{
				Name: "enable_feature",
				Type: "bool",
				// No default or value set
			},
			expectedValue: kotsv1beta1.ConfigValue{
				Value:   "",
				Default: "",
			},
			expectedProcessed: true,
		},
		{
			name: "ignores non-boolean item",
			item: kotsv1beta1.ConfigItem{
				Name:    "some_text",
				Type:    "text",
				Default: multitype.FromString("default text"),
			},
			expectedValue:     kotsv1beta1.ConfigValue{},
			expectedProcessed: false,
		},
		{
			name: "ignores password item",
			item: kotsv1beta1.ConfigItem{
				Name:    "some_password",
				Type:    "password",
				Default: multitype.FromString("secret"),
			},
			expectedValue:     kotsv1beta1.ConfigValue{},
			expectedProcessed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, processed := processBooleanConfigItem(tt.item)
			assert.Equal(t, tt.expectedProcessed, processed)
			if processed {
				assert.Equal(t, tt.expectedValue, value)
			}
		})
	}
}
