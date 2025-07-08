package install

import (
	"context"
	"errors"
	"fmt"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

func (c *InstallController) GetAppConfig(ctx context.Context) (kotsv1beta1.Config, error) {
	if c.releaseData == nil || c.releaseData.AppConfig == nil {
		return kotsv1beta1.Config{}, errors.New("app config not found")
	}

	values, err := c.appConfigManager.GetConfigValues()
	if err != nil {
		return kotsv1beta1.Config{}, fmt.Errorf("get app config values: %w", err)
	}

	appConfig, err := c.appConfigManager.ApplyValuesToConfig(*c.releaseData.AppConfig, values)
	if err != nil {
		return kotsv1beta1.Config{}, fmt.Errorf("apply values to config: %w", err)
	}

	return appConfig, nil
}

func (c *InstallController) SetAppConfigValues(ctx context.Context, values map[string]string) error {
	return c.appConfigManager.SetConfigValues(ctx, values)
}
