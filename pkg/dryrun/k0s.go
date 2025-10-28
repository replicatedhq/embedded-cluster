package dryrun

import (
	"context"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
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

func (c *K0s) Install(rc runtimeconfig.RuntimeConfig, hostname string) error {
	return k0s.New().Install(rc, hostname) // actual implementation accounts for dryrun
}

func (c *K0s) IsInstalled() (bool, error) {
	return c.Status != nil, nil
}

func (c *K0s) NewK0sConfig(networkInterface string, isAirgap bool, podCIDR string, serviceCIDR string, eucfg *ecv1beta1.Config, mutate func(*k0sv1beta1.ClusterConfig) error) (*k0sv1beta1.ClusterConfig, error) {
	return k0s.New().NewK0sConfig(networkInterface, isAirgap, podCIDR, serviceCIDR, eucfg, mutate) // actual implementation accounts for dryrun
}

func (c *K0s) WriteK0sConfig(ctx context.Context, cfg *k0sv1beta1.ClusterConfig) error {
	return k0s.New().WriteK0sConfig(ctx, cfg) // actual implementation accounts for dryrun
}

func (c *K0s) PatchK0sConfig(path string, patch string) error {
	return k0s.New().PatchK0sConfig(path, patch) // actual implementation accounts for dryrun
}

func (c *K0s) WaitForK0s() error {
	return nil
}
