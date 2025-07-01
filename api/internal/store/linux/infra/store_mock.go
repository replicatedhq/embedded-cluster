package infra

import (
	"github.com/replicatedhq/embedded-cluster/api/types"
	"github.com/stretchr/testify/mock"
)

var _ Store = (*MockStore)(nil)

// MockStore is a mock implementation of Store
type MockStore struct {
	mock.Mock
}

func (m *MockStore) Get() (types.LinuxInfra, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return types.LinuxInfra{}, args.Error(1)
	}
	return args.Get(0).(types.LinuxInfra), args.Error(1)
}

func (m *MockStore) GetStatus() (types.Status, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return types.Status{}, args.Error(1)
	}
	return args.Get(0).(types.Status), args.Error(1)
}

func (m *MockStore) SetStatus(status types.Status) error {
	args := m.Called(status)
	return args.Error(0)
}

func (m *MockStore) SetStatusDesc(desc string) error {
	args := m.Called(desc)
	return args.Error(0)
}

func (m *MockStore) RegisterComponent(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockStore) SetComponentStatus(name string, status types.Status) error {
	args := m.Called(name, status)
	return args.Error(0)
}

func (m *MockStore) AddLogs(logs string) error {
	args := m.Called(logs)
	return args.Error(0)
}

func (m *MockStore) GetLogs() (string, error) {
	args := m.Called()
	return args.Get(0).(string), args.Error(1)
}
