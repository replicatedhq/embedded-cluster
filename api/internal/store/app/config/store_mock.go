package config

import (
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/mock"
)

var _ Store = (*MockStore)(nil)

// MockStore is a mock implementation of the Store interface
type MockStore struct {
	mock.Mock
}

// GetConfigValues mocks the GetConfigValues method
func (m *MockStore) GetConfigValues() (types.AppConfigValues, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(types.AppConfigValues), args.Error(1)
}

// SetConfigValues mocks the SetConfigValues method
func (m *MockStore) SetConfigValues(configValues types.AppConfigValues) error {
	args := m.Called(configValues)
	return args.Error(0)
}
