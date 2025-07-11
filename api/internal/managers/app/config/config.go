package config

import (
	"fmt"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/tiendc/go-deepcopy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (m *appConfigManager) GetConfig(config kotsv1beta1.Config) (kotsv1beta1.Config, error) {
	return filterAppConfig(config)
}

func (m *appConfigManager) GetConfigValues() (map[string]string, error) {
	return m.appConfigStore.GetConfigValues()
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
	filteredConfig, err := m.GetConfig(config)
	if err != nil {
		return kotsv1beta1.ConfigValues{}, fmt.Errorf("get config: %w", err)
	}

	storedValues, err := m.GetConfigValues()
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
			configValue := kotsv1beta1.ConfigValue{
				Value:   item.Value.String(),
				Default: item.Default.String(),
			}
			// override values from the config values store
			if value, ok := storedValues[item.Name]; ok {
				configValue.Value = value
			}
			kotsadmConfigValues.Spec.Values[item.Name] = configValue

			for _, subItem := range item.Items {
				subConfigValue := kotsv1beta1.ConfigValue{
					Value:   subItem.Value.String(),
					Default: subItem.Default.String(),
				}
				// override values from the config values store
				if value, ok := storedValues[subItem.Name]; ok {
					subConfigValue.Value = value
				}
				kotsadmConfigValues.Spec.Values[subItem.Name] = subConfigValue
			}
		}
	}

	return kotsadmConfigValues, nil
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
