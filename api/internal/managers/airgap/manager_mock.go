package airgap

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/mock"
)

var _ AirgapManager = (*MockAirgapManager)(nil)

// MockAirgapManager is a mock implementation of the AirgapManager interface
type MockAirgapManager struct {
	mock.Mock
}

// Process mocks the Process method
func (m *MockAirgapManager) Process(ctx context.Context, registrySettings *types.RegistrySettings) error {
	args := m.Called(ctx, registrySettings)
	return args.Error(0)
}

// GetStatus mocks the GetStatus method
func (m *MockAirgapManager) GetStatus() (types.Airgap, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return types.Airgap{}, args.Error(1)
	}
	return args.Get(0).(types.Airgap), args.Error(1)
}
