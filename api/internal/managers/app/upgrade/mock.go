package appupgrademanager

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/mock"
)

// MockAppUpgradeManager is a mock implementation of the AppUpgradeManager interface
type MockAppUpgradeManager struct {
	mock.Mock
}

// Upgrade mocks the Upgrade method
func (m *MockAppUpgradeManager) Upgrade(ctx context.Context, configValues kotsv1beta1.ConfigValues) error {
	args := m.Called(ctx, configValues)
	return args.Error(0)
}

// GetStatus mocks the GetStatus method
func (m *MockAppUpgradeManager) GetStatus() (types.AppUpgrade, error) {
	args := m.Called()
	return args.Get(0).(types.AppUpgrade), args.Error(1)
}
