package prompts

import (
	"github.com/stretchr/testify/mock"
)

// MockPrompt is a mock implementation of the Prompt interface for testing
type MockPrompt struct {
	mock.Mock
}

// Confirm mocks the Confirm method
func (m *MockPrompt) Confirm(msg string, defvalue bool) (bool, error) {
	args := m.Called(msg, defvalue)
	return args.Bool(0), args.Error(1)
}

// Password mocks the Password method
func (m *MockPrompt) Password(msg string) (string, error) {
	args := m.Called(msg)
	return args.String(0), args.Error(1)
}

// Input mocks the Input method
func (m *MockPrompt) Input(msg string, defvalue string, required bool) (string, error) {
	args := m.Called(msg, defvalue, required)
	return args.String(0), args.Error(1)
}

// NewMock creates a new mock prompt instance
func NewMock() *MockPrompt {
	return &MockPrompt{}
}
