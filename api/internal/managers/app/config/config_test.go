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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAppConfigManager_TemplateConfig(t *testing.T) {
	tests := []struct {
		name          string
		config        kotsv1beta1.Config
		configValues  types.AppConfigValues
		maskPasswords bool
		expected      types.AppConfig
	}{
		{
			name: "filtering: hardcoded and templated when, mixed delims, sprig and repl",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "hardcoded_true",
							Title: "Hardcoded True",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "item_true",
									Title: "Visible Item",
									Type:  "text",
									Value: multitype.FromString("visible"),
									When:  "true",
								},
								{
									Name:  "item_false",
									Title: "Hidden Item",
									Type:  "text",
									Value: multitype.FromString("hidden"),
									When:  "false",
								},
							},
						},
						{
							Name:  "hardcoded_false",
							Title: "Hardcoded False",
							When:  "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "item_should_not_appear",
									Title: "Should Not Appear",
									Type:  "text",
									Value: multitype.FromString("nope"),
									When:  "true",
								},
							},
						},
						{
							Name:  "templated_when",
							Title: "{{repl print \"Templated Group\"}}",
							When:  "{{repl eq \"yes\" (print \"yes\")}}",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "templated_true",
									Title: "repl{{ upper \"templated true\" }}",
									Type:  "text",
									Value: multitype.FromString("{{repl print \"ok\"}}"),
									When:  "{{repl eq \"yes\" \"yes\"}}",
								},
								{
									Name:  "templated_false",
									Title: "{{repl lower \"TEMPLATED FALSE\"}}",
									Type:  "text",
									Value: multitype.FromString("{{repl print \"no\"}}"),
									When:  "{{repl eq \"yes\" \"no\"}}",
								},
								{
									Name:  "sprig_item",
									Title: "{{repl printf \"Sprig: %d\" (add 2 3)}}",
									Type:  "text",
									Value: multitype.FromString("{{repl add 2 3}}"),
									When:  "true",
								},
							},
						},
					},
				},
			},
			expected: types.AppConfig{
				Groups: []kotsv1beta1.ConfigGroup{
					{
						Name:  "hardcoded_true",
						Title: "Hardcoded True",
						When:  "true",
						Items: []kotsv1beta1.ConfigItem{
							{
								Name:  "item_true",
								Title: "Visible Item",
								Type:  "text",
								Value: multitype.FromString("visible"),
								When:  "true",
							},
						},
					},
					{
						Name:  "templated_when",
						Title: "Templated Group",
						When:  "true",
						Items: []kotsv1beta1.ConfigItem{
							{
								Name:  "templated_true",
								Title: "TEMPLATED TRUE",
								Type:  "text",
								Value: multitype.FromString("ok"),
								When:  "true",
							},
							{
								Name:  "sprig_item",
								Title: "Sprig: 5",
								Type:  "text",
								Value: multitype.FromString("5"),
								When:  "true",
							},
						},
					},
				},
			},
		},
		{
			name: "ConfigOptionEquals in when fields",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "database",
							Title: "Database Configuration",
							When:  "{{repl ConfigOptionEquals \"db_enabled\" \"true\" }}",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "db_enabled",
									Title: "Enable Database",
									Type:  "bool",
									Value: multitype.FromString("true"),
									When:  "true",
								},
								{
									Name:  "db_type",
									Title: "Database Type",
									Type:  "select_one",
									Value: multitype.FromString("postgresql"),
									When:  "{{repl ConfigOptionEquals \"db_enabled\" \"true\" }}",
								},
								{
									Name:  "db_host",
									Title: "Database Host",
									Type:  "text",
									Value: multitype.FromString("localhost"),
									When:  "{{repl ConfigOptionEquals \"db_type\" \"postgresql\" }}",
								},
								{
									Name:  "db_port",
									Title: "Database Port",
									Type:  "text",
									Value: multitype.FromString("5432"),
									When:  "{{repl ConfigOptionEquals \"db_type\" \"mysql\" }}",
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"db_enabled": {Value: "true"},
				"db_type":    {Value: "postgresql"},
			},
			expected: types.AppConfig{
				Groups: []kotsv1beta1.ConfigGroup{
					{
						Name:  "database",
						Title: "Database Configuration",
						When:  "true",
						Items: []kotsv1beta1.ConfigItem{
							{
								Name:  "db_enabled",
								Title: "Enable Database",
								Type:  "bool",
								Value: multitype.FromString("true"),
								When:  "true",
							},
							{
								Name:  "db_type",
								Title: "Database Type",
								Type:  "select_one",
								Value: multitype.FromString("postgresql"),
								When:  "true",
							},
							{
								Name:  "db_host",
								Title: "Database Host",
								Type:  "text",
								Value: multitype.FromString("localhost"),
								When:  "true",
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
			expected: types.AppConfig{
				Groups: []kotsv1beta1.ConfigGroup{},
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
			expected: types.AppConfig{
				Groups: []kotsv1beta1.ConfigGroup{},
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
									Value: multitype.FromString("item_with_empty_when_value"),
									When:  "",
								},
								{
									Name:  "item_with_false_when",
									Title: "Item with False When",
									Type:  "text",
									Value: multitype.FromString("item_with_false_when_value"),
									When:  "false",
								},
							},
						},
					},
				},
			},
			expected: types.AppConfig{
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
								Value: multitype.FromString("item_with_empty_when_value"),
								When:  "",
							},
						},
					},
				},
			},
		},
		{
			name: "password masking enabled",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "security",
							Title: "Security",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "user",
									Title: "User",
									Type:  "text",
									Value: multitype.FromString("admin"),
									When:  "true",
								},
								{
									Name:  "password",
									Title: "Password",
									Type:  "password",
									Value: multitype.FromString("secret"),
									When:  "true",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "confirm",
											Title: "Confirm",
											Value: multitype.FromString("secret"),
										},
									},
								},
							},
						},
					},
				},
			},
			maskPasswords: true,
			expected: types.AppConfig{
				Groups: []kotsv1beta1.ConfigGroup{
					{
						Name:  "security",
						Title: "Security",
						When:  "true",
						Items: []kotsv1beta1.ConfigItem{
							{
								Name:  "user",
								Title: "User",
								Type:  "text",
								Value: multitype.FromString("admin"),
								When:  "true",
							},
							{
								Name:  "password",
								Title: "Password",
								Type:  "password",
								Value: multitype.FromString(PasswordMask),
								When:  "true",
								Items: []kotsv1beta1.ConfigChildItem{
									{
										Name:  "confirm",
										Title: "Confirm",
										Value: multitype.FromString(PasswordMask),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "password masking disabled",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "security",
							Title: "Security",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "user",
									Title: "User",
									Type:  "text",
									Value: multitype.FromString("admin"),
									When:  "true",
								},
								{
									Name:  "password",
									Title: "Password",
									Type:  "password",
									Value: multitype.FromString("secret"),
									When:  "true",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "confirm",
											Title: "Confirm",
											Value: multitype.FromString("secret"),
										},
									},
								},
							},
						},
					},
				},
			},
			maskPasswords: false,
			expected: types.AppConfig{
				Groups: []kotsv1beta1.ConfigGroup{
					{
						Name:  "security",
						Title: "Security",
						When:  "true",
						Items: []kotsv1beta1.ConfigItem{
							{
								Name:  "user",
								Title: "User",
								Type:  "text",
								Value: multitype.FromString("admin"),
								When:  "true",
							},
							{
								Name:  "password",
								Title: "Password",
								Type:  "password",
								Value: multitype.FromString("secret"),
								When:  "true",
								Items: []kotsv1beta1.ConfigChildItem{
									{
										Name:  "confirm",
										Title: "Confirm",
										Value: multitype.FromString("secret"),
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "all groups and items disabled",
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
									Value: multitype.FromString("item_in_disabled_group_value"),
									When:  "true",
								},
							},
						},
						{
							Name:  "disabled_group_2",
							Title: "Disabled Group 2",
							When:  "{{repl eq \"true\" \"false\"}}",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "another_item_in_disabled_group",
									Title: "Another Item in Disabled Group",
									Type:  "text",
									Value: multitype.FromString("another_item_in_disabled_group_value"),
									When:  "true",
								},
							},
						},
						{
							Name:  "enabled_group_with_all_disabled_items",
							Title: "Enabled Group with All Disabled Items",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "disabled_item_1",
									Title: "Disabled Item 1",
									Type:  "text",
									Value: multitype.FromString("disabled_item_1_value"),
									When:  "false",
								},
								{
									Name:  "disabled_item_2",
									Title: "Disabled Item 2",
									Type:  "text",
									Value: multitype.FromString("disabled_item_2_value"),
									When:  "{{repl eq \"true\" \"false\"}}",
								},
							},
						},
					},
				},
			},
			expected: types.AppConfig{
				Groups: []kotsv1beta1.ConfigGroup{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewAppConfigManager(tt.config)
			require.NoError(t, err)

			result, err := manager.TemplateConfig(tt.configValues, tt.maskPasswords)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
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
									Value: multitype.FromString("original-value-1"),
									When:  "true",
								},
								{
									Name:  "enabled-item-2",
									Title: "Enabled Item 2",
									Type:  "text",
									Value: multitype.FromString("original-value-2"),
									When:  "true",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "enabled-child-item",
											Title: "Enabled Child Item",
											Value: multitype.FromString("original-child-value"),
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
									Value: multitype.FromString("original-value"),
									When:  "true",
								},
								{
									Name:  "child-in-disabled-group",
									Title: "Child in Disabled Group",
									Type:  "text",
									Value: multitype.FromString("original-child-value"),
									When:  "true",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "grandchild-in-disabled-group",
											Title: "Grandchild in Disabled Group",
											Value: multitype.FromString("original-grandchild-value"),
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
									Value: multitype.FromString("original-value"),
									When:  "false",
								},
								{
									Name:  "enabled-item-in-disabled-group",
									Title: "Enabled Item in Disabled Group",
									Type:  "text",
									Value: multitype.FromString("original-enabled-value"),
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
									Value: multitype.FromString("enabled-original-value"),
									When:  "true",
								},
								{
									Name:  "disabled-item",
									Title: "Disabled Item",
									Type:  "text",
									Value: multitype.FromString("disabled-original-value"),
									When:  "false",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "child-of-disabled-item",
											Title: "Child of Disabled Item",
											Value: multitype.FromString("child-of-disabled-original-value"),
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
									Value: multitype.FromString("enabled-original"),
									When:  "true",
								},
								{
									Name:  "disabled-item",
									Title: "Disabled Item",
									Type:  "text",
									Value: multitype.FromString("disabled-original"),
									When:  "false",
								},
								{
									Name:  "enabled-item-with-children",
									Title: "Enabled Item with Children",
									Type:  "group",
									Value: multitype.FromString("parent-original"),
									When:  "true",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:  "enabled-child",
											Title: "Enabled Child",
											Value: multitype.FromString("enabled-child-original"),
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
									Value: multitype.FromString("original-1"),
									When:  "true",
								},
								{
									Name:  "disabled-item-1",
									Title: "Disabled Item 1",
									Type:  "text",
									Value: multitype.FromString("original-disabled-1"),
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
									Value: multitype.FromString("original-disabled-group"),
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
									Value: multitype.FromString("original-2"),
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
									Value: multitype.FromString("original-value"),
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
									Value: multitype.FromString("original-value"),
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
									Value: multitype.FromString("original-value"),
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
									Value: multitype.FromString("original-1"),
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
									Value: multitype.FromString("original-2"),
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
									Value: multitype.FromString("original-value-1"),
									When:  "true",
								},
								{
									Name:  "enabled-item-2",
									Title: "Enabled Item 2",
									Type:  "text",
									Value: multitype.FromString("original-value-2"),
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
									Value: multitype.FromString("original-1"),
									When:  "true",
								},
								{
									Name:  "item-2",
									Title: "Item 2",
									Type:  "text",
									Value: multitype.FromString("original-2"),
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
									Value: multitype.FromString("original-1"),
									When:  "true",
								},
								{
									Name:  "item-2",
									Title: "Item 2",
									Type:  "text",
									Value: multitype.FromString("original-2"),
									When:  "true",
								},
								{
									Name:  "item-3",
									Title: "Item 3",
									Type:  "text",
									Value: multitype.FromString("original-3"),
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
									Value: multitype.FromString("original-1"),
									When:  "true",
								},
								{
									Name:  "item-2",
									Title: "Item 2",
									Type:  "text",
									Value: multitype.FromString("original-2"),
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
									Value: multitype.FromString("original-enabled"),
									When:  "true",
								},
								{
									Name:  "disabled-item",
									Title: "Disabled Item",
									Type:  "text",
									Value: multitype.FromString("original-disabled"),
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
									Value: multitype.FromString("original-existing"),
									When:  "true",
								},
								{
									Name:  "new-item",
									Title: "New Item",
									Type:  "text",
									Value: multitype.FromString("original-new"),
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
									Value: multitype.FromString("original-1"),
									When:  "false",
								},
								{
									Name:  "disabled-item-2",
									Title: "Disabled Item 2",
									Type:  "text",
									Value: multitype.FromString("original-2"),
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
							Title: "repl{{ upper \"http configuration\" }}",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:  "templated-enabled-item",
									Title: "{{repl printf \"Port: %d\" 8080 }}",
									Type:  "text",
									When:  "true",
								},
								{
									Name:  "templated-disabled-item",
									Title: "repl{{ lower \"DISABLED ITEM\" }}",
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
		{
			name: "file items with filename field",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "file-group",
							Title: "File Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "file-with-filename",
									Title:    "File with Filename",
									Type:     "file",
									Filename: "config.yaml",
									When:     "true",
								},
								{
									Name:  "file-without-filename",
									Title: "File without Filename",
									Type:  "file",
									When:  "true",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"file-with-filename":    types.AppConfigValue{Value: "SGVsbG8gV29ybGQ=", Filename: "custom.yaml"},
				"file-without-filename": types.AppConfigValue{Value: "VGVzdCBDb250ZW50"},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{
					"file-with-filename":    types.AppConfigValue{Value: "SGVsbG8gV29ybGQ=", Filename: "custom.yaml"},
					"file-without-filename": types.AppConfigValue{Value: "VGVzdCBDb250ZW50"},
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "file items with existing values and filename merging",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "file-group",
							Title: "File Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "file-item-1",
									Title:    "File Item 1",
									Type:     "file",
									Filename: "default-1.txt",
									When:     "true",
								},
								{
									Name:     "file-item-2",
									Title:    "File Item 2",
									Type:     "file",
									Filename: "default-2.txt",
									When:     "true",
								},
								{
									Name:  "file-item-3",
									Title: "File Item 3",
									Type:  "file",
									When:  "true",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"file-item-1": types.AppConfigValue{Value: "TmV3IENvbnRlbnQ=", Filename: "new-1.txt"},     // update both value and filename
				"file-item-3": types.AppConfigValue{Value: "VGhpcmQgQ29udGVudA==", Filename: "new-3.txt"}, // update item 3
				// file-item-2 not provided, should keep existing value
			},
			setupMock: func(mockStore *config.MockStore) {
				existingValues := types.AppConfigValues{
					"file-item-1": types.AppConfigValue{Value: "T2xkIENvbnRlbnQ=", Filename: "old-1.txt"},
					"file-item-2": types.AppConfigValue{Value: "U2Vjb25kIENvbnRlbnQ=", Filename: "existing-2.txt"},
				}
				mockStore.On("GetConfigValues").Return(existingValues, nil)
				expectedValues := types.AppConfigValues{
					"file-item-1": types.AppConfigValue{Value: "TmV3IENvbnRlbnQ=", Filename: "new-1.txt"},          // new values override existing
					"file-item-2": types.AppConfigValue{Value: "U2Vjb25kIENvbnRlbnQ=", Filename: "existing-2.txt"}, // existing values preserved
					"file-item-3": types.AppConfigValue{Value: "VGhpcmQgQ29udGVudA==", Filename: "new-3.txt"},      // new values added
				}
				mockStore.On("SetConfigValues", expectedValues).Return(nil)
			},
			wantErr: false,
		},
		{
			name: "file items in disabled groups are filtered out",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "enabled-file",
									Title:    "Enabled File",
									Type:     "file",
									Filename: "enabled.txt",
									When:     "true",
								},
							},
						},
						{
							Name:  "disabled-group",
							Title: "Disabled Group",
							When:  "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "disabled-file",
									Title:    "Disabled File",
									Type:     "file",
									Filename: "disabled.txt",
									When:     "true",
								},
							},
						},
					},
				},
			},
			newValues: types.AppConfigValues{
				"enabled-file":  types.AppConfigValue{Value: "RW5hYmxlZCBDb250ZW50", Filename: "enabled.txt"},
				"disabled-file": types.AppConfigValue{Value: "RGlzYWJsZWQgQ29udGVudA==", Filename: "disabled.txt"},
			},
			setupMock: func(mockStore *config.MockStore) {
				mockStore.On("GetConfigValues").Return(types.AppConfigValues{}, nil)
				expectedValues := types.AppConfigValues{
					"enabled-file": types.AppConfigValue{Value: "RW5hYmxlZCBDb250ZW50", Filename: "enabled.txt"},
					// disabled-file should be filtered out
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
									Value:   multitype.FromString("default-value-1"),
									Default: multitype.FromString("default-value-1"),
									When:    "true",
								},
								{
									Name:    "enabled-item-2",
									Title:   "Enabled Item 2",
									Type:    "text",
									Value:   multitype.FromString("default-value-2"),
									Default: multitype.FromString("default-value-2"),
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
									Value:   multitype.FromString("enabled-default"),
									Default: multitype.FromString("enabled-default"),
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
									Value:   multitype.FromString("disabled-default"),
									Default: multitype.FromString("disabled-default"),
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
									Value:   multitype.FromString("empty-default"),
									Default: multitype.FromString("empty-default"),
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
									Value:   multitype.FromString("default-1"),
									Default: multitype.FromString("default-1"),
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
									Value:   multitype.FromString("default-2"),
									Default: multitype.FromString("default-2"),
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
									Value:   multitype.FromString("default-value"),
									Default: multitype.FromString("default-value"),
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
									Value:   multitype.FromString("enabled-default"),
									Default: multitype.FromString("enabled-default"),
									When:    "true",
								},
								{
									Name:    "disabled-item",
									Title:   "Disabled Item",
									Type:    "text",
									Value:   multitype.FromString("disabled-default"),
									Default: multitype.FromString("disabled-default"),
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
									Value:   multitype.FromString("parent-default"),
									Default: multitype.FromString("parent-default"),
									When:    "true",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:    "child-item-1",
											Title:   "Child Item 1",
											Value:   multitype.FromString("child-default-1"),
											Default: multitype.FromString("child-default-1"),
										},
										{
											Name:    "child-item-2",
											Title:   "Child Item 2",
											Value:   multitype.FromString("child-default-2"),
											Default: multitype.FromString("child-default-2"),
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
									Value:   multitype.FromString("enabled-default"),
									Default: multitype.FromString("enabled-default"),
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
									Value:   multitype.FromString("default-with-store"),
									Default: multitype.FromString("default-with-store"),
									When:    "true",
								},
								{
									Name:    "item-without-store",
									Title:   "Item without Store",
									Type:    "text",
									Value:   multitype.FromString("default-without-store"),
									Default: multitype.FromString("default-without-store"),
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
									Value:   multitype.FromString("config-value"),
									Default: multitype.FromString("config-default"),
									When:    "true",
								},
								{
									Name:    "item-with-non-empty-store",
									Title:   "Item with Non-Empty Store",
									Type:    "text",
									Value:   multitype.FromString("config-value-2"),
									Default: multitype.FromString("config-default-2"),
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
									Value:   multitype.FromString("admin"),
									Default: multitype.FromString("user"),
									When:    "true",
								},
								{
									Name:    "password",
									Title:   "Password",
									Type:    "password",
									Value:   multitype.FromString("schema-password"),
									Default: multitype.FromString("default-password"),
									When:    "true",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:    "confirm-password",
											Title:   "Confirm Password",
											Value:   multitype.FromString("schema-confirm-password"),
											Default: multitype.FromString("default-confirm-password"),
										},
										{
											Name:    "password-hint",
											Title:   "Password Hint",
											Value:   multitype.FromString("schema-password-hint"),
											Default: multitype.FromString("default-password-hint"),
										},
									},
								},
								{
									Name:    "api-key",
									Title:   "API Key",
									Type:    "password",
									Value:   multitype.FromString("schema-api-key"),
									Default: multitype.FromString("default-api-key"),
									When:    "true",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:    "api-secret",
											Title:   "API Secret",
											Value:   multitype.FromString("schema-api-secret"),
											Default: multitype.FromString("default-api-secret"),
										},
									},
								},
								{
									Name:    "secret-token",
									Title:   "Secret Token",
									Type:    "password",
									Value:   multitype.FromString("schema-token"),
									Default: multitype.FromString("default-token"),
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
							Title: "{{repl title \"server configuration\" }}",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:    "templated-item",
									Title:   "repl{{ upper \"http port\" }}",
									Type:    "text",
									Value:   multitype.FromString("{{repl toString 8080 }}"),
									Default: multitype.FromString("{{repl toString 8080 }}"),
									When:    "true",
								},
								{
									Name:    "sprig-functions-item",
									Title:   "repl{{ printf \"Port: %d\" 9090 }}",
									Type:    "text",
									Value:   multitype.FromString("{{repl add 9000 90 }}"),
									Default: multitype.FromString("{{repl add 9000 90 }}"),
									When:    "true",
								},
								{
									Name:    "disabled-templated-item",
									Title:   "{{repl lower \"DISABLED ITEM\" }}",
									Type:    "text",
									Value:   multitype.FromString("repl{{ trim \"  disabled  \" }}"),
									Default: multitype.FromString("{{repl trim \"  disabled  \" }}"),
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
		{
			name: "file items with filename field",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "file-group",
							Title: "File Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "file-with-filename",
									Title:    "File with Filename",
									Type:     "file",
									Value:    multitype.FromString("RGVmYXVsdCBDb250ZW50"), // "Default Conten)"
									Default:  multitype.FromString("RGVmYXVsdCBDb250ZW50"), // "Default Conten)"
									Filename: "config.yaml",
									When:     "true",
								},
								{
									Name:    "file-without-filename",
									Title:   "File without Filename",
									Type:    "file",
									Value:   multitype.FromString("QW5vdGhlciBEZWZhdWx0"), // "Another Defaul)"
									Default: multitype.FromString("QW5vdGhlciBEZWZhdWx0"), // "Another Defaul)"
									When:    "true",
								},
							},
						},
					},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				storeValues := types.AppConfigValues{
					"file-with-filename":    types.AppConfigValue{Value: "SGVsbG8gV29ybGQ=", Filename: "custom.yaml"},
					"file-without-filename": types.AppConfigValue{Value: "VGVzdCBDb250ZW50"},
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
						"file-with-filename": {
							Value:    "SGVsbG8gV29ybGQ=", // store value overrides schema value
							Default:  "RGVmYXVsdCBDb250ZW50",
							Filename: "custom.yaml", // store filename overrides schema filename
						},
						"file-without-filename": {
							Value:   "VGVzdCBDb250ZW50", // store value overrides schema value
							Default: "QW5vdGhlciBEZWZhdWx0",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "file items with child items and filename handling",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "file-group",
							Title: "File Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "parent-file",
									Title:    "Parent File",
									Type:     "file",
									Value:    multitype.FromString("UGFyZW50IERlZmF1bHQ="), // "Parent Defaul)"
									Default:  multitype.FromString("UGFyZW50IERlZmF1bHQ="), // "Parent Defaul)"
									Filename: "parent.txt",
									When:     "true",
									Items: []kotsv1beta1.ConfigChildItem{
										{
											Name:    "child-file",
											Title:   "Child File",
											Value:   multitype.FromString("Q2hpbGQgRGVmYXVsdA=="), // "Child Defaul)"
											Default: multitype.FromString("Q2hpbGQgRGVmYXVsdA=="), // "Child Defaul)"
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
					"parent-file": types.AppConfigValue{Value: "TmV3IFBhcmVudA==", Filename: "new-parent.txt"},
					"child-file":  types.AppConfigValue{Value: "TmV3IENoaWxk", Filename: "new-child.txt"},
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
						"parent-file": {
							Value:    "TmV3IFBhcmVudA==",
							Default:  "UGFyZW50IERlZmF1bHQ=",
							Filename: "new-parent.txt", // child inherits parent's filename behavior
						},
						"child-file": {
							Value:    "TmV3IENoaWxk",
							Default:  "Q2hpbGQgRGVmYXVsdA==",
							Filename: "new-child.txt", // child inherits parent's filename behavior
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "file items without store values use defaults",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "file-group",
							Title: "File Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "file-with-store",
									Title:    "File with Store",
									Type:     "file",
									Value:    multitype.FromString("RGVmYXVsdCBXaXRoIFN0b3Jl"), // "Default With Stor)"
									Default:  multitype.FromString("RGVmYXVsdCBXaXRoIFN0b3Jl"), // "Default With Stor)"
									Filename: "with-store.txt",
									When:     "true",
								},
								{
									Name:     "file-without-store",
									Title:    "File without Store",
									Type:     "file",
									Value:    multitype.FromString("RGVmYXVsdCBXaXRob3V0IFN0b3Jl"), // "Default Without Stor)"
									Default:  multitype.FromString("RGVmYXVsdCBXaXRob3V0IFN0b3Jl"), // "Default Without Stor)"
									Filename: "without-store.txt",
									When:     "true",
								},
							},
						},
					},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				storeValues := types.AppConfigValues{
					"file-with-store": types.AppConfigValue{Value: "U3RvcmVkIFZhbHVl", Filename: "stored.txt"},
					// file-without-store is not in store
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
						"file-with-store": {
							Value:    "U3RvcmVkIFZhbHVl",
							Default:  "RGVmYXVsdCBXaXRoIFN0b3Jl",
							Filename: "stored.txt",
						},
						"file-without-store": {
							Value:    "RGVmYXVsdCBXaXRob3V0IFN0b3Jl", // uses schema value
							Default:  "RGVmYXVsdCBXaXRob3V0IFN0b3Jl",
							Filename: "without-store.txt", // uses schema filename
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "file items in disabled groups are filtered out",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name:  "enabled-group",
							Title: "Enabled Group",
							When:  "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "enabled-file",
									Title:    "Enabled File",
									Type:     "file",
									Value:    multitype.FromString("RW5hYmxlZCBEZWZhdWx0"), // "Enabled Defaul)"
									Default:  multitype.FromString("RW5hYmxlZCBEZWZhdWx0"), // "Enabled Defaul)"
									Filename: "enabled.txt",
									When:     "true",
								},
							},
						},
						{
							Name:  "disabled-group",
							Title: "Disabled Group",
							When:  "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "disabled-file",
									Title:    "Disabled File",
									Type:     "file",
									Value:    multitype.FromString("RGlzYWJsZWQgRGVmYXVsdA=="), // "Disabled Defaul)"
									Default:  multitype.FromString("RGlzYWJsZWQgRGVmYXVsdA=="), // "Disabled Defaul)"
									Filename: "disabled.txt",
									When:     "true",
								},
							},
						},
					},
				},
			},
			setupMock: func(mockStore *config.MockStore) {
				storeValues := types.AppConfigValues{
					"enabled-file":  types.AppConfigValue{Value: "RW5hYmxlZCBTdG9yZQ==", Filename: "enabled-store.txt"},
					"disabled-file": types.AppConfigValue{Value: "RGlzYWJsZWQgU3RvcmU=", Filename: "disabled-store.txt"},
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
						"enabled-file": {
							Value:    "RW5hYmxlZCBTdG9yZQ==",
							Default:  "RW5hYmxlZCBEZWZhdWx0",
							Filename: "enabled-store.txt",
						},
						// disabled-file should not be present due to disabled group
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
								},
								{
									Name:     "required_password",
									Required: true,
									Type:     "password",
									Value:    multitype.BoolOrString{},
									Default:  multitype.BoolOrString{},
								},
								{
									Name:     "optional_item",
									Required: false,
								},
								{
									Name:     "optional_password",
									Required: false,
									Type:     "password",
									Value:    multitype.BoolOrString{},
									Default:  multitype.BoolOrString{},
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"required_item":     types.AppConfigValue{Value: "value1"},
				"optional_item":     types.AppConfigValue{Value: "value2"},
				"required_password": types.AppConfigValue{Value: "password1"},
				"optional_password": types.AppConfigValue{Value: "password2"},
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
									Default:  multitype.FromString("default_value"),
								},
								{
									Name:     "required_password_with_default",
									Required: true,
									Type:     "password",
									Value:    multitype.BoolOrString{},
									Default:  multitype.BoolOrString{StrVal: "default_password"},
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
									Value:    multitype.FromString("item_value"),
								},
								{
									Name:     "required_password_with_value",
									Required: true,
									Type:     "password",
									Value:    multitype.BoolOrString{StrVal: "password_value"},
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
									Value:    multitype.FromString("item_value"),
								},
								{
									Name:     "required_password_with_value",
									Required: true,
									Type:     "password",
									Value:    multitype.BoolOrString{StrVal: "password_value"},
									Default:  multitype.BoolOrString{},
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"required_with_value":          types.AppConfigValue{Value: ""},
				"required_password_with_value": types.AppConfigValue{Value: ""},
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
								},
								{
									Name:     "hidden_password_required",
									Required: true,
									Hidden:   true,
									Type:     "password",
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
								},
								{
									Name:     "disabled_password_required",
									Required: true,
									When:     "false",
									Type:     "password",
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
								},
								{
									Name:     "required_item2",
									Required: true,
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
		{
			name: "file items with valid base64 values",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "file_group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name: "valid_file_item",
									Type: "file",
								},
								{
									Name: "another_valid_file",
									Type: "file",
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"valid_file_item":    types.AppConfigValue{Value: "SGVsbG8gV29ybGQ="}, // "Hello World" in base64
				"another_valid_file": types.AppConfigValue{Value: "VGVzdCBDb250ZW50"}, // "Test Content" in base64
			},
			wantErr: false,
		},
		{
			name: "file items with invalid base64 values",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "file_group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name: "invalid_file_item",
									Type: "file",
								},
								{
									Name: "another_invalid_file",
									Type: "file",
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"invalid_file_item":    types.AppConfigValue{Value: "not-base64-content!"}, // invalid base64
				"another_invalid_file": types.AppConfigValue{Value: "also-not-base64@#"},   // invalid base64
			},
			wantErr:     true,
			errorFields: []string{"invalid_file_item", "another_invalid_file"},
		},
		{
			name: "file items with empty values should be valid",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "file_group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name: "empty_file_item",
									Type: "file",
								},
								{
									Name: "missing_file_item",
									Type: "file",
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"empty_file_item": types.AppConfigValue{Value: ""}, // empty value should be valid
				// missing_file_item is not provided, should be valid
			},
			wantErr: false,
		},
		{
			name: "mixed file and non-file items with validation errors",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "mixed_group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "required_text",
									Type:     "text",
									Required: true,
								},
								{
									Name: "invalid_file",
									Type: "file",
								},
								{
									Name: "valid_file",
									Type: "file",
								},
								{
									Name: "text_field",
									Type: "text",
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				// required_text is missing - should cause required error
				"invalid_file": types.AppConfigValue{Value: "not-base64!"},      // invalid base64
				"valid_file":   types.AppConfigValue{Value: "VmFsaWQgYmFzZTY0"}, // "Valid base64" in base64
				"text_field":   types.AppConfigValue{Value: "any text is fine"}, // text field doesn't need base64
			},
			wantErr:     true,
			errorFields: []string{"required_text", "invalid_file"},
		},
		{
			name: "file items in disabled groups should not be validated",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "disabled_group",
							When: "false",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name: "file_in_disabled_group",
									Type: "file",
								},
							},
						},
						{
							Name: "enabled_group",
							When: "true",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name: "file_in_enabled_group",
									Type: "file",
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"file_in_disabled_group": types.AppConfigValue{Value: "not-base64-but-disabled"}, // should not be validated
				"file_in_enabled_group":  types.AppConfigValue{Value: "also-not-base64"},         // should be validated
			},
			wantErr:     true,
			errorFields: []string{"file_in_enabled_group"},
		},
		{
			name: "disabled file items should not be validated",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "file_group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name: "disabled_file_item",
									Type: "file",
									When: "false",
								},
								{
									Name: "enabled_file_item",
									Type: "file",
									When: "true",
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"disabled_file_item": types.AppConfigValue{Value: "not-base64-but-disabled"}, // should not be validated
				"enabled_file_item":  types.AppConfigValue{Value: "also-not-base64"},         // should be validated
			},
			wantErr:     true,
			errorFields: []string{"enabled_file_item"},
		},
		{
			name: "file items with edge case base64 values",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "file_group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name: "padding_file",
									Type: "file",
								},
								{
									Name: "no_padding_file",
									Type: "file",
								},
								{
									Name: "single_char_file",
									Type: "file",
								},
								{
									Name: "url_safe_file",
									Type: "file",
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				"padding_file":     types.AppConfigValue{Value: "SGVsbG8="},         // with padding
				"no_padding_file":  types.AppConfigValue{Value: "SGVsbG8"},          // without padding - should be invalid
				"single_char_file": types.AppConfigValue{Value: "QQ=="},             // single character "A"
				"url_safe_file":    types.AppConfigValue{Value: "SGVsbG8gV29ybGQ="}, // standard base64
			},
			wantErr:     true,
			errorFields: []string{"no_padding_file"}, // only no_padding_file should fail
		},
		{
			name: "required file items with invalid base64 should have both errors",
			config: kotsv1beta1.Config{
				Spec: kotsv1beta1.ConfigSpec{
					Groups: []kotsv1beta1.ConfigGroup{
						{
							Name: "file_group",
							Items: []kotsv1beta1.ConfigItem{
								{
									Name:     "required_file",
									Type:     "file",
									Required: true,
								},
								{
									Name:     "required_file_with_invalid_base64",
									Type:     "file",
									Required: true,
								},
							},
						},
					},
				},
			},
			configValues: types.AppConfigValues{
				// required_file is missing - should cause required error
				"required_file_with_invalid_base64": types.AppConfigValue{Value: "not-base64!"}, // invalid base64
			},
			wantErr:     true,
			errorFields: []string{"required_file", "required_file_with_invalid_base64"},
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
