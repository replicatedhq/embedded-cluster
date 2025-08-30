package helm

import (
	"context"

	"github.com/stretchr/testify/mock"
)

var _ BinaryExecutor = (*MockBinaryExecutor)(nil)

// MockBinaryExecutor is a mock implementation of BinaryExecutor for testing
type MockBinaryExecutor struct {
	mock.Mock
}

// ExecuteCommand mocks the ExecuteCommand method
func (m *MockBinaryExecutor) ExecuteCommand(ctx context.Context, env map[string]string, args ...string) (string, string, error) {
	callArgs := m.Called(ctx, env, args)
	return callArgs.String(0), callArgs.String(1), callArgs.Error(2)
}
