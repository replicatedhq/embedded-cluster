package dryrun

import (
	"context"

	apv1b2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/autopilot/controller/plans/core"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg-new/k0s"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ k0s.K0sInterface = (*K0s)(nil)

type K0s struct {
	Status                                *k0s.K0sStatus
	k0s                                   *k0s.K0s
	WaitForAutopilotPlanFn                func(ctx context.Context, cli client.Client, logger logrus.FieldLogger) (apv1b2.Plan, error)
	WaitForClusterNodesMatchVersionFn     func(ctx context.Context, cli client.Client, desiredVersion string, logger logrus.FieldLogger) error
	ClusterNodesMatchVersionFn            func(ctx context.Context, cli client.Client, version string) (bool, error)
	WaitForAirgapArtifactsAutopilotPlanFn func(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) error
}

func (c *K0s) GetStatus(ctx context.Context) (*k0s.K0sStatus, error) {
	return c.Status, nil
}

func (c *K0s) Install(rc runtimeconfig.RuntimeConfig, hostname string) error {
	return c.k0s.Install(rc, hostname) // actual implementation accounts for dryrun
}

func (c *K0s) IsInstalled() (bool, error) {
	return c.Status != nil, nil
}

func (c *K0s) NewK0sConfig(networkInterface string, isAirgap bool, podCIDR string, serviceCIDR string, eucfg *ecv1beta1.Config, mutate func(*k0sv1beta1.ClusterConfig) error) (*k0sv1beta1.ClusterConfig, error) {
	return c.k0s.NewK0sConfig(networkInterface, isAirgap, podCIDR, serviceCIDR, eucfg, mutate) // actual implementation accounts for dryrun
}

func (c *K0s) WriteK0sConfig(ctx context.Context, cfg *k0sv1beta1.ClusterConfig) error {
	return c.k0s.WriteK0sConfig(ctx, cfg) // actual implementation accounts for dryrun
}

func (c *K0s) PatchK0sConfig(path string, patch string) error {
	return c.k0s.PatchK0sConfig(path, patch) // actual implementation accounts for dryrun
}

func (c *K0s) WaitForK0s() error {
	return nil
}

func (c *K0s) WaitForAutopilotPlan(ctx context.Context, cli client.Client, logger logrus.FieldLogger) (apv1b2.Plan, error) {
	if c.WaitForAutopilotPlanFn != nil {
		return c.WaitForAutopilotPlanFn(ctx, cli, logger)
	}
	return apv1b2.Plan{
		Status: apv1b2.PlanStatus{
			State: core.PlanCompleted,
		},
	}, nil
}

func (c *K0s) WaitForClusterNodesMatchVersion(ctx context.Context, cli client.Client, desiredVersion string, logger logrus.FieldLogger) error {
	if c.WaitForClusterNodesMatchVersionFn != nil {
		return c.WaitForClusterNodesMatchVersionFn(ctx, cli, desiredVersion, logger)
	}
	return nil
}

func (c *K0s) ClusterNodesMatchVersion(ctx context.Context, cli client.Client, version string) (bool, error) {
	if c.ClusterNodesMatchVersionFn != nil {
		return c.ClusterNodesMatchVersionFn(ctx, cli, version)
	}
	return true, nil
}

func (c *K0s) WaitForAirgapArtifactsAutopilotPlan(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) error {
	if c.WaitForAirgapArtifactsAutopilotPlanFn != nil {
		return c.WaitForAirgapArtifactsAutopilotPlanFn(ctx, cli, in)
	}
	return nil
}
