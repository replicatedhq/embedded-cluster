package config

import (
	"testing"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tiendc/go-deepcopy"
)

func TestAppConfigManager_ApplyValuesToConfig(t *testing.T) {
	tests := []struct {
		name         string
		config       kotsv1beta1.Config
		configValues map[string]string
		expected     kotsv1beta1.Config
	}{
		{
			name: "apply value to item and child item",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "test-group",
							Title: "Test Group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "test-item",
									Type:    "text",
									Title:   "Test Item",
									Default: multitype.BoolOrString{StrVal: "default"},
									Value:   multitype.BoolOrString{StrVal: "original"},
								},
								{
									Name:  "parent-item",
									Type:  "group",
									Title: "Parent Item",
									Value: multitype.BoolOrString{StrVal: "parent-original"},
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:    "child-item",
											Title:   "Child Item",
											Default: multitype.BoolOrString{StrVal: "child-default"},
											Value:   multitype.BoolOrString{StrVal: "child-original"},
										},
									},
								},
							},
						},
					},
				},
			},
			configValues: map[string]string{
				"test-item":  "new-value",
				"child-item": "child-new-value",
			},
			expected: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "test-group",
							Title: "Test Group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "test-item",
									Type:    "text",
									Title:   "Test Item",
									Default: multitype.BoolOrString{StrVal: "default"},
									Value:   multitype.BoolOrString{StrVal: "new-value"},
								},
								{
									Name:  "parent-item",
									Type:  "group",
									Title: "Parent Item",
									Value: multitype.BoolOrString{StrVal: "parent-original"},
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:    "child-item",
											Title:   "Child Item",
											Default: multitype.BoolOrString{StrVal: "child-default"},
											Value:   multitype.BoolOrString{StrVal: "child-new-value"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "empty config values",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "test-group",
							Title: "Test Group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "test-item",
									Type:    "text",
									Title:   "Test Item",
									Default: multitype.BoolOrString{StrVal: "default"},
									Value:   multitype.BoolOrString{StrVal: "original"},
								},
							},
						},
					},
				},
			},
			configValues: map[string]string{},
			expected: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "test-group",
							Title: "Test Group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "test-item",
									Type:    "text",
									Title:   "Test Item",
									Default: multitype.BoolOrString{StrVal: "default"},
									Value:   multitype.BoolOrString{StrVal: "original"},
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
			// Create a deep copy of the original config before testing
			var originalConfig kotsv1beta1.Config
			err := deepcopy.Copy(&originalConfig, &tt.config)
			require.NoError(t, err)

			// Create a new app config manager
			manager := NewAppConfigManager()

			// Apply values to config
			result, err := manager.ApplyValuesToConfig(tt.config, tt.configValues)

			// Verify no error occurred
			require.NoError(t, err)

			// Verify the result matches expected
			assert.Equal(t, tt.expected, result)

			// Verify the original config was not modified (deep copy worked)
			assert.Equal(t, originalConfig, tt.config, "original config should not be modified")
		})
	}
}
