package config

import (
	"context"
)

func (m *appConfigManager) GetConfigValues() (map[string]string, error) {
	return m.appConfigStore.GetConfigValues()
}

func (m *appConfigManager) SetConfigValues(ctx context.Context, values map[string]string) error {
	return m.appConfigStore.SetConfigValues(values)
}
