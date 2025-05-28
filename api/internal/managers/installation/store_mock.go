package installation

import (
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/mock"
)

var _ InstallationStore = (*MockInstallationStore)(nil)

// MockInstallationStore is a mock implementation of the InstallationStore interface
type MockInstallationStore struct {
	mock.Mock
}

// GetConfig mocks the GetConfig method
func (m *MockInstallationStore) GetConfig() (*types.InstallationConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.InstallationConfig), args.Error(1)
}

// SetConfig mocks the SetConfig method
func (m *MockInstallationStore) SetConfig(cfg types.InstallationConfig) error {
	args := m.Called(cfg)
	return args.Error(0)
}

// GetStatus mocks the GetStatus method
func (m *MockInstallationStore) GetStatus() (*types.Status, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Status), args.Error(1)
}

// SetStatus mocks the SetStatus method
func (m *MockInstallationStore) SetStatus(status types.Status) error {
	args := m.Called(status)
	return args.Error(0)
}
