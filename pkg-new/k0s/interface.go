package k0s

import (
	"context"

	apv1b2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_k0s K0sInterface
)

func init() {
	Set(&K0s{})
}

func Set(k0s K0sInterface) {
	_k0s = k0s
}

type K0sStatus struct {
	Role          string                   `json:"Role"`
	Vars          K0sVars                  `json:"K0sVars"`
	ClusterConfig k0sv1beta1.ClusterConfig `json:"ClusterConfig"`
}

type K0sVars struct {
	AdminKubeConfigPath   string `json:"AdminKubeConfigPath"`
	KubeletAuthConfigPath string `json:"KubeletAuthConfigPath"`
	CertRootDir           string `json:"CertRootDir"`
	EtcdCertDir           string `json:"EtcdCertDir"`
}

type K0sInterface interface {
	GetStatus(ctx context.Context) (*K0sStatus, error)
	Install(rc runtimeconfig.RuntimeConfig, hostname string) error
	IsInstalled() (bool, error)
	NewK0sConfig(networkInterface string, isAirgap bool, podCIDR string, serviceCIDR string, eucfg *ecv1beta1.Config, mutate func(*k0sv1beta1.ClusterConfig) error) (*k0sv1beta1.ClusterConfig, error)
	WriteK0sConfig(ctx context.Context, cfg *k0sv1beta1.ClusterConfig) error
	PatchK0sConfig(path string, patch string) error
	WaitForK0s() error
	WaitForAutopilotPlan(ctx context.Context, cli client.Client, logger logrus.FieldLogger) (apv1b2.Plan, error)
	WaitForClusterNodesMatchVersion(ctx context.Context, cli client.Client, desiredVersion string, logger logrus.FieldLogger) error
	ClusterNodesMatchVersion(ctx context.Context, cli client.Client, version string) (bool, error)
	WaitForAirgapArtifactsAutopilotPlan(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) error
}

func GetStatus(ctx context.Context) (*K0sStatus, error) {
	return _k0s.GetStatus(ctx)
}

func Install(rc runtimeconfig.RuntimeConfig, hostname string) error {
	return _k0s.Install(rc, hostname)
}

func IsInstalled() (bool, error) {
	return _k0s.IsInstalled()
}

func NewK0sConfig(networkInterface string, isAirgap bool, podCIDR string, serviceCIDR string, eucfg *ecv1beta1.Config, mutate func(*k0sv1beta1.ClusterConfig) error) (*k0sv1beta1.ClusterConfig, error) {
	return _k0s.NewK0sConfig(networkInterface, isAirgap, podCIDR, serviceCIDR, eucfg, mutate)
}

func WriteK0sConfig(ctx context.Context, cfg *k0sv1beta1.ClusterConfig) error {
	return _k0s.WriteK0sConfig(ctx, cfg)
}

func PatchK0sConfig(path string, patch string) error {
	return _k0s.PatchK0sConfig(path, patch)
}

func WaitForK0s() error {
	return _k0s.WaitForK0s()
}

func WaitForAutopilotPlan(ctx context.Context, cli client.Client, logger logrus.FieldLogger) (apv1b2.Plan, error) {
	return _k0s.WaitForAutopilotPlan(ctx, cli, logger)
}

func WaitForClusterNodesMatchVersion(ctx context.Context, cli client.Client, desiredVersion string, logger logrus.FieldLogger) error {
	return _k0s.WaitForClusterNodesMatchVersion(ctx, cli, desiredVersion, logger)
}

func ClusterNodesMatchVersion(ctx context.Context, cli client.Client, version string) (bool, error) {
	return _k0s.ClusterNodesMatchVersion(ctx, cli, version)
}

func WaitForAirgapArtifactsAutopilotPlan(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) error {
	return _k0s.WaitForAirgapArtifactsAutopilotPlan(ctx, cli, in)
}
