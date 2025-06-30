package k0s

import (
	"context"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/mock"
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
func (m *MockK0s) Install(rc runtimeconfig.RuntimeConfig) error {
	args := m.Called(rc)
	return args.Error(0)
}

// IsInstalled mocks the IsInstalled method
func (m *MockK0s) IsInstalled() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

// WriteK0sConfig mocks the WriteK0sConfig method
func (m *MockK0s) WriteK0sConfig(ctx context.Context, networkInterface string, airgapBundle string, podCIDR string, serviceCIDR string, eucfg *ecv1beta1.Config, mutate func(*k0sv1beta1.ClusterConfig) error) (*k0sv1beta1.ClusterConfig, error) {
	args := m.Called(ctx, networkInterface, airgapBundle, podCIDR, serviceCIDR, eucfg, mutate)
	return args.Get(0).(*k0sv1beta1.ClusterConfig), args.Error(1)
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
