package release

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/mock"
)

var _ AppReleaseManager = (*MockAppReleaseManager)(nil)

// MockAppReleaseManager is a mock implementation of the AppReleaseManager interface
type MockAppReleaseManager struct {
	mock.Mock
}

// ExtractAppPreflightSpec mocks the ExtractAppPreflightSpec method
func (m *MockAppReleaseManager) ExtractAppPreflightSpec(ctx context.Context, configValues types.AppConfigValues, proxySpec *ecv1beta1.ProxySpec, registrySettings *types.RegistrySettings) (*troubleshootv1beta2.PreflightSpec, error) {
	args := m.Called(ctx, configValues, proxySpec, registrySettings)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*troubleshootv1beta2.PreflightSpec), args.Error(1)
}

// ExtractInstallableHelmCharts mocks the ExtractInstallableHelmCharts method
func (m *MockAppReleaseManager) ExtractInstallableHelmCharts(ctx context.Context, configValues types.AppConfigValues, proxySpec *ecv1beta1.ProxySpec, registrySettings *types.RegistrySettings) ([]types.InstallableHelmChart, error) {
	args := m.Called(ctx, configValues, proxySpec, registrySettings)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.InstallableHelmChart), args.Error(1)
}
