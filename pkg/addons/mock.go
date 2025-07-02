package addons

import (
	"context"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	ectypes "github.com/replicatedhq/embedded-cluster/kinds/types"
	"github.com/replicatedhq/embedded-cluster/pkg/spinner"
	"github.com/stretchr/testify/mock"
)

var _ AddOnsInterface = (*MockAddOns)(nil)

// MockAddOns is a mock implementation of the AddOnsInterface
type MockAddOns struct {
	mock.Mock
}

// Install mocks the Install method
func (m *MockAddOns) Install(ctx context.Context, opts InstallOptions) error {
	args := m.Called(ctx, opts)
	return args.Error(0)
}

// Upgrade mocks the Upgrade method
func (m *MockAddOns) Upgrade(ctx context.Context, in *ecv1beta1.Installation, meta *ectypes.ReleaseMetadata, opts UpgradeOptions) error {
	args := m.Called(ctx, in, meta, opts)
	return args.Error(0)
}

// CanEnableHA mocks the CanEnableHA method
func (m *MockAddOns) CanEnableHA(ctx context.Context) (bool, string, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.String(1), args.Error(2)
}

// EnableHA mocks the EnableHA method
func (m *MockAddOns) EnableHA(ctx context.Context, opts EnableHAOptions, spinner *spinner.MessageWriter) error {
	args := m.Called(ctx, opts, spinner)
	return args.Error(0)
}

// EnableAdminConsoleHA mocks the EnableAdminConsoleHA method
func (m *MockAddOns) EnableAdminConsoleHA(ctx context.Context, opts EnableHAOptions) error {
	args := m.Called(ctx, opts)
	return args.Error(0)
}
