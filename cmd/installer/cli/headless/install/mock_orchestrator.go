package install

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockOrchestrator is a mock implementation of the Orchestrator interface for testing.
// It records method calls and allows configuring return values.
type MockOrchestrator struct {
	mock.Mock
}

// NewMockOrchestrator creates a new MockOrchestrator instance
func NewMockOrchestrator() *MockOrchestrator {
	return &MockOrchestrator{}
}

// RunHeadlessInstall implements the Orchestrator interface
func (m *MockOrchestrator) RunHeadlessInstall(ctx context.Context, opts HeadlessInstallOptions) error {
	return m.Called(ctx, opts).Error(0)
}
