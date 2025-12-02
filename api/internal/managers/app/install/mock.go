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
func (m *MockAppInstallManager) Install(ctx context.Context, installableCharts []types.InstallableHelmChart) error {
	args := m.Called(ctx, installableCharts)
	return args.Error(0)
}

// GetStatus mocks the GetStatus method
func (m *MockAppInstallManager) GetStatus() (types.AppInstall, error) {
	args := m.Called()
	return args.Get(0).(types.AppInstall), args.Error(1)
}
