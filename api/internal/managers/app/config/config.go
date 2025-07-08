package config

import (
	"context"
	"fmt"

	"github.com/replicatedhq/embedded-cluster/api/internal/render"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

func (m *appConfigManager) Get() (kotsv1beta1.Config, error) {
	return m.appConfigStore.Get()
}

func (m *appConfigManager) Set(ctx context.Context, config kotsv1beta1.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.appConfigStore.Set(config); err != nil {
		return fmt.Errorf("setting config in store: %w", err)
	}

	return nil
}

// RenderAppConfigValues converts config items to ConfigValues
func (m *appConfigManager) RenderAppConfigValues() (kotsv1beta1.ConfigValues, error) {
	// Get current Config from store
	appConfig, err := m.appConfigStore.Get()
	if err != nil {
		return kotsv1beta1.ConfigValues{}, err
	}

	// Use the render package to generate ConfigValues
	return render.GenerateConfigValues(appConfig), nil
}
