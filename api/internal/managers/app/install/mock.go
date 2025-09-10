package install

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/mock"
)

// MockAppInstallManager is a mock implementation of the AppInstallManager interface
type MockAppInstallManager struct {
	mock.Mock
}

// Install mocks the Install method
func (m *MockAppInstallManager) Install(ctx context.Context, configValues kotsv1beta1.ConfigValues) error {
	args := m.Called(ctx, configValues)
	return args.Error(0)
}

// GetStatus mocks the GetStatus method
func (m *MockAppInstallManager) GetStatus() (types.AppInstall, error) {
	args := m.Called()
	return args.Get(0).(types.AppInstall), args.Error(1)
}
