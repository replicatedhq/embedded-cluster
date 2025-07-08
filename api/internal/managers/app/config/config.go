package config

import (
	"context"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

func (m *appConfigManager) Get() (kotsv1beta1.Config, error) {
	return m.appConfigStore.Get()
}

func (m *appConfigManager) Set(ctx context.Context) error {
	return nil
}
