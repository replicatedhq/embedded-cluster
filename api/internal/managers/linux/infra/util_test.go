package infra

import (
	"testing"

	infrastore "github.com/replicatedhq/embedded-cluster/api/internal/store/linux/infra"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestInfraManager_logFn(t *testing.T) {
	tests := []struct {
		name      string
		component string
		format    string
		args      []interface{}
		expected  string
	}{
		{
			name:      "simple log message",
			component: "k0s",
			format:    "installing component",
			args:      []interface{}{},
			expected:  "[k0s] installing component",
		},
		{
			name:      "log message with arguments",
			component: "addons",
			format:    "installing %s version %s",
			args:      []interface{}{"helm", "v3.12.0"},
			expected:  "[addons] installing helm version v3.12.0",
		},
		{
			name:      "log message with multiple arguments",
			component: "helm",
			format:    "chart %s installed in namespace %s with values %v",
			args:      []interface{}{"test-chart", "default", map[string]string{"key": "value"}},
			expected:  "[helm] chart test-chart installed in namespace default with values map[key:value]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock store
			mockStore := &infrastore.MockStore{}
			mockStore.On("AddLogs", tt.expected).Return(nil)

			// Create a manager with the mock store
			manager := &infraManager{
				infraStore: mockStore,
				logger:     logger.NewDiscardLogger(),
			}

			// Call logFn and execute the returned function
			logFunc := manager.logFn(tt.component)
			logFunc(tt.format, tt.args...)

			// Verify the mock was called with expected arguments
			mockStore.AssertExpectations(t)
		})
	}
}

func TestInfraManager_logFn_StoreError(t *testing.T) {
	// Create a mock store that returns an error
	mockStore := &infrastore.MockStore{}
	mockStore.On("AddLogs", "[test] error message").Return(assert.AnError)

	// Create a manager with the mock store
	manager := &infraManager{
		infraStore: mockStore,
		logger:     logger.NewDiscardLogger(),
	}

	// Call logFn and execute the returned function
	logFunc := manager.logFn("test")

	// This should not panic even if AddLogs returns an error
	assert.NotPanics(t, func() {
		logFunc("error message")
	})

	// Verify the mock was called
	mockStore.AssertExpectations(t)
}
