package config

import (
	"errors"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/tiendc/go-deepcopy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	ErrConfigItemRequired  = errors.New("item is required")
	ErrConfigValueNotFound = errors.New("value not found")
)

func (m *appConfigManager) GetConfig(config kotsv1beta1.Config) (kotsv1beta1.Config, error) {
	return filterAppConfig(config)
}

func (m *appConfigManager) GetConfigValues() (map[string]string, error) {
	return m.appConfigStore.GetConfigValues()
}

func (m *appConfigManager) ValidateConfigValues(config kotsv1beta1.Config, configValues map[string]string) error {
	var ve *types.APIError

	configItems := make(map[string]kotsv1beta1.ConfigItem)
	configChildItems := make(map[string]kotsv1beta1.ConfigChildItem)

	// first check required items
	for _, group := range config.Spec.Groups {
		for _, item := range group.Items {
			configItems[item.Name] = item

			configValue := getConfigValueFromItem(item, configValues)
			if isRequiredItem(item) && isUnsetItem(configValue) {
				ve = types.AppendFieldError(ve, item.Name, ErrConfigItemRequired)
			}

			for _, childItem := range item.Items {
				configChildItems[childItem.Name] = childItem
			}
		}
	}

	// then check if all the config values are present
	for name := range configValues {
		if _, ok := configItems[name]; ok {
			continue
		}
		if _, ok := configChildItems[name]; ok {
			continue
		}
		ve = types.AppendFieldError(ve, name, ErrConfigValueNotFound)
	}

	return ve.ErrorOrNil()
}

func (m *appConfigManager) SetConfigValues(config kotsv1beta1.Config, configValues map[string]string) error {
	filteredValues := make(map[string]string)

	// only include values for enabled groups and items
	for _, g := range config.Spec.Groups {
		for _, i := range g.Items {
			if isItemEnabled(g.When) && isItemEnabled(i.When) {
				value, ok := configValues[i.Name]
				if ok {
					filteredValues[i.Name] = value
				}
				for _, c := range i.Items {
					value, ok := configValues[c.Name]
					if ok {
						filteredValues[c.Name] = value
					}
				}
			}
		}
	}

	return m.appConfigStore.SetConfigValues(filteredValues)
}

func (m *appConfigManager) GetKotsadmConfigValues(config kotsv1beta1.Config) (kotsv1beta1.ConfigValues, error) {
	storedValues, err := m.GetConfigValues()
	if err != nil {
		return kotsv1beta1.ConfigValues{}, fmt.Errorf("get config values: %w", err)
	}

	return m.getKotsadmConfigValues(config, storedValues)
}

func (m *appConfigManager) getKotsadmConfigValues(config kotsv1beta1.Config, configValues map[string]string) (kotsv1beta1.ConfigValues, error) {
	filteredConfig, err := m.GetConfig(config)
	if err != nil {
		return kotsv1beta1.ConfigValues{}, fmt.Errorf("get config: %w", err)
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
			kotsadmConfigValues.Spec.Values[item.Name] = getConfigValueFromItem(item, configValues)

			for _, childItem := range item.Items {
				kotsadmConfigValues.Spec.Values[childItem.Name] = getConfigValueFromChildItem(childItem, configValues)
			}
		}
	}

	return kotsadmConfigValues, nil
}

func getConfigValueFromItem(item kotsv1beta1.ConfigItem, configValues map[string]string) kotsv1beta1.ConfigValue {
	configValue := kotsv1beta1.ConfigValue{
		Value:   item.Value.String(),
		Default: item.Default.String(),
	}
	// override values from the config values store
	value, ok := configValues[item.Name]
	if ok {
		configValue.Value = value
	}
	return configValue
}

func getConfigValueFromChildItem(item kotsv1beta1.ConfigChildItem, configValues map[string]string) kotsv1beta1.ConfigValue {
	configValue := kotsv1beta1.ConfigValue{
		Value:   item.Value.String(),
		Default: item.Default.String(),
	}
	// override values from the config values store
	value, ok := configValues[item.Name]
	if ok {
		configValue.Value = value
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

func isRequiredItem(item kotsv1beta1.ConfigItem) bool {
	if !item.Required {
		return false
	}
	// TODO: should an item really not be required if it's hidden?
	if item.Hidden || item.When == "false" {
		return false
	}
	return true
}

func isUnsetItem(configValue kotsv1beta1.ConfigValue) bool {
	// TODO: repeatable items
	return configValue.Value == "" && configValue.Default == ""
}
