package dryrun

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg/k0s"
)

var _ k0s.ClientInterface = (*K0sClient)(nil)

type K0sClient struct {
	Status *k0s.K0sStatus
}

func (c *K0sClient) GetStatus(ctx context.Context) (*k0s.K0sStatus, error) {
	return c.Status, nil
}
