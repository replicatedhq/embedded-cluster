package systemd

import (
	"context"

	"github.com/stretchr/testify/mock"
)

var (
	_ dbusInterface = (*MockDBus)(nil)
)

type MockDBus struct {
	mock.Mock
}

func (m *MockDBus) EnableAndStart(ctx context.Context, unit string) error {
	args := m.Called(ctx, unit)
	return args.Error(0)
}

func (m *MockDBus) Stop(ctx context.Context, unit string) error {
	args := m.Called(ctx, unit)
	return args.Error(0)
}

func (m *MockDBus) Disable(ctx context.Context, unit string) error {
	args := m.Called(ctx, unit)
	return args.Error(0)
}

func (m *MockDBus) Restart(ctx context.Context, unit string) error {
	args := m.Called(ctx, unit)
	return args.Error(0)
}

func (m *MockDBus) IsActive(ctx context.Context, unit string) (bool, error) {
	args := m.Called(ctx, unit)
	return args.Bool(0), args.Error(1)
}

func (m *MockDBus) IsEnabled(ctx context.Context, unit string) (bool, error) {
	args := m.Called(ctx, unit)
	return args.Bool(0), args.Error(1)
}

func (m *MockDBus) UnitExists(ctx context.Context, unit string) (bool, error) {
	args := m.Called(ctx, unit)
	return args.Bool(0), args.Error(1)
}

func (m *MockDBus) Reload(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
