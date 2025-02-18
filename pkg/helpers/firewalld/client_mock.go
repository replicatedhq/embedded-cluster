package firewalld

import (
	"context"

	"github.com/stretchr/testify/mock"
)

var (
	_ Interface = (*MockClient)(nil)
)

type MockClient struct {
	mock.Mock
}

func (m *MockClient) ZoneExists(ctx context.Context, zone string) (bool, error) {
	args := m.Called(ctx, zone)
	return args.Bool(0), args.Error(1)
}

func (m *MockClient) NewZone(ctx context.Context, zone string, opts ...Option) error {
	args := m.Called(ctx, zone, opts)
	return args.Error(0)
}

func (m *MockClient) SetZoneTarget(ctx context.Context, target string, opts ...Option) error {
	args := m.Called(ctx, target, opts)
	return args.Error(0)
}

func (m *MockClient) AddSourceToZone(ctx context.Context, source string, opts ...Option) error {
	args := m.Called(ctx, source, opts)
	return args.Error(0)
}

func (m *MockClient) AddInterfaceToZone(ctx context.Context, iface string, opts ...Option) error {
	args := m.Called(ctx, iface, opts)
	return args.Error(0)
}

func (m *MockClient) AddPortToZone(ctx context.Context, port string, opts ...Option) error {
	args := m.Called(ctx, port, opts)
	return args.Error(0)
}

func (m *MockClient) Reload(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
