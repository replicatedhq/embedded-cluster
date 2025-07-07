package install

import (
	"context"

	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
)

func (c *InstallController) GetAppConfig(ctx context.Context) (kotsv1beta1.Config, error) {
	return c.appConfigManager.Get()
}
