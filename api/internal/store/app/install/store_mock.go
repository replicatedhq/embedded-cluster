package install

import (
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/mock"
)

var _ Store = (*MockStore)(nil)

// MockStore is a mock implementation of the Store interface
type MockStore struct {
	mock.Mock
}

// Get mocks the Get method
func (m *MockStore) Get() (types.AppInstall, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return types.AppInstall{}, args.Error(1)
	}
	return args.Get(0).(types.AppInstall), args.Error(1)
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

// SetStatusDesc mocks the SetStatusDesc method
func (m *MockStore) SetStatusDesc(desc string) error {
	args := m.Called(desc)
	return args.Error(0)
}

// AddLogs mocks the AddLogs method
func (m *MockStore) AddLogs(logs string) error {
	args := m.Called(logs)
	return args.Error(0)
}

// GetLogs mocks the GetLogs method
func (m *MockStore) GetLogs() (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}

// SetComponentStatus mocks the SetComponentStatus method
func (m *MockStore) SetComponentStatus(componentName string, status types.Status) error {
	args := m.Called(componentName, status)
	return args.Error(0)
}

// RegisterComponents mocks the RegisterComponents method
func (m *MockStore) RegisterComponents(componentNames []string) error {
	args := m.Called(componentNames)
	return args.Error(0)
}
