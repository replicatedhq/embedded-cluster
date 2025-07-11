package config

import (
	"context"
	"errors"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/internal/store/app/config"
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
			manager := NewAppConfigManager()

			// Apply values to config
			result, err := manager.GetConfig(tt.config)

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
		newValues map[string]string
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
			newValues: map[string]string{
				"enabled-item-1":     "new-value-1",
				"enabled-item-2":     "new-value-2",
				"enabled-child-item": "new-child-value",
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(map[string]string{}, nil)
				expectedValues := map[string]string{
					"enabled-item-1":     "new-value-1",
					"enabled-item-2":     "new-value-2",
					"enabled-child-item": "new-child-value",
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "disabled group keeps original values",
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
			newValues: map[string]string{
				"item-in-disabled-group":       "new-value",
				"child-in-disabled-group":      "new-child-value",
				"grandchild-in-disabled-group": "new-grandchild-value",
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(map[string]string{}, nil)
				expectedValues := map[string]string{}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "disabled item keeps original value",
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
			newValues: map[string]string{
				"enabled-item":           "new-enabled-value",
				"disabled-item":          "new-disabled-value",
				"child-of-disabled-item": "new-child-value",
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(map[string]string{}, nil)
				expectedValues := map[string]string{
					"enabled-item": "new-enabled-value",
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "mixed enabled and disabled items",
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
			newValues: map[string]string{
				"enabled-item":               "new-enabled-value",
				"disabled-item":              "new-disabled-value",
				"enabled-item-with-children": "new-parent-value",
				"enabled-child":              "new-enabled-child-value",
				"disabled-child":             "new-disabled-child-value",
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(map[string]string{}, nil)
				expectedValues := map[string]string{
					"enabled-item":               "new-enabled-value",
					"enabled-item-with-children": "new-parent-value",
					"enabled-child":              "new-enabled-child-value",
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
			newValues: map[string]string{},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(map[string]string{}, nil)
				expectedValues := map[string]string{}
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
			newValues: map[string]string{
				"enabled-item": "new-value",
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(map[string]string{}, nil)
				expectedValues := map[string]string{
					"enabled-item": "new-value",
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
			newValues: map[string]string{
				"item-with-empty-when": "new-value",
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(map[string]string{}, nil)
				expectedValues := map[string]string{
					"item-with-empty-when": "new-value",
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
			newValues: map[string]string{
				"item-1": "new-value-1",
				"item-2": "new-value-2",
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(map[string]string{}, nil)
				expectedValues := map[string]string{}
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
			newValues: map[string]string{
				"enabled-item-1": "new-value-1",
				// enabled-item-2 intentionally omitted
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(map[string]string{}, nil)
				expectedValues := map[string]string{
					"enabled-item-1": "new-value-1",
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
			newValues: map[string]string{
				"item-1": "new-value-1",
				"item-2": "new-value-2",
			},
			setupMock: func(mockStore *config.MockStore) {
				existingValues := map[string]string{
					"item-1": "existing-value-1",
					"item-2": "existing-value-2",
				}
				mockStore.On("GetConfigValues").Return(existingValues, nil)
				expectedValues := map[string]string{
					"item-1": "new-value-1",
					"item-2": "new-value-2",
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
			newValues: map[string]string{
				"item-1": "new-value-1",
				// item-2 not provided, should keep existing value
				"item-3": "new-value-3",
			},
			setupMock: func(mockStore *config.MockStore) {
				existingValues := map[string]string{
					"item-1": "existing-value-1",
					"item-2": "existing-value-2",
					"item-3": "existing-value-3",
				}
				mockStore.On("GetConfigValues").Return(existingValues, nil)
				expectedValues := map[string]string{
					"item-1": "new-value-1",
					"item-2": "existing-value-2",
					"item-3": "new-value-3",
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
			newValues: map[string]string{
				"item-1": "",
				"item-2": "new-value-2",
			},
			setupMock: func(mockStore *config.MockStore) {
				existingValues := map[string]string{
					"item-1": "existing-value-1",
					"item-2": "existing-value-2",
				}
				mockStore.On("GetConfigValues").Return(existingValues, nil)
				expectedValues := map[string]string{
					"item-1": "",
					"item-2": "new-value-2",
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
			newValues: map[string]string{
				"enabled-item":  "new-enabled-value",
				"disabled-item": "new-disabled-value",
			},
			setupMock: func(mockStore *config.MockStore) {
				existingValues := map[string]string{
					"enabled-item":  "existing-enabled-value",
					"disabled-item": "existing-disabled-value",
				}
				mockStore.On("GetConfigValues").Return(existingValues, nil)
				expectedValues := map[string]string{
					"enabled-item": "new-enabled-value",
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
			newValues: map[string]string{
				"existing-item": "updated-existing-value",
				"new-item":      "brand-new-value",
			},
			setupMock: func(mockStore *config.MockStore) {
				existingValues := map[string]string{
					"existing-item": "existing-value",
					// new-item not in existing values
				}
				mockStore.On("GetConfigValues").Return(existingValues, nil)
				expectedValues := map[string]string{
					"existing-item": "updated-existing-value",
					"new-item":      "brand-new-value",
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
			manager := &appConfigManager{
				appConfigStore: mockStore,
			}

			// Call PatchConfigValues
			err := manager.PatchConfigValues(context.Background(), tt.config, tt.newValues)

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
				storeValues := map[string]string{
					"enabled-item-1": "store-value-1",
					"enabled-item-2": "store-value-2",
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
				storeValues := map[string]string{
					"enabled-item":  "enabled-store",
					"disabled-item": "disabled-store",
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
				storeValues := map[string]string{
					"item-with-empty-when": "empty-store",
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
				storeValues := map[string]string{
					"some-store-value": "store-value",
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
				storeValues := map[string]string{
					"item-1": "store-1",
					"item-2": "store-2",
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
				storeValues := map[string]string{
					"enabled-item":  "enabled-store",
					"disabled-item": "disabled-store",
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
				storeValues := map[string]string{
					"parent-item":  "parent-store",
					"child-item-1": "child-store-1",
					"child-item-2": "child-store-2",
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
				storeValues := map[string]string{
					"enabled-item":    "enabled-store",
					"non-config-item": "non-config-value",
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
				storeValues := map[string]string{
					"item-with-store": "store-value",
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
				storeValues := map[string]string{
					"item-with-empty-store":     "",
					"item-with-non-empty-store": "store-value",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock store
			mockStore := &config.MockStore{}
			tt.setupMock(mockStore)

			// Create manager with mock store
			manager := &appConfigManager{
				appConfigStore: mockStore,
			}

			// Call GetKotsadmConfigValues
			result, err := manager.GetKotsadmConfigValues(tt.config)

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
