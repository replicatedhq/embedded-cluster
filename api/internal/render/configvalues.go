package render

import (
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GenerateConfigValues converts a Config to ConfigValues, processing supported field types
func GenerateConfigValues(config kotsv1beta1.Config) kotsv1beta1.ConfigValues {
	configValues := kotsv1beta1.ConfigValues{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kots.io/v1beta1",
			Kind:       "ConfigValues",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "app-config",
		},
		Spec: kotsv1beta1.ConfigValuesSpec{
			Values: make(map[string]kotsv1beta1.ConfigValue),
		},
	}

	// Process all config items by type
	for _, group := range config.Spec.Groups {
		for _, item := range group.Items {
			if configValue, processed := processBooleanConfigItem(item); processed {
				configValues.Spec.Values[item.Name] = configValue
			}
		}
	}

	return configValues
}

// processBooleanConfigItem processes boolean config items and returns the ConfigValue and whether it was processed
func processBooleanConfigItem(item kotsv1beta1.ConfigItem) (kotsv1beta1.ConfigValue, bool) {
	if item.Type != "bool" {
		return kotsv1beta1.ConfigValue{}, false
	}

	// Get the value and default from the config item
	itemValue := item.Value.String()
	defaultValue := item.Default.String()

	// Use value if set, otherwise use default as value
	finalValue := defaultValue
	if itemValue != "" {
		finalValue = itemValue
	}

	configValue := kotsv1beta1.ConfigValue{
		Value:   finalValue,
		Default: defaultValue,
	}

	return configValue, true
}
