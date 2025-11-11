package k0s

import (
	"context"

	apv1b2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ K0sInterface = (*MockK0s)(nil)

// MockK0s is a mock implementation of the K0sInterface
type MockK0s struct {
	mock.Mock
}

// GetStatus mocks the GetStatus method
func (m *MockK0s) GetStatus(ctx context.Context) (*K0sStatus, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*K0sStatus), args.Error(1)
}

// Install mocks the Install method
func (m *MockK0s) Install(rc runtimeconfig.RuntimeConfig, hostname string) error {
	args := m.Called(rc, hostname)
	return args.Error(0)
}

// IsInstalled mocks the IsInstalled method
func (m *MockK0s) IsInstalled() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

// NewK0sConfig mocks the NewK0sConfig method
func (m *MockK0s) NewK0sConfig(networkInterface string, isAirgap bool, podCIDR string, serviceCIDR string, eucfg *ecv1beta1.Config, mutate func(*k0sv1beta1.ClusterConfig) error) (*k0sv1beta1.ClusterConfig, error) {
	args := m.Called(networkInterface, isAirgap, podCIDR, serviceCIDR, eucfg, mutate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*k0sv1beta1.ClusterConfig), args.Error(1)
}

// WriteK0sConfig mocks the WriteK0sConfig method
func (m *MockK0s) WriteK0sConfig(ctx context.Context, cfg *k0sv1beta1.ClusterConfig) error {
	args := m.Called(ctx, cfg)
	return args.Error(0)
}

// PatchK0sConfig mocks the PatchK0sConfig method
func (m *MockK0s) PatchK0sConfig(path string, patch string) error {
	args := m.Called(path, patch)
	return args.Error(0)
}

// WaitForK0s mocks the WaitForK0s method
func (m *MockK0s) WaitForK0s() error {
	args := m.Called()
	return args.Error(0)
}

// WaitForAutopilotPlan mocks the WaitForAutopilotPlan method
func (m *MockK0s) WaitForAutopilotPlan(ctx context.Context, cli client.Client, logger logrus.FieldLogger) (apv1b2.Plan, error) {
	args := m.Called(ctx, cli, logger)
	if args.Get(0) == nil {
		return apv1b2.Plan{}, args.Error(1)
	}
	return args.Get(0).(apv1b2.Plan), args.Error(1)
}

// WaitForClusterNodesMatchVersion mocks the WaitForClusterNodesMatchVersion method
func (m *MockK0s) WaitForClusterNodesMatchVersion(ctx context.Context, cli client.Client, desiredVersion string, logger logrus.FieldLogger) error {
	args := m.Called(ctx, cli, desiredVersion, logger)
	return args.Error(0)
}

// ClusterNodesMatchVersion mocks the ClusterNodesMatchVersion method
func (m *MockK0s) ClusterNodesMatchVersion(ctx context.Context, cli client.Client, version string) (bool, error) {
	args := m.Called(ctx, cli, version)
	return args.Bool(0), args.Error(1)
}

// WaitForAirgapArtifactsAutopilotPlan mocks the WaitForAirgapArtifactsAutopilotPlan method
func (m *MockK0s) WaitForAirgapArtifactsAutopilotPlan(ctx context.Context, cli client.Client, in *ecv1beta1.Installation) error {
	args := m.Called(ctx, cli, in)
	return args.Error(0)
}
