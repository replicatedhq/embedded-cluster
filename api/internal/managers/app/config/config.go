package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"maps"

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
)

var (
	// ErrConfigItemRequired is returned when a required item is not set
	ErrConfigItemRequired = errors.New("item is required")
	// ErrValueNotBase64Encoded is returned when a file item value is not base64 encoded
	ErrValueNotBase64Encoded = errors.New("value must be base64 encoded for file items")
)

func (m *appConfigManager) GetConfig() (kotsv1beta1.Config, error) {
	// Execute the config template
	processedYAML, err := m.executeConfigTemplate()
	if err != nil {
		return kotsv1beta1.Config{}, fmt.Errorf("execute config template: %w", err)
	}

	// Parse to Config struct
	var processedConfig kotsv1beta1.Config
	if err := kyaml.Unmarshal([]byte(processedYAML), &processedConfig); err != nil {
		return kotsv1beta1.Config{}, fmt.Errorf("unmarshal processed config: %w", err)
	}

	return filterAppConfig(processedConfig)
}

func (m *appConfigManager) ValidateConfigValues(configValues types.AppConfigValues) error {
	var ve *types.APIError

	processedConfig, err := m.GetConfig()
	if err != nil {
		return fmt.Errorf("get config: %w", err)
	}

	for _, group := range processedConfig.Spec.Groups {
		for _, item := range group.Items {
			configValue := getConfigValueFromItem(item, configValues)
			// check required items
			if isRequiredItem(item) && isUnsetItem(configValue) {
				ve = types.AppendFieldError(ve, item.Name, ErrConfigItemRequired)
			}
			// check value is base64 encoded for file items
			if isFileType(item) && !isValueBase64Encoded(configValue) {
				ve = types.AppendFieldError(ve, item.Name, ErrValueNotBase64Encoded)
			}
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

	// Merge new values with existing ones
	mergedValues := make(types.AppConfigValues)
	maps.Copy(mergedValues, existingValues)
	maps.Copy(mergedValues, newValues)

	// Get processed config to determine enabled groups and items
	processedConfig, err := m.GetConfig()
	if err != nil {
		return fmt.Errorf("get config: %w", err)
	}

	// only keep values for enabled groups and items
	filteredValues := make(types.AppConfigValues)
	for _, g := range processedConfig.Spec.Groups {
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

// GetConfigValues returns config values with optional password field masking
func (m *appConfigManager) GetConfigValues(maskPasswords bool) (types.AppConfigValues, error) {
	configValues, err := m.appConfigStore.GetConfigValues()
	if err != nil {
		return nil, err
	}

	// If masking is not requested, return the original values
	if !maskPasswords {
		return configValues, nil
	}

	// Get processed config to determine password fields
	processedConfig, err := m.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}

	// Create a copy of the config values to mask password fields
	maskedValues := make(types.AppConfigValues)
	maps.Copy(maskedValues, configValues)

	// Mask password fields
	for _, group := range processedConfig.Spec.Groups {
		for _, item := range group.Items {
			if item.Type == "password" {
				// Mask item
				if v, ok := maskedValues[item.Name]; ok && v.Value != "" {
					v.Value = PasswordMask
					maskedValues[item.Name] = v
				}
				// Mask child items
				for _, child := range item.Items {
					if v, ok := maskedValues[child.Name]; ok && v.Value != "" {
						v.Value = PasswordMask
						maskedValues[child.Name] = v
					}
				}
			}
		}
	}

	return maskedValues, nil
}

func (m *appConfigManager) GetKotsadmConfigValues() (kotsv1beta1.ConfigValues, error) {
	processedConfig, err := m.GetConfig()
	if err != nil {
		return kotsv1beta1.ConfigValues{}, fmt.Errorf("get config: %w", err)
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
	for _, group := range processedConfig.Spec.Groups {
		for _, item := range group.Items {
			kotsadmConfigValues.Spec.Values[item.Name] = getConfigValueFromItem(item, storedValues)

			for _, childItem := range item.Items {
				kotsadmConfigValues.Spec.Values[childItem.Name] = getConfigValueFromChildItem(item, childItem, storedValues)
			}
		}
	}

	return kotsadmConfigValues, nil
}

// setConfigValueByType sets either Value or ValuePlaintext based on the item type
func setConfigValueByType(cv *kotsv1beta1.ConfigValue, itemType string, value string) {
	if itemType == "password" {
		cv.ValuePlaintext = value
	} else {
		cv.Value = value
	}
}

// applyStoredValueOverride applies stored values from configValues if they exist
func applyStoredValueOverride(cv *kotsv1beta1.ConfigValue, itemType string, itemName string, configValues types.AppConfigValues) {
	if v, ok := configValues[itemName]; ok {
		setConfigValueByType(cv, itemType, v.Value)
		cv.Filename = v.Filename
	}
}

// createConfigValue creates a ConfigValue with the given parameters and applies overrides
func createConfigValue(defaultValue, initialValue, itemType, filename, itemName string, configValues types.AppConfigValues) kotsv1beta1.ConfigValue {
	configValue := kotsv1beta1.ConfigValue{
		Default:  defaultValue,
		Filename: filename,
	}

	setConfigValueByType(&configValue, itemType, initialValue)
	applyStoredValueOverride(&configValue, itemType, itemName, configValues)

	return configValue
}

func getConfigValueFromItem(item kotsv1beta1.ConfigItem, configValues types.AppConfigValues) kotsv1beta1.ConfigValue {
	return createConfigValue(
		item.Default.String(),
		item.Value.String(),
		item.Type,
		item.Filename,
		item.Name,
		configValues,
	)
}

func getConfigValueFromChildItem(item kotsv1beta1.ConfigItem, childItem kotsv1beta1.ConfigChildItem, configValues types.AppConfigValues) kotsv1beta1.ConfigValue {
	return createConfigValue(
		childItem.Default.String(),
		childItem.Value.String(),
		item.Type,
		item.Filename,
		childItem.Name,
		configValues,
	)
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
	return configValue.Value == "" && configValue.Default == ""
}

// isFileType checks if the item type is "file"
func isFileType(item kotsv1beta1.ConfigItem) bool {
	return item.Type == "file"
}

func isValueBase64Encoded(configValue kotsv1beta1.ConfigValue) bool {
	if configValue.Value == "" {
		return true // empty values are considered valid
	}
	if _, err := base64.StdEncoding.DecodeString(configValue.Value); err != nil {
		return false
	}
	return true
}
