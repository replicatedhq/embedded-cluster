package install

import (
	"context"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

func (c *InstallController) GetAppConfigValues(ctx context.Context) (kotsv1beta1.ConfigValues, error) {
	return c.appConfigManager.GetConfigValues()
}

func (c *InstallController) SetAppConfigValues(ctx context.Context, values kotsv1beta1.ConfigValues) error {
	return c.appConfigManager.SetConfigValues(ctx, values)
}

func (c *InstallController) ApplyValuesToConfig(ctx context.Context, config kotsv1beta1.Config, values kotsv1beta1.ConfigValues) (kotsv1beta1.Config, error) {
	return c.appConfigManager.ApplyValuesToConfig(config, values)
}
