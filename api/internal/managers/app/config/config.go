package config

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/tiendc/go-deepcopy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// PasswordMask is the string used to mask password values in config responses
	PasswordMask = "••••••••"
)

var (
	// ErrConfigItemRequired is returned when a required item is not set
	ErrConfigItemRequired = errors.New("item is required")
)

func (m *appConfigManager) GetConfig(config kotsv1beta1.Config) (kotsv1beta1.Config, error) {
	return filterAppConfig(config)
}

func (m *appConfigManager) ValidateConfigValues(config kotsv1beta1.Config, configValues map[string]string) error {
	var ve *types.APIError

	filteredConfig, err := filterAppConfig(config)
	if err != nil {
		return fmt.Errorf("filter app config: %w", err)
	}

	// check required items
	for _, group := range filteredConfig.Spec.Groups {
		for _, item := range group.Items {
			configValue := getConfigValueFromItem(item, configValues)
			if isRequiredItem(item) && isUnsetItem(configValue) {
				ve = types.AppendFieldError(ve, item.Name, ErrConfigItemRequired)
			}
		}
	}

	return ve.ErrorOrNil()
}

// PatchConfigValues performs a partial update by merging new values with existing ones
func (m *appConfigManager) PatchConfigValues(ctx context.Context, config kotsv1beta1.Config, newValues map[string]string) error {
	// Get existing values
	existingValues, err := m.appConfigStore.GetConfigValues()
	if err != nil {
		return fmt.Errorf("get config values: %w", err)
	}

	// Merge new values with existing ones
	mergedValues := make(map[string]string)
	maps.Copy(mergedValues, existingValues)
	maps.Copy(mergedValues, newValues)

	// only keep values for enabled groups and items
	filteredValues := make(map[string]string)
	for _, g := range config.Spec.Groups {
		for _, i := range g.Items {
			if isItemEnabled(g.When) && isItemEnabled(i.When) {
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
	}

	return m.appConfigStore.SetConfigValues(filteredValues)
}

// GetConfigValues returns config values with optional password field masking
func (m *appConfigManager) GetConfigValues(ctx context.Context, config kotsv1beta1.Config, maskPasswords bool) (map[string]string, error) {
	configValues, err := m.appConfigStore.GetConfigValues()
	if err != nil {
		return nil, err
	}

	// If masking is not requested, return the original values
	if !maskPasswords {
		return configValues, nil
	}

	// Create a copy of the config values to mask password fields
	maskedValues := make(map[string]string)
	maps.Copy(maskedValues, configValues)

	// Mask password fields
	for _, group := range config.Spec.Groups {
		for _, item := range group.Items {
			if item.Type == "password" {
				// Mask item
				if value, ok := maskedValues[item.Name]; ok && value != "" {
					maskedValues[item.Name] = PasswordMask
				}
				// Mask child items
				for _, child := range item.Items {
					if value, ok := maskedValues[child.Name]; ok && value != "" {
						maskedValues[child.Name] = PasswordMask
					}
				}
			}
		}
	}

	return maskedValues, nil
}

func (m *appConfigManager) GetKotsadmConfigValues(ctx context.Context, config kotsv1beta1.Config) (kotsv1beta1.ConfigValues, error) {
	filteredConfig, err := m.GetConfig(config)
	if err != nil {
		return kotsv1beta1.ConfigValues{}, fmt.Errorf("get config: %w", err)
	}

	storedValues, err := m.GetConfigValues(ctx, filteredConfig, false)
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

	// add values from the filtered config
	for _, group := range filteredConfig.Spec.Groups {
		for _, item := range group.Items {
			kotsadmConfigValues.Spec.Values[item.Name] = getConfigValueFromItem(item, storedValues)

			for _, childItem := range item.Items {
				kotsadmConfigValues.Spec.Values[childItem.Name] = getConfigValueFromChildItem(item.Type, childItem, storedValues)
			}
		}
	}

	return kotsadmConfigValues, nil
}

func getConfigValueFromItem(item kotsv1beta1.ConfigItem, configValues map[string]string) kotsv1beta1.ConfigValue {
	configValue := kotsv1beta1.ConfigValue{
		Default: item.Default.String(),
	}
	if item.Type == "password" {
		configValue.ValuePlaintext = item.Value.String()
	} else {
		configValue.Value = item.Value.String()
	}

	// override values from the config values store
	if value, ok := configValues[item.Name]; ok {
		if item.Type == "password" {
			configValue.ValuePlaintext = value
		} else {
			configValue.Value = value
		}
	}

	return configValue
}

func getConfigValueFromChildItem(itemType string, childItem kotsv1beta1.ConfigChildItem, configValues map[string]string) kotsv1beta1.ConfigValue {
	configValue := kotsv1beta1.ConfigValue{
		Default: childItem.Default.String(),
	}
	if itemType == "password" {
		configValue.ValuePlaintext = childItem.Value.String()
	} else {
		configValue.Value = childItem.Value.String()
	}

	// override values from the config values store
	if value, ok := configValues[childItem.Name]; ok {
		if itemType == "password" {
			configValue.ValuePlaintext = value
		} else {
			configValue.Value = value
		}
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
	return configValue.Value == "" && configValue.Default == ""
}
