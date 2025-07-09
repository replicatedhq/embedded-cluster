package config

import (
	"context"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/replicatedhq/kotskinds/multitype"
	"github.com/tiendc/go-deepcopy"
)

func (m *appConfigManager) GetConfigValues() (map[string]string, error) {
	return m.appConfigStore.GetConfigValues()
}

func (m *appConfigManager) SetConfigValues(ctx context.Context, values map[string]string) error {
	return m.appConfigStore.SetConfigValues(values)
}

func (m *appConfigManager) ApplyValuesToConfig(config kotsv1beta1.Config, configValues map[string]string) (kotsv1beta1.Config, error) {
	// deepcopy the config to avoid mutating the original config
	var updatedConfig kotsv1beta1.Config
	if err := deepcopy.Copy(&updatedConfig, &config); err != nil {
		return kotsv1beta1.Config{}, err
	}

	for idxG, g := range updatedConfig.Spec.Groups {
		for idxI, i := range g.Items {
			value, ok := configValues[i.Name]
			if ok {
				updatedConfig.Spec.Groups[idxG].Items[idxI].Value = multitype.FromString(value)
			}
			for idxC, c := range i.Items {
				value, ok := configValues[c.Name]
				if ok {
					updatedConfig.Spec.Groups[idxG].Items[idxI].Items[idxC].Value = multitype.FromString(value)
				}
			}
		}
	}

	return updatedConfig, nil
}
