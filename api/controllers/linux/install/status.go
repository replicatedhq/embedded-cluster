package install

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func (c *InstallController) SetStatus(ctx context.Context, status types.Status) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.install.Status = status
	return nil
}

func (c *InstallController) GetStatus(ctx context.Context) (types.Status, error) {
	return c.install.Status, nil
}
