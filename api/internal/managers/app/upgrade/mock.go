package appupgrademanager

import (
	"context"

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
