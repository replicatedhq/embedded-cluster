package config

import (
	kotsv1beta1 "github.com/replicatedhq/kotskinds/apis/kots/v1beta1"
	"github.com/stretchr/testify/mock"
)

var _ Store = (*MockStore)(nil)

// MockStore is a mock implementation of the Store interface
type MockStore struct {
	mock.Mock
}

// GetConfigValues mocks the GetConfigValues method
func (m *MockStore) GetConfigValues() (kotsv1beta1.ConfigValues, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return kotsv1beta1.ConfigValues{}, args.Error(1)
	}
	return args.Get(0).(kotsv1beta1.ConfigValues), args.Error(1)
}

// SetConfigValues mocks the SetConfigValues method
func (m *MockStore) SetConfigValues(configValues kotsv1beta1.ConfigValues) error {
	args := m.Called(configValues)
	return args.Error(0)
}
