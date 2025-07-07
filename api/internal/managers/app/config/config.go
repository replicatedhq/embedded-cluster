package config

import (
	"context"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (m *appConfigManager) Get() (kotsv1beta1.Config, error) {
	return m.appConfigStore.Get()
}

func (m *appConfigManager) Set(ctx context.Context) error {
	return nil
}

// GetAppConfigValues converts boolean config items to ConfigValues
func (m *appConfigManager) GetAppConfigValues() (kotsv1beta1.ConfigValues, error) {
	// 1. Get current Config from store
	config, err := m.appConfigStore.Get()
	if err != nil {
		return kotsv1beta1.ConfigValues{}, err
	}

	// 2. Generate ConfigValues structure
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

	// 3. Process only boolean config items
	for _, group := range config.Spec.Groups {
		for _, item := range group.Items {
			if item.Type == "bool" {
				// Get the value and default from the config item
				itemValue := item.Value.String()
				defaultValue := item.Default.String()

				// Use value if set, otherwise use default as value
				finalValue := defaultValue
				if itemValue != "" {
					finalValue = itemValue
				}

				configValues.Spec.Values[item.Name] = kotsv1beta1.ConfigValue{
					Value:   finalValue,
					Default: defaultValue,
				}
			}
		}
	}

	return configValues, nil
}
