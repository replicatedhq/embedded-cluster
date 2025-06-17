package utils

import (
	"net"

	"github.com/stretchr/testify/mock"
)

var _ NetUtils = (*MockNetUtils)(nil)

// MockNetUtils is a mock implementation of the NetUtils interface
type MockNetUtils struct {
	mock.Mock
}

// ListValidNetworkInterfaces mocks the ListValidNetworkInterfaces method
func (m *MockNetUtils) ListValidNetworkInterfaces() ([]string, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// DetermineBestNetworkInterface mocks the DetermineBestNetworkInterface method
func (m *MockNetUtils) DetermineBestNetworkInterface() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// FirstValidIPNet mocks the FirstValidIPNet method
func (m *MockNetUtils) FirstValidIPNet(networkInterface string) (*net.IPNet, error) {
	args := m.Called(networkInterface)
	return args.Get(0).(*net.IPNet), args.Error(1)
}

// FirstValidAddress mocks the FirstValidAddress method
func (m *MockNetUtils) FirstValidAddress(networkInterface string) (string, error) {
	args := m.Called(networkInterface)
	return args.String(0), args.Error(1)
}
