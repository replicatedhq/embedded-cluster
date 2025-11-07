package mock

import (
	"runtime"
	"strings"

	"github.com/stretchr/testify/mock"
)

// Mock embeds testify/mock.Mock and provides helper methods for creating mocks
// with default stub behavior.
//
// When embedding this in your mock structs, mock methods can use MaybeRegisterCall
// to automatically provide sensible defaults when tests don't set explicit expectations.
//
// Example usage:
//
//	type MockService struct {
//	    mock.Mock // Replace with: ecmock.Mock
//	}
//
//	func (m *MockService) GetData(id string) (string, error) {
//	    if registered, args := m.MaybeRegisterCall(id); registered {
//	        return args.String(0), args.Error(1)
//	    }
//	    // Return default stub when no expectation is set
//	    return "", nil
//	}
type Mock struct {
	mock.Mock
}

// MaybeRegisterCall checks if there are explicit expectations registered for a method.
// It automatically detects the calling method name using runtime.Caller.
//
// If expectations exist, it calls m.MethodCalled() and returns (true, arguments).
// If no expectations exist, it returns (false, nil) and the caller should use default stubs.
//
// This allows mocks to have default "happy path" behavior without requiring
// every test to set up expectations for methods they don't care about.
//
// Usage in mock methods:
//
//	func (m *MockService) GetData(id string) (string, error) {
//	    if registered, args := m.MaybeRegisterCall(id); registered {
//	        return args.String(0), args.Error(1)
//	    }
//	    // Return default stub values
//	    return "default-data", nil
//	}
func (m *Mock) MaybeRegisterCall(args ...interface{}) (bool, mock.Arguments) {
	// Get the method name of the caller
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		panic("Couldn't get the caller information")
	}
	fullMethodName := runtime.FuncForPC(pc).Name()

	// Extract just the method name from the full path
	// fullMethodName is like: "github.com/user/pkg.(*Type).MethodName"
	// We need to extract "MethodName"
	parts := strings.Split(fullMethodName, ".")
	methodName := parts[len(parts)-1]

	// Check if any expectations are registered for this method
	for _, call := range m.ExpectedCalls {
		if call.Method == methodName {
			// Found an explicit expectation - use normal mock behavior
			return true, m.MethodCalled(methodName, args...)
		}
	}

	// No expectations found - caller should use default stub
	return false, nil
}
