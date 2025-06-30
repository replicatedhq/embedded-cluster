package cloudutils

import "github.com/stretchr/testify/mock"

var _ Interface = (*MockCloudUtils)(nil)

// MockCloudUtils is a mock implementation of the CloudUtilsInterface
type MockCloudUtils struct {
	mock.Mock
}

func (m *MockCloudUtils) TryDiscoverPublicIP() string {
	args := m.Called()
	return args.String(0)
}
