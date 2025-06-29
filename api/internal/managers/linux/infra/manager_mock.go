package infra

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg/runtimeconfig"
	"github.com/stretchr/testify/mock"
)

var _ InfraManager = (*MockInfraManager)(nil)

// MockInfraManager is a mock implementation of InfraManager
type MockInfraManager struct {
	mock.Mock
}

func (m *MockInfraManager) Install(ctx context.Context, rc runtimeconfig.RuntimeConfig) error {
	args := m.Called(ctx, rc)
	return args.Error(0)
}

func (m *MockInfraManager) Get() (types.LinuxInfra, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return types.LinuxInfra{}, args.Error(1)
	}
	return args.Get(0).(types.LinuxInfra), args.Error(1)
}
