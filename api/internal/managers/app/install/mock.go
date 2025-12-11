package install

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/mock"
)

// MockAppInstallManager is a mock implementation of the AppInstallManager interface
type MockAppInstallManager struct {
	mock.Mock
}

// Install mocks the Install method
func (m *MockAppInstallManager) Install(ctx context.Context, installableCharts []types.InstallableHelmChart, configValues types.AppConfigValues, registrySettings *types.RegistrySettings, hostCABundlePath string) error {
	args := m.Called(ctx, installableCharts, configValues, registrySettings, hostCABundlePath)
	return args.Error(0)
}
