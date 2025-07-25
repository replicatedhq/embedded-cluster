package installation

import (
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/mock"
)

var _ Store = (*MockStore)(nil)

// MockStore is a mock implementation of the InstallationStore interface
type MockStore struct {
	mock.Mock
}

// GetConfig mocks the GetConfig method
func (m *MockStore) GetConfig() (types.LinuxInstallationConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return types.LinuxInstallationConfig{}, args.Error(1)
	}
	return args.Get(0).(types.LinuxInstallationConfig), args.Error(1)
}

// SetConfig mocks the SetConfig method
func (m *MockStore) SetConfig(cfg types.LinuxInstallationConfig) error {
	args := m.Called(cfg)
	return args.Error(0)
}

// GetStatus mocks the GetStatus method
func (m *MockStore) GetStatus() (types.Status, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return types.Status{}, args.Error(1)
	}
	return args.Get(0).(types.Status), args.Error(1)
}

// SetStatus mocks the SetStatus method
func (m *MockStore) SetStatus(status types.Status) error {
	args := m.Called(status)
	return args.Error(0)
}
