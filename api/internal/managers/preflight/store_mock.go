package preflight

import (
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/mock"
)

var _ HostPreflightStore = (*MockHostPreflightStore)(nil)

// MockHostPreflightStore is a mock implementation of the HostPreflightStore interface
type MockHostPreflightStore struct {
	mock.Mock
}

// GetTitles mocks the GetTitles method
func (m *MockHostPreflightStore) GetTitles() ([]string, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// SetTitles mocks the SetTitles method
func (m *MockHostPreflightStore) SetTitles(titles []string) error {
	args := m.Called(titles)
	return args.Error(0)
}

// GetOutput mocks the GetOutput method
func (m *MockHostPreflightStore) GetOutput() (*types.HostPreflightOutput, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.HostPreflightOutput), args.Error(1)
}

// SetOutput mocks the SetOutput method
func (m *MockHostPreflightStore) SetOutput(output *types.HostPreflightOutput) error {
	args := m.Called(output)
	return args.Error(0)
}

// GetStatus mocks the GetStatus method
func (m *MockHostPreflightStore) GetStatus() (*types.Status, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.Status), args.Error(1)
}

// SetStatus mocks the SetStatus method
func (m *MockHostPreflightStore) SetStatus(status *types.Status) error {
	args := m.Called(status)
	return args.Error(0)
}

// IsRunning mocks the IsRunning method
func (m *MockHostPreflightStore) IsRunning() bool {
	args := m.Called()
	return args.Bool(0)
}
