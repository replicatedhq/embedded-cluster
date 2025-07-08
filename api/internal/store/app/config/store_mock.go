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

// Get mocks the Get method
func (m *MockStore) Get() (kotsv1beta1.Config, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return kotsv1beta1.Config{}, args.Error(1)
	}
	return args.Get(0).(kotsv1beta1.Config), args.Error(1)
}

// Set mocks the Set method
func (m *MockStore) Set(config kotsv1beta1.Config) error {
	args := m.Called(config)
	return args.Error(0)
}
