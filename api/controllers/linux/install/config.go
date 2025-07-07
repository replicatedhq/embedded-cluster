package install

import (
	"context"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

// GetAppConfigValues gets the config values from the app config manager
func (c *InstallController) GetAppConfigValues(ctx context.Context) (kotsv1beta1.ConfigValues, error) {
	return c.appConfigManager.GetAppConfigValues()
}
