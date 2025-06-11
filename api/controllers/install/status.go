package install

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *InstallController) SetStatus(ctx context.Context, status types.Status) error {
	return c.installationManager.SetStatus(status)
}

func (c *InstallController) GetStatus(ctx context.Context) (*types.Status, error) {
	return c.installationManager.GetStatus()
}
