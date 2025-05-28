package hostutils

import (
	"context"

	"github.com/stretchr/testify/mock"
)

var _ HostUtilsInterface = (*MockHostUtils)(nil)

// MockHostUtils is a mock implementation of the HostUtilsInterface
type MockHostUtils struct {
	mock.Mock
}

// ConfigureForInstall mocks the ConfigureForInstall method
func (m *MockHostUtils) ConfigureForInstall(ctx context.Context, opts InitForInstallOptions) error {
	args := m.Called(ctx, opts)
	return args.Error(0)
}

// ConfigureSysctl mocks the ConfigureSysctl method
func (m *MockHostUtils) ConfigureSysctl() error {
	args := m.Called()
	return args.Error(0)
}

// ConfigureKernelModules mocks the ConfigureKernelModules method
func (m *MockHostUtils) ConfigureKernelModules() error {
	args := m.Called()
	return args.Error(0)
}

// ConfigureNetworkManager mocks the ConfigureNetworkManager method
func (m *MockHostUtils) ConfigureNetworkManager(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// ConfigureFirewalld mocks the ConfigureFirewalld method
func (m *MockHostUtils) ConfigureFirewalld(ctx context.Context, podNetwork, serviceNetwork string) error {
	args := m.Called(ctx, podNetwork, serviceNetwork)
	return args.Error(0)
}

// ResetFirewalld mocks the ResetFirewalld method
func (m *MockHostUtils) ResetFirewalld(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MaterializeFiles mocks the MaterializeFiles method
func (m *MockHostUtils) MaterializeFiles(airgapBundle string) error {
	args := m.Called(airgapBundle)
	return args.Error(0)
}
