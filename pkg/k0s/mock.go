package k0s

import (
	"context"

	"github.com/stretchr/testify/mock"
)

var _ K0sInterface = (*MockK0s)(nil)

// MockK0s is a mock implementation of the K0sInterface
type MockK0s struct {
	mock.Mock
}

// GetStatus mocks the GetStatus method
func (m *MockK0s) GetStatus(ctx context.Context) (*K0sStatus, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*K0sStatus), args.Error(1)
}

// IsInstalled mocks the IsInstalled method
func (m *MockK0s) IsInstalled() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

// WaitForK0s mocks the WaitForK0s method
func (m *MockK0s) WaitForK0s() error {
	args := m.Called()
	return args.Error(0)
}
