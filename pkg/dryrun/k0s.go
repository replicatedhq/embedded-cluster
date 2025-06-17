package dryrun

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
)

var _ k0s.K0sInterface = (*K0s)(nil)

type K0s struct {
	Status *k0s.K0sStatus
}

func (c *K0s) GetStatus(ctx context.Context) (*k0s.K0sStatus, error) {
	return c.Status, nil
}

func (c *K0s) Install(rc runtimeconfig.RuntimeConfig) error {
	return nil // TODO: implement
}

func (c *K0s) IsInstalled() (bool, error) {
	return c.Status != nil, nil
}

func (c *K0s) WaitForK0s() error {
	return nil
}
