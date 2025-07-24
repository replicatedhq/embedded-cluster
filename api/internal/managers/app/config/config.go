package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/tiendc/go-deepcopy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kyaml "sigs.k8s.io/yaml"
)

const (
	// PasswordMask is the string used to mask password values in config responses
	PasswordMask = "••••••••"

	// Validation error messages
	DefaultValidationErrorMessage = "field errors"
	RequiredFieldsErrorMessage    = "required fields not completed"
)

var (
	// ErrValueNotBase64Encoded is returned when a file item value is not base64 encoded
	ErrValueNotBase64Encoded = errors.New("value must be base64 encoded for file items")
)

// TemplateConfig templates the config with provided values and returns the templated config
func (m *appConfigManager) TemplateConfig(newValues types.AppConfigValues, maskPasswords bool) (types.AppConfig, error) {
	// Get current config values from the store
	storedValues, err := m.appConfigStore.GetConfigValues()
	if err != nil {
		return types.AppConfig{}, fmt.Errorf("get stored config values: %w", err)
	}

	// Merge the provided values with the stored values
	// The provided values take precedence
	mergedValues := make(types.AppConfigValues)
	maps.Copy(mergedValues, storedValues)
	maps.Copy(mergedValues, newValues)

	// Execute the config template with the merged values
	processedYAML, err := m.executeConfigTemplate(mergedValues)
	if err != nil {
		return types.AppConfig{}, fmt.Errorf("execute config template: %w", err)
	}

	// Parse to Config struct
	var processedConfig kotsv1beta1.Config
	if err := kyaml.Unmarshal([]byte(processedYAML), &processedConfig); err != nil {
		return types.AppConfig{}, fmt.Errorf("unmarshal processed config: %w", err)
	}

	// Filter out disabled groups and items
	filteredConfig, err := filterAppConfig(processedConfig)
	if err != nil {
		return types.AppConfig{}, fmt.Errorf("filter app config: %w", err)
	}

	// Mask password fields if requested
	if maskPasswords {
		for i, group := range filteredConfig.Spec.Groups {
			for j, item := range group.Items {
				if item.Type == "password" {
					if filteredConfig.Spec.Groups[i].Items[j].Value.String() != "" {
						filteredConfig.Spec.Groups[i].Items[j].Value = multitype.FromString(PasswordMask)
					}
					// also mask child items
					for k := range item.Items {
						if filteredConfig.Spec.Groups[i].Items[j].Items[k].Value.String() != "" {
							filteredConfig.Spec.Groups[i].Items[j].Items[k].Value = multitype.FromString(PasswordMask)
						}
					}
				}
			}
		}
	}

	return types.AppConfig(filteredConfig.Spec), nil
}

func (m *appConfigManager) ValidateConfigValues(configValues types.AppConfigValues) error {
	var ve *types.APIError

	processedConfig, err := m.TemplateConfig(configValues, false)
	if err != nil {
		return fmt.Errorf("template config: %w", err)
	}

	for _, group := range processedConfig.Groups {
		for _, item := range group.Items {
			configValue := getConfigValueFromItem(item, configValues)
			// check required items
			if isRequiredItem(item) && isUnsetItem(configValue) {
				fieldError := createRequiredFieldError(item)
				ve = types.AppendFieldError(ve, item.Name, fieldError)
			}
			// check value is base64 encoded for file items
			if isFileType(item) && !isValueBase64Encoded(configValue) {
				ve = types.AppendFieldError(ve, item.Name, ErrValueNotBase64Encoded)
			}
		}
	}

	// Set appropriate message based on error types
	if ve != nil && len(ve.Errors) > 0 {
		// Check if ALL errors are about required fields
		allRequired := true
		for _, err := range ve.Errors {
			if !strings.Contains(err.Message, "is required") {
				allRequired = false
				break
			}
		}

		if allRequired {
			ve.Message = RequiredFieldsErrorMessage
		} else {
			ve.Message = DefaultValidationErrorMessage
		}
	}

	return ve.ErrorOrNil()
}

// PatchConfigValues performs a partial update by merging new values with existing ones
func (m *appConfigManager) PatchConfigValues(newValues types.AppConfigValues) error {
	// Get existing values
	existingValues, err := m.appConfigStore.GetConfigValues()
	if err != nil {
		return fmt.Errorf("get config values: %w", err)
	}

	// Get processed config to determine enabled groups and items
	processedConfig, err := m.TemplateConfig(newValues, false)
	if err != nil {
		return fmt.Errorf("template config: %w", err)
	}

	// Merge new values with existing ones
	mergedValues := make(types.AppConfigValues)
	maps.Copy(mergedValues, existingValues)
	maps.Copy(mergedValues, newValues)

	// only keep values for enabled groups and items
	filteredValues := make(types.AppConfigValues)
	for _, g := range processedConfig.Groups {
		// skip the group if it is not enabled
		if !isItemEnabled(g.When) {
			continue
		}
		for _, i := range g.Items {
			// skip the item if it is not enabled
			if !isItemEnabled(i.When) {
				continue
			}
			value, ok := mergedValues[i.Name]
			if ok {
				filteredValues[i.Name] = value
			}
			for _, c := range i.Items {
				value, ok := mergedValues[c.Name]
				if ok {
					filteredValues[c.Name] = value
				}
			}
		}
	}

	return m.appConfigStore.SetConfigValues(filteredValues)
}

func (m *appConfigManager) GetConfigValues() (types.AppConfigValues, error) {
	return m.appConfigStore.GetConfigValues()
}

func (m *appConfigManager) GetKotsadmConfigValues() (kotsv1beta1.ConfigValues, error) {
	processedConfig, err := m.TemplateConfig(nil, false)
	if err != nil {
		return kotsv1beta1.ConfigValues{}, fmt.Errorf("template config: %w", err)
	}

	storedValues, err := m.appConfigStore.GetConfigValues()
	if err != nil {
		return kotsv1beta1.ConfigValues{}, fmt.Errorf("get config values: %w", err)
	}

	kotsadmConfigValues := kotsv1beta1.ConfigValues{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kots.io/v1beta1",
			Kind:       "ConfigValues",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kots-app-config",
		},
		Spec: kotsv1beta1.ConfigValuesSpec{
			Values: make(map[string]kotsv1beta1.ConfigValue),
		},
	}

	// add values from the processed config
	for _, group := range processedConfig.Groups {
		for _, item := range group.Items {
			kotsadmConfigValues.Spec.Values[item.Name] = getConfigValueFromItem(item, storedValues)

			for _, childItem := range item.Items {
				kotsadmConfigValues.Spec.Values[childItem.Name] = getConfigValueFromChildItem(item, childItem, storedValues)
			}
		}
	}

	return kotsadmConfigValues, nil
}

func getConfigValueFromItem(item kotsv1beta1.ConfigItem, configValues types.AppConfigValues) kotsv1beta1.ConfigValue {
	configValue := kotsv1beta1.ConfigValue{
		Default:  item.Default.String(),
		Filename: item.Filename,
	}

	// Set value based on item type
	if item.Type == "password" {
		configValue.ValuePlaintext = item.Value.String()
	} else {
		configValue.Value = item.Value.String()
	}

	// Apply stored value override if it exists
	if v, ok := configValues[item.Name]; ok {
		if item.Type == "password" {
			configValue.ValuePlaintext = v.Value
		} else {
			configValue.Value = v.Value
		}
		configValue.Filename = v.Filename
	}

	return configValue
}

func getConfigValueFromChildItem(item kotsv1beta1.ConfigItem, childItem kotsv1beta1.ConfigChildItem, configValues types.AppConfigValues) kotsv1beta1.ConfigValue {
	configValue := kotsv1beta1.ConfigValue{
		Default:  childItem.Default.String(),
		Filename: item.Filename,
	}

	// Set value based on parent item type
	if item.Type == "password" {
		configValue.ValuePlaintext = childItem.Value.String()
	} else {
		configValue.Value = childItem.Value.String()
	}

	// Apply stored value override if it exists
	if v, ok := configValues[childItem.Name]; ok {
		if item.Type == "password" {
			configValue.ValuePlaintext = v.Value
		} else {
			configValue.Value = v.Value
		}
		configValue.Filename = v.Filename
	}

	return configValue
}

// filterAppConfig filters out disabled groups and items based on their 'when' condition
func filterAppConfig(config kotsv1beta1.Config) (kotsv1beta1.Config, error) {
	// deepcopy the config to avoid mutating the original config
	var updatedConfig kotsv1beta1.Config
	if err := deepcopy.Copy(&updatedConfig, &config); err != nil {
		return kotsv1beta1.Config{}, fmt.Errorf("deepcopy: %w", err)
	}

	filteredGroups := make([]kotsv1beta1.ConfigGroup, 0)

	for _, group := range config.Spec.Groups {
		if !isItemEnabled(group.When) {
			continue
		}
		filteredItems := make([]kotsv1beta1.ConfigItem, 0)
		for _, item := range group.Items {
			if !isItemEnabled(item.When) {
				continue
			}
			filteredItems = append(filteredItems, item)
		}
		if len(filteredItems) > 0 {
			group.Items = filteredItems
			filteredGroups = append(filteredGroups, group)
		}
	}
	updatedConfig.Spec.Groups = filteredGroups
	return updatedConfig, nil
}

// isItemEnabled checks if an item is enabled based on its 'when' condition
func isItemEnabled(when multitype.QuotedBool) bool {
	return when != "false"
}

// isRequiredItem checks if an item is required based on whether Required is true and the item is
// enabled and not hidden
func isRequiredItem(item kotsv1beta1.ConfigItem) bool {
	if !item.Required {
		return false
	}
	if !isItemEnabled(item.When) {
		return false
	}
	if item.Hidden {
		return false
	}
	return true
}

func isUnsetItem(configValue kotsv1beta1.ConfigValue) bool {
	// TODO: repeatable items
	return configValue.ValuePlaintext == "" && configValue.Value == "" && configValue.Default == ""
}

// isFileType checks if the item type is "file"
func isFileType(item kotsv1beta1.ConfigItem) bool {
	return item.Type == "file"
}

// createRequiredFieldError creates a user-friendly error message for required fields
func createRequiredFieldError(item kotsv1beta1.ConfigItem) error {
	fieldName := item.Title
	if fieldName == "" {
		fieldName = item.Name
	}
	return fmt.Errorf("%s is required", fieldName)
}

// isValueBase64Encoded checks if the value of a ConfigValue is base64 encoded, this is used for file items
func isValueBase64Encoded(configValue kotsv1beta1.ConfigValue) bool {
	if configValue.Value == "" {
		return true // empty values are considered valid
	}
	if _, err := base64.StdEncoding.DecodeString(configValue.Value); err != nil {
		return false
	}
	return true
}
