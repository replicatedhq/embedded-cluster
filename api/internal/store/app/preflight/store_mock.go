package preflight

import (
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/mock"
)

var _ Store = (*MockStore)(nil)

// MockStore is a mock implementation of the Store interface
type MockStore struct {
	mock.Mock
}

// GetTitles mocks the GetTitles method
func (m *MockStore) GetTitles() ([]string, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// SetTitles mocks the SetTitles method
func (m *MockStore) SetTitles(titles []string) error {
	args := m.Called(titles)
	return args.Error(0)
}

// GetOutput mocks the GetOutput method
func (m *MockStore) GetOutput() (*types.PreflightsOutput, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*types.PreflightsOutput), args.Error(1)
}

// SetOutput mocks the SetOutput method
func (m *MockStore) SetOutput(output *types.PreflightsOutput) error {
	args := m.Called(output)
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
