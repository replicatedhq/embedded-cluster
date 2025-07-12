package config

import (
	"errors"
	"testing"

	"github.com/replicatedhq/embedded-cluster/api/internal/store/app/config"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tiendc/go-deepcopy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/replicatedhq/embedded-cluster/api/types"
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

func TestAppConfigManager_SetConfigValues(t *testing.T) {
	tests := []struct {
		name         string
		config       kotsv1beta1.Config
		configValues map[string]string
		setupMock    func(*config.MockStore)
		wantErr      bool
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
			configValues: map[string]string{
				"enabled-item-1":     "new-value-1",
				"enabled-item-2":     "new-value-2",
				"enabled-child-item": "new-child-value",
			},
			setupMock: func(mockStore *config.MockStore) {
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
			configValues: map[string]string{
				"item-in-disabled-group":       "new-value",
				"child-in-disabled-group":      "new-child-value",
				"grandchild-in-disabled-group": "new-grandchild-value",
			},
			setupMock: func(mockStore *config.MockStore) {
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
			configValues: map[string]string{
				"enabled-item":           "new-enabled-value",
				"disabled-item":          "new-disabled-value",
				"child-of-disabled-item": "new-child-value",
			},
			setupMock: func(mockStore *config.MockStore) {
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
			configValues: map[string]string{
				"enabled-item":               "new-enabled-value",
				"disabled-item":              "new-disabled-value",
				"enabled-item-with-children": "new-parent-value",
				"enabled-child":              "new-enabled-child-value",
				"disabled-child":             "new-disabled-child-value",
			},
			setupMock: func(mockStore *config.MockStore) {
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
			configValues: map[string]string{},
			setupMock: func(mockStore *config.MockStore) {
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
			configValues: map[string]string{
				"enabled-item": "new-value",
			},
			setupMock: func(mockStore *config.MockStore) {
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
			configValues: map[string]string{
				"item-with-empty-when": "new-value",
			},
			setupMock: func(mockStore *config.MockStore) {
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
			configValues: map[string]string{
				"item-1": "new-value-1",
				"item-2": "new-value-2",
			},
			setupMock: func(mockStore *config.MockStore) {
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
			configValues: map[string]string{
				"enabled-item-1": "new-value-1",
				// enabled-item-2 intentionally omitted
			},
			setupMock: func(mockStore *config.MockStore) {
				expectedValues := map[string]string{
					"enabled-item-1": "new-value-1",
					// enabled-item-2 should not be included since no value provided
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

			// Call SetConfigValues
			err := manager.SetConfigValues(tt.config, tt.configValues)

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

func TestValidateConfigValues(t *testing.T) {
	tests := []struct {
		name         string
		config       kotsv1beta1.Config
		configValues map[string]string
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
			configValues: map[string]string{
				"required_item": "value1",
				"optional_item": "value2",
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
			configValues: map[string]string{
				"optional_item": "value1",
			},
			wantErr:     true,
			errorFields: []string{"required_item"},
		},
		{
			name: "unknown config value",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "group1",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "known_item",
									Required: false,
									Value:    multitype.BoolOrString{},
									Default:  multitype.BoolOrString{},
								},
							},
						},
					},
				},
			},
			configValues: map[string]string{
				"known_item":   "value1",
				"unknown_item": "value2",
			},
			wantErr:     true,
			errorFields: []string{"unknown_item"},
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
			configValues: map[string]string{},
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
			configValues: map[string]string{},
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
			configValues: map[string]string{
				"required_with_value": "",
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
			configValues: map[string]string{},
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
			configValues: map[string]string{},
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
			configValues: map[string]string{
				"child_item": "child_value",
			},
			wantErr: false,
		},
		{
			name: "unknown child item",
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
											Name: "known_child",
										},
									},
								},
							},
						},
					},
				},
			},
			configValues: map[string]string{
				"known_child":   "value1",
				"unknown_child": "value2",
			},
			wantErr:     true,
			errorFields: []string{"unknown_child"},
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
			configValues: map[string]string{
				"unknown_item1": "value1",
				"unknown_item2": "value2",
			},
			wantErr:     true,
			errorFields: []string{"required_item1", "required_item2", "unknown_item1", "unknown_item2"},
		},
		{
			name: "empty config and values",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{},
				},
			},
			configValues: map[string]string{},
			wantErr:      false,
		},
		{
			name: "empty config with unknown values",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{},
				},
			},
			configValues: map[string]string{
				"unknown_item": "value1",
			},
			wantErr:     true,
			errorFields: []string{"unknown_item"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a real appConfigManager instance for testing
			manager := &appConfigManager{}

			// Run the validation
			err := manager.ValidateConfigValues(tt.config, tt.configValues)

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
