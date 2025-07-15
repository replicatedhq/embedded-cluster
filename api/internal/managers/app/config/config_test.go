package config

import (
	"errors"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/internal/store/app/config"
	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tiendc/go-deepcopy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAppConfigManager_GetConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   kotsv1beta1.Config
		expected kotsv1beta1.Config
	}{
		{
			name: "config with template processing",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "templated_group",
							Title: "{{ print \"HTTP Configuration\" }}",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "templated_item",
									Title: "{{ upper \"http enabled\" }}",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "{{ print \"8080\" }}"},
									When:  "true",
								},
							},
						},
					},
				},
			},
			expected: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "templated_group",
							Title: "HTTP Configuration",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "templated_item",
									Title: "HTTP ENABLED",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "8080"},
									When:  "true",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "config with template processing and filtering",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "templated_enabled_group",
							Title: "{{ print \"Enabled Group\" }}",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "templated_enabled_item",
									Title: "{{ printf \"Port: %d\" 8080 }}",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "{{ print \"true\" }}"},
									When:  "true",
								},
								{
									Name:  "templated_disabled_item",
									Title: "{{ print \"Disabled Item\" }}",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "{{ print \"false\" }}"},
									When:  "false",
								},
							},
						},
						{
							Name:  "templated_disabled_group",
							Title: "{{ print \"Disabled Group\" }}",
							When:  "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "item_in_disabled_group",
									Title: "{{ print \"Item in Disabled Group\" }}",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "{{ print \"disabled\" }}"},
									When:  "true",
								},
							},
						},
					},
				},
			},
			expected: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "templated_enabled_group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "templated_enabled_item",
									Title: "Port: 8080",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "true"},
									When:  "true",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "conditional filtering with when conditions",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "visible_config_group",
							Title: "Visible Config Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "visible_config_group_visible_item_1",
									Title: "Visible Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "visible_config_group_visible_item_1_value"},
									When:  "true",
								},
								{
									Name:  "visible_config_group_invisible_item_1",
									Title: "Invisible Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "visible_config_group_invisible_item_1_value"},
									When:  "false",
								},
							},
						},
						{
							Name:  "invisible_config_group",
							Title: "Invisible Config Group",
							When:  "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "invisible_config_group_visible_item_1",
									Title: "Visible Item 2",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "invisible_config_group_visible_item_1_value"},
									When:  "true",
								},
								{
									Name:  "invisible_config_group_invisible_item_2",
									Title: "Invisible Item 2",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "invisible_config_group_invisible_item_2_value"},
									When:  "false",
								},
							},
						},
						{
							Name:  "no_visible_items_group",
							Title: "No Visible Items Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "no_visible_items_group_item_1",
									Title: "No Visible Items Group Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "no_visible_items_group_item_1_value"},
									When:  "false",
								},
							},
						},
					},
				},
			},
			expected: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "visible_config_group",
							Title: "Visible Config Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "visible_config_group_visible_item_1",
									Title: "Visible Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "visible_config_group_visible_item_1_value"},
									When:  "true",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "conditional filtering with empty when conditions",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "group_with_empty_when",
							Title: "Group with Empty When",
							When:  "",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "item_with_empty_when",
									Title: "Item with Empty When",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "item_with_empty_when_value"},
									When:  "",
								},
								{
									Name:  "item_with_false_when",
									Title: "Item with False When",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "item_with_false_when_value"},
									When:  "false",
								},
							},
						},
					},
				},
			},
			expected: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "group_with_empty_when",
							Title: "Group with Empty When",
							When:  "",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "item_with_empty_when",
									Title: "Item with Empty When",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "item_with_empty_when_value"},
									When:  "",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "empty config with no groups",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{},
				},
			},
			expected: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{},
				},
			},
		},
		{
			name: "config with empty groups",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "empty_group",
							Title: "Empty Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{},
						},
					},
				},
			},
			expected: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{},
				},
			},
		},
		{
			name: "config with all disabled groups",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "disabled_group_1",
							Title: "Disabled Group 1",
							When:  "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "item_in_disabled_group",
									Title: "Item in Disabled Group",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "item_in_disabled_group_value"},
									When:  "true",
								},
							},
						},
						{
							Name:  "disabled_group_2",
							Title: "Disabled Group 2",
							When:  "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "another_item_in_disabled_group",
									Title: "Another Item in Disabled Group",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "another_item_in_disabled_group_value"},
									When:  "true",
								},
							},
						},
					},
				},
			},
			expected: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{},
				},
			},
		},
		{
			name: "config with mixed enabled and disabled items in enabled group",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "mixed_group",
							Title: "Mixed Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "enabled_item_1",
									Title: "Enabled Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "enabled_item_1_value"},
									When:  "true",
								},
								{
									Name:  "disabled_item_1",
									Title: "Disabled Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "disabled_item_1_value"},
									When:  "false",
								},
								{
									Name:  "enabled_item_2",
									Title: "Enabled Item 2",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "enabled_item_2_value"},
									When:  "true",
								},
								{
									Name:  "disabled_item_2",
									Title: "Disabled Item 2",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "disabled_item_2_value"},
									When:  "false",
								},
							},
						},
					},
				},
			},
			expected: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "mixed_group",
							Title: "Mixed Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "enabled_item_1",
									Title: "Enabled Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "enabled_item_1_value"},
									When:  "true",
								},
								{
									Name:  "enabled_item_2",
									Title: "Enabled Item 2",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "enabled_item_2_value"},
									When:  "true",
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
			manager, err := NewAppConfigManager(tt.config)
			assert.NoError(t, err)

			// Apply values to config
			result, err := manager.GetConfig()

			// Verify no error occurred
			require.NoError(t, err)

			// Verify the result matches expected
			assert.Equal(t, tt.expected, result)

			// Verify the original config was not modified (deep copy worked)
			assert.Equal(t, originalConfig, tt.config, "original config should not be modified")
		})
	}
}

func TestAppConfigManager_PatchConfigValues(t *testing.T) {
	tests := []struct {
		name      string
		config    kotsv1beta1.Config
		newValues types.AppConfigValues
		setupMock func(*config.MockStore)
		wantErr   bool
	}{
		{
			name: "enabled group and items with new values",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "enabled-item-1",
									Title: "Enabled Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-value-1"},
									When:  "true",
								},
								{
									Name:  "enabled-item-2",
									Title: "Enabled Item 2",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-value-2"},
									When:  "true",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "enabled-child-item",
											Title: "Enabled Child Item",
											Value: multitype.BoolOrString{StrVal: "original-child-value"},
										},
									},
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"enabled-item-1":     types.AppConfigValue{Value: "new-value-1"},
				"enabled-item-2":     types.AppConfigValue{Value: "new-value-2"},
				"enabled-child-item": types.AppConfigValue{Value: "new-child-value"},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{
					"enabled-item-1":     types.AppConfigValue{Value: "new-value-1"},
					"enabled-item-2":     types.AppConfigValue{Value: "new-value-2"},
					"enabled-child-item": types.AppConfigValue{Value: "new-child-value"},
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "disabled group with enabled items - items should be filtered out",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "disabled-group",
							Title: "Disabled Group",
							When:  "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "item-in-disabled-group",
									Title: "Item in Disabled Group",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-value"},
									When:  "true",
								},
								{
									Name:  "child-in-disabled-group",
									Title: "Child in Disabled Group",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-child-value"},
									When:  "true",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "grandchild-in-disabled-group",
											Title: "Grandchild in Disabled Group",
											Value: multitype.BoolOrString{StrVal: "original-grandchild-value"},
										},
									},
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"item-in-disabled-group":       types.AppConfigValue{Value: "new-value"},
				"child-in-disabled-group":      types.AppConfigValue{Value: "new-child-value"},
				"grandchild-in-disabled-group": types.AppConfigValue{Value: "new-grandchild-value"},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "disabled group with disabled items - items should be filtered out",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "disabled-group",
							Title: "Disabled Group",
							When:  "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "disabled-item-in-disabled-group",
									Title: "Disabled Item in Disabled Group",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-value"},
									When:  "false",
								},
								{
									Name:  "enabled-item-in-disabled-group",
									Title: "Enabled Item in Disabled Group",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-enabled-value"},
									When:  "true",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"disabled-item-in-disabled-group": types.AppConfigValue{Value: "new-disabled-value"},
				"enabled-item-in-disabled-group":  types.AppConfigValue{Value: "new-enabled-value"},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "enabled group with disabled item - disabled item should be filtered out",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "enabled-item",
									Title: "Enabled Item",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "enabled-original-value"},
									When:  "true",
								},
								{
									Name:  "disabled-item",
									Title: "Disabled Item",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "disabled-original-value"},
									When:  "false",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "child-of-disabled-item",
											Title: "Child of Disabled Item",
											Value: multitype.BoolOrString{StrVal: "child-of-disabled-original-value"},
										},
									},
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"enabled-item":           types.AppConfigValue{Value: "new-enabled-value"},
				"disabled-item":          types.AppConfigValue{Value: "new-disabled-value"},
				"child-of-disabled-item": types.AppConfigValue{Value: "new-child-value"},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{
					"enabled-item": types.AppConfigValue{Value: "new-enabled-value"},
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "mixed enabled and disabled items in enabled group",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "mixed-group",
							Title: "Mixed Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "enabled-item",
									Title: "Enabled Item",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "enabled-original"},
									When:  "true",
								},
								{
									Name:  "disabled-item",
									Title: "Disabled Item",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "disabled-original"},
									When:  "false",
								},
								{
									Name:  "enabled-item-with-children",
									Title: "Enabled Item with Children",
									Type:  "group",
									Value: multitype.BoolOrString{StrVal: "parent-original"},
									When:  "true",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "enabled-child",
											Title: "Enabled Child",
											Value: multitype.BoolOrString{StrVal: "enabled-child-original"},
										},
									},
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"enabled-item":               types.AppConfigValue{Value: "new-enabled-value"},
				"disabled-item":              types.AppConfigValue{Value: "new-disabled-value"},
				"enabled-item-with-children": types.AppConfigValue{Value: "new-parent-value"},
				"enabled-child":              types.AppConfigValue{Value: "new-enabled-child-value"},
				"disabled-child":             types.AppConfigValue{Value: "new-disabled-child-value"},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{
					"enabled-item":               types.AppConfigValue{Value: "new-enabled-value"},
					"enabled-item-with-children": types.AppConfigValue{Value: "new-parent-value"},
					"enabled-child":              types.AppConfigValue{Value: "new-enabled-child-value"},
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "multiple groups with mixed enabled/disabled states",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group-1",
							Title: "Enabled Group 1",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "enabled-item-1",
									Title: "Enabled Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-1"},
									When:  "true",
								},
								{
									Name:  "disabled-item-1",
									Title: "Disabled Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-disabled-1"},
									When:  "false",
								},
							},
						},
						{
							Name:  "disabled-group",
							Title: "Disabled Group",
							When:  "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "item-in-disabled-group",
									Title: "Item in Disabled Group",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-disabled-group"},
									When:  "true",
								},
							},
						},
						{
							Name:  "enabled-group-2",
							Title: "Enabled Group 2",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "enabled-item-2",
									Title: "Enabled Item 2",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-2"},
									When:  "true",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"enabled-item-1":         types.AppConfigValue{Value: "new-value-1"},
				"disabled-item-1":        types.AppConfigValue{Value: "new-disabled-value-1"},
				"item-in-disabled-group": types.AppConfigValue{Value: "new-disabled-group-value"},
				"enabled-item-2":         types.AppConfigValue{Value: "new-value-2"},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{
					"enabled-item-1": types.AppConfigValue{Value: "new-value-1"},
					"enabled-item-2": types.AppConfigValue{Value: "new-value-2"},
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "empty config values",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "enabled-item",
									Title: "Enabled Item",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-value"},
									When:  "true",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "store error",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "enabled-item",
									Title: "Enabled Item",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-value"},
									When:  "true",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"enabled-item": types.AppConfigValue{Value: "new-value"},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{
					"enabled-item": types.AppConfigValue{Value: "new-value"},
				}
				mockStore.On("SetConfigValues", expectedValues).Return(errors.New("store error"))
			},
			wantErr: true,
		},
		{
			name: "empty when conditions treated as enabled",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "group-with-empty-when",
							Title: "Group with Empty When",
							When:  "",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "item-with-empty-when",
									Title: "Item with Empty When",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-value"},
									When:  "",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"item-with-empty-when": types.AppConfigValue{Value: "new-value"},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{
					"item-with-empty-when": types.AppConfigValue{Value: "new-value"},
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "all groups disabled",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "disabled-group-1",
							Title: "Disabled Group 1",
							When:  "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "item-1",
									Title: "Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-1"},
									When:  "true",
								},
							},
						},
						{
							Name:  "disabled-group-2",
							Title: "Disabled Group 2",
							When:  "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "item-2",
									Title: "Item 2",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-2"},
									When:  "true",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"item-1": types.AppConfigValue{Value: "new-value-1"},
				"item-2": types.AppConfigValue{Value: "new-value-2"},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "enabled item without config value",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "enabled-item-1",
									Title: "Enabled Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-value-1"},
									When:  "true",
								},
								{
									Name:  "enabled-item-2",
									Title: "Enabled Item 2",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-value-2"},
									When:  "true",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"enabled-item-1": types.AppConfigValue{Value: "new-value-1"},
				// enabled-item-2 intentionally omitted
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{
					"enabled-item-1": types.AppConfigValue{Value: "new-value-1"},
					// enabled-item-2 should not be included since no value provided
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "patch with existing values - new values override existing",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "item-1",
									Title: "Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-1"},
									When:  "true",
								},
								{
									Name:  "item-2",
									Title: "Item 2",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-2"},
									When:  "true",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"item-1": types.AppConfigValue{Value: "new-value-1"},
				"item-2": types.AppConfigValue{Value: "new-value-2"},
			},
			setupMock: func(mockStore *config.MockStore) {
				existingValues := types.AppConfigValues{
					"item-1": types.AppConfigValue{Value: "existing-value-1"},
					"item-2": types.AppConfigValue{Value: "existing-value-2"},
				}
				mockStore.On("GetConfigValues").Return(existingValues, nil)
				expectedValues := types.AppConfigValues{
					"item-1": types.AppConfigValue{Value: "new-value-1"},
					"item-2": types.AppConfigValue{Value: "new-value-2"},
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "patch with existing values - partial update",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "item-1",
									Title: "Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-1"},
									When:  "true",
								},
								{
									Name:  "item-2",
									Title: "Item 2",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-2"},
									When:  "true",
								},
								{
									Name:  "item-3",
									Title: "Item 3",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-3"},
									When:  "true",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"item-1": types.AppConfigValue{Value: "new-value-1"},
				// item-2 not provided, should keep existing value
				"item-3": types.AppConfigValue{Value: "new-value-3"},
			},
			setupMock: func(mockStore *config.MockStore) {
				existingValues := types.AppConfigValues{
					"item-1": types.AppConfigValue{Value: "existing-value-1"},
					"item-2": types.AppConfigValue{Value: "existing-value-2"},
					"item-3": types.AppConfigValue{Value: "existing-value-3"},
				}
				mockStore.On("GetConfigValues").Return(existingValues, nil)
				expectedValues := types.AppConfigValues{
					"item-1": types.AppConfigValue{Value: "new-value-1"},
					"item-2": types.AppConfigValue{Value: "existing-value-2"},
					"item-3": types.AppConfigValue{Value: "new-value-3"},
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "patch with empty string values",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "item-1",
									Title: "Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-1"},
									When:  "true",
								},
								{
									Name:  "item-2",
									Title: "Item 2",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-2"},
									When:  "true",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"item-1": types.AppConfigValue{Value: ""},
				"item-2": types.AppConfigValue{Value: "new-value-2"},
			},
			setupMock: func(mockStore *config.MockStore) {
				existingValues := types.AppConfigValues{
					"item-1": types.AppConfigValue{Value: "existing-value-1"},
					"item-2": types.AppConfigValue{Value: "existing-value-2"},
				}
				mockStore.On("GetConfigValues").Return(existingValues, nil)
				expectedValues := types.AppConfigValues{
					"item-1": types.AppConfigValue{Value: ""},
					"item-2": types.AppConfigValue{Value: "new-value-2"},
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "patch with existing values and disabled items",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "enabled-item",
									Title: "Enabled Item",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-enabled"},
									When:  "true",
								},
								{
									Name:  "disabled-item",
									Title: "Disabled Item",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-disabled"},
									When:  "false",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"enabled-item":  types.AppConfigValue{Value: "new-enabled-value"},
				"disabled-item": types.AppConfigValue{Value: "new-disabled-value"},
			},
			setupMock: func(mockStore *config.MockStore) {
				existingValues := types.AppConfigValues{
					"enabled-item":  types.AppConfigValue{Value: "existing-enabled-value"},
					"disabled-item": types.AppConfigValue{Value: "existing-disabled-value"},
				}
				mockStore.On("GetConfigValues").Return(existingValues, nil)
				expectedValues := types.AppConfigValues{
					"enabled-item": types.AppConfigValue{Value: "new-enabled-value"},
					// disabled-item should not be included in filtered values
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "patch with existing values - new item added",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "existing-item",
									Title: "Existing Item",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-existing"},
									When:  "true",
								},
								{
									Name:  "new-item",
									Title: "New Item",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-new"},
									When:  "true",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"existing-item": types.AppConfigValue{Value: "updated-existing-value"},
				"new-item":      types.AppConfigValue{Value: "brand-new-value"},
			},
			setupMock: func(mockStore *config.MockStore) {
				existingValues := types.AppConfigValues{
					"existing-item": types.AppConfigValue{Value: "existing-value"},
					// new-item not in existing values
				}
				mockStore.On("GetConfigValues").Return(existingValues, nil)
				expectedValues := types.AppConfigValues{
					"existing-item": types.AppConfigValue{Value: "updated-existing-value"},
					"new-item":      types.AppConfigValue{Value: "brand-new-value"},
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "empty config with no groups",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{},
				},
			},
			newValues: types.AppConfigValues{
				"some-item": types.AppConfigValue{Value: "some-value"},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "enabled group with empty items",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "empty-group",
							Title: "Empty Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"some-item": types.AppConfigValue{Value: "some-value"},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "enabled group with all disabled items",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "all-disabled-items-group",
							Title: "All Disabled Items Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "disabled-item-1",
									Title: "Disabled Item 1",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-1"},
									When:  "false",
								},
								{
									Name:  "disabled-item-2",
									Title: "Disabled Item 2",
									Type:  "text",
									Value: multitype.BoolOrString{StrVal: "original-2"},
									When:  "false",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"disabled-item-1": types.AppConfigValue{Value: "new-value-1"},
				"disabled-item-2": types.AppConfigValue{Value: "new-value-2"},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "config with template processing and filtering",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "templated-group",
							Title: "{{ upper \"http configuration\" }}",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "templated-enabled-item",
									Title: "{{ printf \"Port: %d\" 8080 }}",
									Type:  "text",
									When:  "true",
								},
								{
									Name:  "templated-disabled-item",
									Title: "{{ lower \"DISABLED ITEM\" }}",
									Type:  "text",
									When:  "false",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"templated-enabled-item":  types.AppConfigValue{Value: "enabled-value"},
				"templated-disabled-item": types.AppConfigValue{Value: "disabled-value"},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{
					"templated-enabled-item": types.AppConfigValue{Value: "enabled-value"},
					// templated-disabled-item should be filtered out due to when: "false"
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock store
			mockStore := &config.MockStore{}
			tt.setupMock(mockStore)

			// Create manager with mock store
			manager, err := NewAppConfigManager(tt.config, WithAppConfigStore(mockStore))
			assert.NoError(t, err)

			// Call PatchConfigValues
			err = manager.PatchConfigValues(tt.newValues)

			// Verify expectations
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			// Verify mock expectations
			mockStore.AssertExpectations(t)
		})
	}
}

func TestAppConfigManager_GetConfigValues(t *testing.T) {
	tests := []struct {
		name           string
		appConfig      kotsv1beta1.Config
		maskPasswords  bool
		storeValues    types.AppConfigValues
		storeError     error
		expectedValues types.AppConfigValues
		wantErr        bool
	}{
		{
			name: "get config values without masking",
			appConfig: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "test-group",
							Title: "Test Group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "username",
									Title: "Username",
									Type:  "text",
								},
								{
									Name:  "password",
									Title: "Password",
									Type:  "password",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "confirm-password",
											Title: "Confirm Password",
										},
									},
								},
								{
									Name:  "email",
									Title: "Email Address",
									Type:  "text",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "email-verification",
											Title: "Email Verification",
										},
									},
								},
							},
						},
					},
				},
			},
			maskPasswords: false,
			storeValues: types.AppConfigValues{
				"username":           types.AppConfigValue{Value: "admin"},
				"password":           types.AppConfigValue{Value: "secret123"},
				"confirm-password":   types.AppConfigValue{Value: "different-secret"},
				"email":              types.AppConfigValue{Value: "admin@example.com"},
				"email-verification": types.AppConfigValue{Value: "verified"},
			},
			expectedValues: types.AppConfigValues{
				"username":           types.AppConfigValue{Value: "admin"},
				"password":           types.AppConfigValue{Value: "secret123"},
				"confirm-password":   types.AppConfigValue{Value: "different-secret"},
				"email":              types.AppConfigValue{Value: "admin@example.com"},
				"email-verification": types.AppConfigValue{Value: "verified"},
			},
			wantErr: false,
		},
		{
			name: "get config values with password masking",
			appConfig: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "test-group",
							Title: "Test Group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "username",
									Title: "Username",
									Type:  "text",
								},
								{
									Name:  "password",
									Title: "Password",
									Type:  "password",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "confirm-password",
											Title: "Confirm Password",
										},
									},
								},
								{
									Name:  "api-key",
									Title: "API Key",
									Type:  "password",
								},
								{
									Name:  "email",
									Title: "Email Address",
									Type:  "text",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "email-verification",
											Title: "Email Verification",
										},
									},
								},
							},
						},
					},
				},
			},
			maskPasswords: true,
			storeValues: types.AppConfigValues{
				"username":           types.AppConfigValue{Value: "admin"},
				"password":           types.AppConfigValue{Value: "secret123"},
				"confirm-password":   types.AppConfigValue{Value: "different-secret"},
				"api-key":            types.AppConfigValue{Value: "key-abc123"},
				"email":              types.AppConfigValue{Value: "admin@example.com"},
				"email-verification": types.AppConfigValue{Value: "verified"},
			},
			expectedValues: types.AppConfigValues{
				"username":           types.AppConfigValue{Value: "admin"},
				"password":           types.AppConfigValue{Value: PasswordMask},
				"confirm-password":   types.AppConfigValue{Value: PasswordMask},
				"api-key":            types.AppConfigValue{Value: PasswordMask},
				"email":              types.AppConfigValue{Value: "admin@example.com"},
				"email-verification": types.AppConfigValue{Value: "verified"},
			},
			wantErr: false,
		},
		{
			name: "password masking with empty password values",
			appConfig: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "test-group",
							Title: "Test Group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "username",
									Title: "Username",
									Type:  "text",
								},
								{
									Name:  "password",
									Title: "Password",
									Type:  "password",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "confirm-password",
											Title: "Confirm Password",
										},
									},
								},
								{
									Name:  "api-key",
									Title: "API Key",
									Type:  "password",
								},
								{
									Name:  "secret-token",
									Title: "Secret Token",
									Type:  "password",
								},
								{
									Name:  "email",
									Title: "Email Address",
									Type:  "text",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "email-verification",
											Title: "Email Verification",
										},
									},
								},
							},
						},
					},
				},
			},
			maskPasswords: true,
			storeValues: types.AppConfigValues{
				"username":           types.AppConfigValue{Value: "admin"},
				"password":           types.AppConfigValue{Value: ""}, // empty password should not be masked
				"confirm-password":   types.AppConfigValue{Value: ""}, // empty child password should not be masked
				"api-key":            types.AppConfigValue{Value: "key-abc123"},
				"secret-token":       types.AppConfigValue{Value: ""}, // another empty password should not be masked
				"email":              types.AppConfigValue{Value: "admin@example.com"},
				"email-verification": types.AppConfigValue{Value: "verified"},
			},
			expectedValues: types.AppConfigValues{
				"username":           types.AppConfigValue{Value: "admin"},
				"password":           types.AppConfigValue{Value: ""}, // empty password values are not masked
				"confirm-password":   types.AppConfigValue{Value: ""}, // empty child password values are not masked
				"api-key":            types.AppConfigValue{Value: PasswordMask},
				"secret-token":       types.AppConfigValue{Value: ""}, // empty password values are not masked
				"email":              types.AppConfigValue{Value: "admin@example.com"},
				"email-verification": types.AppConfigValue{Value: "verified"},
			},
			wantErr: false,
		},
		{
			name: "password masking with missing password fields",
			appConfig: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "test-group",
							Title: "Test Group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "username",
									Title: "Username",
									Type:  "text",
								},
								{
									Name:  "password",
									Title: "Password",
									Type:  "password",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "confirm-password",
											Title: "Confirm Password",
										},
									},
								},
								{
									Name:  "email",
									Title: "Email Address",
									Type:  "text",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "email-verification",
											Title: "Email Verification",
										},
									},
								},
							},
						},
					},
				},
			},
			maskPasswords: true,
			storeValues: types.AppConfigValues{
				"username":           types.AppConfigValue{Value: "admin"},
				"email":              types.AppConfigValue{Value: "admin@example.com"},
				"email-verification": types.AppConfigValue{Value: "verified"},
				// password and confirm-password not in store values
			},
			expectedValues: types.AppConfigValues{
				"username":           types.AppConfigValue{Value: "admin"},
				"email":              types.AppConfigValue{Value: "admin@example.com"},
				"email-verification": types.AppConfigValue{Value: "verified"},
				// password and confirm-password should not appear in result
			},
			wantErr: false,
		},
		{
			name: "store error",
			appConfig: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "test-group",
							Title: "Test Group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "username",
									Title: "Username",
									Type:  "text",
								},
							},
						},
					},
				},
			},
			maskPasswords:  false,
			storeValues:    nil,
			storeError:     errors.New("store connection error"),
			expectedValues: nil,
			wantErr:        true,
		},
		{
			name: "config with template processing and password masking",
			appConfig: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "templated-group",
							Title: "{{ title \"user configuration\" }}",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "templated-text",
									Title: "{{ upper \"text field\" }}",
									Type:  "text",
									When:  "true",
								},
								{
									Name:  "templated-password",
									Title: "{{ printf \"Password for %s\" \"admin\" }}",
									Type:  "password",
									When:  "true",
								},
								{
									Name:  "disabled-templated-item",
									Title: "{{ lower \"DISABLED ITEM\" }}",
									Type:  "text",
									When:  "false",
								},
							},
						},
					},
				},
			},
			maskPasswords: true,
			storeValues: types.AppConfigValues{
				"templated-text":          types.AppConfigValue{Value: "text-value"},
				"templated-password":      types.AppConfigValue{Value: "secret-password"},
				"disabled-templated-item": types.AppConfigValue{Value: "disabled-value"},
			},
			expectedValues: types.AppConfigValues{
				"templated-text":          types.AppConfigValue{Value: "text-value"},
				"templated-password":      types.AppConfigValue{Value: PasswordMask},
				"disabled-templated-item": types.AppConfigValue{Value: "disabled-value"}, // store value is returned even if item is disabled
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock store
			mockStore := &config.MockStore{}
			mockStore.On("GetConfigValues").Return(tt.storeValues, tt.storeError)

			// Create manager with mock store
			manager, err := NewAppConfigManager(tt.appConfig, WithAppConfigStore(mockStore))
			assert.NoError(t, err)

			// Call GetConfigValues
			result, err := manager.GetConfigValues(tt.maskPasswords)

			// Verify expectations
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedValues, result)
			}

			// Verify mock expectations
			mockStore.AssertExpectations(t)
		})
	}
}

func TestAppConfigManager_GetKotsadmConfigValues(t *testing.T) {
	tests := []struct {
		name      string
		config    kotsv1beta1.Config
		setupMock func(*config.MockStore)
		expected  kotsv1beta1.ConfigValues
		wantErr   bool
	}{
		{
			name: "successful merge of config defaults and store values",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "enabled-item-1",
									Title:   "Enabled Item 1",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "default-value-1"},
									Default: multitype.BoolOrString{StrVal: "default-value-1"},
									When:    "true",
								},
								{
									Name:    "enabled-item-2",
									Title:   "Enabled Item 2",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "default-value-2"},
									Default: multitype.BoolOrString{StrVal: "default-value-2"},
									When:    "true",
								},
							},
						},
					},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				storeValues := types.AppConfigValues{
					"enabled-item-1": types.AppConfigValue{Value: "store-value-1"},
					"enabled-item-2": types.AppConfigValue{Value: "store-value-2"},
				}
				mockStore.On("GetConfigValues").Return(storeValues, nil)
			},
			expected: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kots-app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"enabled-item-1": {
							Value:   "store-value-1",
							Default: "default-value-1",
						},
						"enabled-item-2": {
							Value:   "store-value-2",
							Default: "default-value-2",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "disabled groups and items are filtered out",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "enabled-item",
									Title:   "Enabled Item",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "enabled-default"},
									Default: multitype.BoolOrString{StrVal: "enabled-default"},
									When:    "true",
								},
							},
						},
						{
							Name:  "disabled-group",
							Title: "Disabled Group",
							When:  "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "disabled-item",
									Title:   "Disabled Item",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "disabled-default"},
									Default: multitype.BoolOrString{StrVal: "disabled-default"},
									When:    "true",
								},
							},
						},
					},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				storeValues := types.AppConfigValues{
					"enabled-item":  types.AppConfigValue{Value: "enabled-store"},
					"disabled-item": types.AppConfigValue{Value: "disabled-store"},
				}
				mockStore.On("GetConfigValues").Return(storeValues, nil)
			},
			expected: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kots-app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"enabled-item": {
							Value:   "enabled-store",
							Default: "enabled-default",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty when conditions treated as enabled",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "group-with-empty-when",
							Title: "Group with Empty When",
							When:  "",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "item-with-empty-when",
									Title:   "Item with Empty When",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "empty-default"},
									Default: multitype.BoolOrString{StrVal: "empty-default"},
									When:    "",
								},
							},
						},
					},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				storeValues := types.AppConfigValues{
					"item-with-empty-when": types.AppConfigValue{Value: "empty-store"},
				}
				mockStore.On("GetConfigValues").Return(storeValues, nil)
			},
			expected: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kots-app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"item-with-empty-when": {
							Value:   "empty-store",
							Default: "empty-default",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty config with no groups",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				storeValues := types.AppConfigValues{
					"some-store-value": types.AppConfigValue{Value: "store-value"},
				}
				mockStore.On("GetConfigValues").Return(storeValues, nil)
			},
			expected: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kots-app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{},
				},
			},
			wantErr: false,
		},
		{
			name: "all groups disabled",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "disabled-group-1",
							Title: "Disabled Group 1",
							When:  "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "item-1",
									Title:   "Item 1",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "default-1"},
									Default: multitype.BoolOrString{StrVal: "default-1"},
									When:    "true",
								},
							},
						},
						{
							Name:  "disabled-group-2",
							Title: "Disabled Group 2",
							When:  "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "item-2",
									Title:   "Item 2",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "default-2"},
									Default: multitype.BoolOrString{StrVal: "default-2"},
									When:    "true",
								},
							},
						},
					},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				storeValues := types.AppConfigValues{
					"item-1": types.AppConfigValue{Value: "store-1"},
					"item-2": types.AppConfigValue{Value: "store-2"},
				}
				mockStore.On("GetConfigValues").Return(storeValues, nil)
			},
			expected: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kots-app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{},
				},
			},
			wantErr: false,
		},
		{
			name: "store error",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "enabled-item",
									Title:   "Enabled Item",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "default-value"},
									Default: multitype.BoolOrString{StrVal: "default-value"},
									When:    "true",
								},
							},
						},
					},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(nil, errors.New("store error"))
			},
			wantErr: true,
		},
		{
			name: "mixed enabled and disabled items in enabled group",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "mixed-group",
							Title: "Mixed Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "enabled-item",
									Title:   "Enabled Item",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "enabled-default"},
									Default: multitype.BoolOrString{StrVal: "enabled-default"},
									When:    "true",
								},
								{
									Name:    "disabled-item",
									Title:   "Disabled Item",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "disabled-default"},
									Default: multitype.BoolOrString{StrVal: "disabled-default"},
									When:    "false",
								},
							},
						},
					},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				storeValues := types.AppConfigValues{
					"enabled-item":  types.AppConfigValue{Value: "enabled-store"},
					"disabled-item": types.AppConfigValue{Value: "disabled-store"},
				}
				mockStore.On("GetConfigValues").Return(storeValues, nil)
			},
			expected: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kots-app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"enabled-item": {
							Value:   "enabled-store",
							Default: "enabled-default",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "items with child items",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "parent-item",
									Title:   "Parent Item",
									Type:    "group",
									Value:   multitype.BoolOrString{StrVal: "parent-default"},
									Default: multitype.BoolOrString{StrVal: "parent-default"},
									When:    "true",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:    "child-item-1",
											Title:   "Child Item 1",
											Value:   multitype.BoolOrString{StrVal: "child-default-1"},
											Default: multitype.BoolOrString{StrVal: "child-default-1"},
										},
										{
											Name:    "child-item-2",
											Title:   "Child Item 2",
											Value:   multitype.BoolOrString{StrVal: "child-default-2"},
											Default: multitype.BoolOrString{StrVal: "child-default-2"},
										},
									},
								},
							},
						},
					},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				storeValues := types.AppConfigValues{
					"parent-item":  types.AppConfigValue{Value: "parent-store"},
					"child-item-1": types.AppConfigValue{Value: "child-store-1"},
					"child-item-2": types.AppConfigValue{Value: "child-store-2"},
				}
				mockStore.On("GetConfigValues").Return(storeValues, nil)
			},
			expected: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kots-app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"parent-item": {
							Value:   "parent-store",
							Default: "parent-default",
						},
						"child-item-1": {
							Value:   "child-store-1",
							Default: "child-default-1",
						},
						"child-item-2": {
							Value:   "child-store-2",
							Default: "child-default-2",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "store values not in config are ignored",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "enabled-item",
									Title:   "Enabled Item",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "enabled-default"},
									Default: multitype.BoolOrString{StrVal: "enabled-default"},
									When:    "true",
								},
							},
						},
					},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				storeValues := types.AppConfigValues{
					"enabled-item":    types.AppConfigValue{Value: "enabled-store"},
					"non-config-item": types.AppConfigValue{Value: "non-config-value"},
				}
				mockStore.On("GetConfigValues").Return(storeValues, nil)
			},
			expected: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kots-app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"enabled-item": {
							Value:   "enabled-store",
							Default: "enabled-default",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "items without store values use defaults",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "item-with-store",
									Title:   "Item with Store",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "default-with-store"},
									Default: multitype.BoolOrString{StrVal: "default-with-store"},
									When:    "true",
								},
								{
									Name:    "item-without-store",
									Title:   "Item without Store",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "default-without-store"},
									Default: multitype.BoolOrString{StrVal: "default-without-store"},
									When:    "true",
								},
							},
						},
					},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				storeValues := types.AppConfigValues{
					"item-with-store": types.AppConfigValue{Value: "store-value"},
				}
				mockStore.On("GetConfigValues").Return(storeValues, nil)
			},
			expected: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kots-app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"item-with-store": {
							Value:   "store-value",
							Default: "default-with-store",
						},
						"item-without-store": {
							Value:   "default-without-store",
							Default: "default-without-store",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty config values are not overridden by config value",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "item-with-empty-store",
									Title:   "Item with Empty Store",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "config-value"},
									Default: multitype.BoolOrString{StrVal: "config-default"},
									When:    "true",
								},
								{
									Name:    "item-with-non-empty-store",
									Title:   "Item with Non-Empty Store",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "config-value-2"},
									Default: multitype.BoolOrString{StrVal: "config-default-2"},
									When:    "true",
								},
							},
						},
					},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				storeValues := types.AppConfigValues{
					"item-with-empty-store":     types.AppConfigValue{Value: ""},
					"item-with-non-empty-store": types.AppConfigValue{Value: "store-value"},
				}
				mockStore.On("GetConfigValues").Return(storeValues, nil)
			},
			expected: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kots-app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"item-with-empty-store": {
							Value:   "",
							Default: "config-default",
						},
						"item-with-non-empty-store": {
							Value:   "store-value",
							Default: "config-default-2",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "password fields use ValuePlaintext",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "auth-group",
							Title: "Authentication Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "username",
									Title:   "Username",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "admin"},
									Default: multitype.BoolOrString{StrVal: "user"},
									When:    "true",
								},
								{
									Name:    "password",
									Title:   "Password",
									Type:    "password",
									Value:   multitype.BoolOrString{StrVal: "schema-password"},
									Default: multitype.BoolOrString{StrVal: "default-password"},
									When:    "true",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:    "confirm-password",
											Title:   "Confirm Password",
											Value:   multitype.BoolOrString{StrVal: "schema-confirm-password"},
											Default: multitype.BoolOrString{StrVal: "default-confirm-password"},
										},
										{
											Name:    "password-hint",
											Title:   "Password Hint",
											Value:   multitype.BoolOrString{StrVal: "schema-password-hint"},
											Default: multitype.BoolOrString{StrVal: "default-password-hint"},
										},
									},
								},
								{
									Name:    "api-key",
									Title:   "API Key",
									Type:    "password",
									Value:   multitype.BoolOrString{StrVal: "schema-api-key"},
									Default: multitype.BoolOrString{StrVal: "default-api-key"},
									When:    "true",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:    "api-secret",
											Title:   "API Secret",
											Value:   multitype.BoolOrString{StrVal: "schema-api-secret"},
											Default: multitype.BoolOrString{StrVal: "default-api-secret"},
										},
									},
								},
								{
									Name:    "secret-token",
									Title:   "Secret Token",
									Type:    "password",
									Value:   multitype.BoolOrString{StrVal: "schema-token"},
									Default: multitype.BoolOrString{StrVal: "default-token"},
									When:    "true",
								},
							},
						},
					},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				storeValues := types.AppConfigValues{
					"username": types.AppConfigValue{Value: "stored-username"},
					"password": types.AppConfigValue{Value: "stored-password"},
					"api-key":  types.AppConfigValue{Value: "stored-api-key"},
					// secret-token intentionally omitted to test fallback behavior
					"confirm-password": types.AppConfigValue{Value: "stored-confirm-password"},
					"password-hint":    types.AppConfigValue{Value: "stored-password-hint"},
					// api-secret intentionally omitted to test fallback behavior
				}
				mockStore.On("GetConfigValues").Return(storeValues, nil)
			},
			expected: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kots-app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"username": {
							Value:   "stored-username", // text field uses stored value
							Default: "user",
						},
						"password": {
							Value:          "",
							ValuePlaintext: "stored-password", // password with stored value: uses stored value
							Default:        "default-password",
						},
						"confirm-password": {
							Value:          "",
							ValuePlaintext: "stored-confirm-password", // child password with stored value
							Default:        "default-confirm-password",
						},
						"password-hint": {
							Value:          "",
							ValuePlaintext: "stored-password-hint", // child password with stored value
							Default:        "default-password-hint",
						},
						"api-key": {
							Value:          "",
							ValuePlaintext: "stored-api-key", // password with stored value: uses stored value
							Default:        "default-api-key",
						},
						"api-secret": {
							Value:          "",
							ValuePlaintext: "schema-api-secret", // child password without stored value: uses value from schema
							Default:        "default-api-secret",
						},
						"secret-token": {
							Value:          "",
							ValuePlaintext: "schema-token", // password without stored value: uses value from schema
							Default:        "default-token",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "config with template processing",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "templated-group",
							Title: "{{ title \"server configuration\" }}",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "templated-item",
									Title:   "{{ upper \"http port\" }}",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "{{ toString 8080 }}"},
									Default: multitype.BoolOrString{StrVal: "{{ toString 8080 }}"},
									When:    "true",
								},
								{
									Name:    "sprig-functions-item",
									Title:   "{{ printf \"Port: %d\" 9090 }}",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "{{ add 9000 90 }}"},
									Default: multitype.BoolOrString{StrVal: "{{ add 9000 90 }}"},
									When:    "true",
								},
								{
									Name:    "disabled-templated-item",
									Title:   "{{ lower \"DISABLED ITEM\" }}",
									Type:    "text",
									Value:   multitype.BoolOrString{StrVal: "{{ trim \"  disabled  \" }}"},
									Default: multitype.BoolOrString{StrVal: "{{ trim \"  disabled  \" }}"},
									When:    "false",
								},
							},
						},
					},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				storeValues := types.AppConfigValues{
					"templated-item":          types.AppConfigValue{Value: "store-overridden-value"},
					"sprig-functions-item":    types.AppConfigValue{Value: "store-sprig-value"},
					"disabled-templated-item": types.AppConfigValue{Value: "disabled-store-value"},
				}
				mockStore.On("GetConfigValues").Return(storeValues, nil)
			},
			expected: kotsv1beta1.ConfigValues{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "kots.io/v1beta1",
					Kind:       "ConfigValues",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "kots-app-config",
				},
				Spec: kotsv1beta1.ConfigValuesSpec{
					Values: map[string]kotsv1beta1.ConfigValue{
						"templated-item": {
							Value:   "store-overridden-value",
							Default: "8080",
						},
						"sprig-functions-item": {
							Value:   "store-sprig-value",
							Default: "9090",
						},
						// disabled-templated-item should not be present due to when: "false"
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock store
			mockStore := &config.MockStore{}
			tt.setupMock(mockStore)

			// Create manager with mock store
			manager, err := NewAppConfigManager(tt.config, WithAppConfigStore(mockStore))
			assert.NoError(t, err)

			// Call GetKotsadmConfigValues
			result, err := manager.GetKotsadmConfigValues()

			// Verify expectations
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}

			// Verify mock expectations
			mockStore.AssertExpectations(t)
		})
	}
}

func TestValidateConfigValues(t *testing.T) {
	tests := []struct {
		name         string
		config       kotsv1beta1.Config
		configValues types.AppConfigValues
		wantErr      bool
		errorFields  []string
	}{
		{
			name: "valid config with all required items set",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "group1",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "required_item",
									Required: true,
									Value:    multitype.BoolOrString{},
									Default:  multitype.BoolOrString{},
								},
								{
									Name:     "optional_item",
									Required: false,
									Value:    multitype.BoolOrString{},
									Default:  multitype.BoolOrString{},
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"required_item": types.AppConfigValue{Value: "value1"},
				"optional_item": types.AppConfigValue{Value: "value2"},
			},
			wantErr: false,
		},
		{
			name: "missing required item",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "group1",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "required_item",
									Required: true,
									Value:    multitype.BoolOrString{},
									Default:  multitype.BoolOrString{},
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"optional_item": types.AppConfigValue{Value: "value1"},
			},
			wantErr:     true,
			errorFields: []string{"required_item"},
		},
		{
			name: "required item with default value should not be required",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "group1",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "required_with_default",
									Required: true,
									Value:    multitype.BoolOrString{},
									Default:  multitype.BoolOrString{StrVal: "default_value"},
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{},
			wantErr:      false,
		},
		{
			name: "required item with value and no config value should not be required",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "group1",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "required_with_value",
									Required: true,
									Value:    multitype.BoolOrString{StrVal: "item_value"},
									Default:  multitype.BoolOrString{},
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{},
			wantErr:      false,
		},
		{
			name: "required item with value and empty config value should be required",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "group1",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "required_with_value",
									Required: true,
									Value:    multitype.BoolOrString{StrVal: "item_value"},
									Default:  multitype.BoolOrString{},
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"required_with_value": types.AppConfigValue{Value: ""},
			},
			wantErr:     true,
			errorFields: []string{"required_with_value"},
		},
		{
			name: "hidden required item should not be required",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "group1",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "hidden_required",
									Required: true,
									Hidden:   true,
									Value:    multitype.BoolOrString{},
									Default:  multitype.BoolOrString{},
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{},
			wantErr:      false,
		},
		{
			name: "disabled required item should not be required",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "group1",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "disabled_required",
									Required: true,
									When:     "false",
									Value:    multitype.BoolOrString{},
									Default:  multitype.BoolOrString{},
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{},
			wantErr:      false,
		},
		{
			name: "child item validation",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "group1",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name: "parent_item",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name: "child_item",
										},
									},
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"child_item": types.AppConfigValue{Value: "child_value"},
			},
			wantErr: false,
		},
		{
			name: "multiple validation errors",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "group1",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "required_item1",
									Required: true,
									Value:    multitype.BoolOrString{},
									Default:  multitype.BoolOrString{},
								},
								{
									Name:     "required_item2",
									Required: true,
									Value:    multitype.BoolOrString{},
									Default:  multitype.BoolOrString{},
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"unknown_item1": types.AppConfigValue{Value: "value1"},
				"unknown_item2": types.AppConfigValue{Value: "value2"},
			},
			wantErr:     true,
			errorFields: []string{"required_item1", "required_item2"},
		},
		{
			name: "empty config and values",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{},
				},
			},
			configValues: types.AppConfigValues{},
			wantErr:      false,
		},
		{
			name: "empty config with unknown values",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{},
				},
			},
			configValues: types.AppConfigValues{
				"unknown_item": types.AppConfigValue{Value: "value1"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a real appConfigManager instance for testing
			manager, err := NewAppConfigManager(tt.config)
			assert.NoError(t, err)

			// Run the validation
			err = manager.ValidateConfigValues(tt.configValues)

			// Check if error is expected
			if tt.wantErr {
				require.Error(t, err, "Expected validation to fail")

				// Check if it's an APIError with field errors
				var apiErr *types.APIError
				if assert.ErrorAs(t, err, &apiErr) {
					// Verify that all expected error fields are present
					for _, field := range tt.errorFields {
						found := false
						for _, fieldErr := range apiErr.Errors {
							if fieldErr.Field == field {
								found = true
								break
							}
						}
						assert.True(t, found, "Expected error for field %s", field)
					}
				}
			} else {
				assert.NoError(t, err, "Expected validation to succeed")
			}
		})
	}
}
