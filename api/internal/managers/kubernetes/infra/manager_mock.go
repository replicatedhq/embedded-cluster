package infra

import (
	"context"

	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/replicatedhq/embedded-cluster/pkg-new/kubernetesinstallation"
	"github.com/stretchr/testify/mock"
)

var _ InfraManager = (*MockInfraManager)(nil)

// MockInfraManager is a mock implementation of InfraManager
type MockInfraManager struct {
	mock.Mock
}

func (m *MockInfraManager) Install(ctx context.Context, ki kubernetesinstallation.Installation) error {
	args := m.Called(ctx, ki)
	return args.Error(0)
}

func (m *MockInfraManager) Get() (types.Infra, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return types.Infra{}, args.Error(1)
	}
	return args.Get(0).(types.Infra), args.Error(1)
}
