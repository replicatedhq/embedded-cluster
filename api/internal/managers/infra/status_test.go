package infra

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func TestInfraWithLogs(t *testing.T) {
	manager := NewInfraManager()

	// Add some logs through the internal logging mechanism
	logFn := manager.logFn("test")
	logFn("Test log message")
	logFn("Another log message with arg: %s", "value")

	// Get the infra and verify logs are included
	infra, err := manager.Get()
	assert.NoError(t, err)
	assert.Contains(t, infra.Logs, "[test] Test log message")
	assert.Contains(t, infra.Logs, "[test] Another log message with arg: value")
}

func TestInstallDidRun(t *testing.T) {
	tests := []struct {
		name           string
		currentStatus  *types.Status
		expectedResult bool
		expectedErr    bool
	}{
		{
			name:           "nil status",
			currentStatus:  nil,
			expectedResult: false,
			expectedErr:    false,
		},
		{
			name: "empty state",
			currentStatus: &types.Status{
				State: "",
			},
			expectedResult: false,
			expectedErr:    false,
		},
		{
			name: "pending state",
			currentStatus: &types.Status{
				State: types.StatePending,
			},
			expectedResult: false,
			expectedErr:    false,
		},
		{
			name: "running state",
			currentStatus: &types.Status{
				State: types.StateRunning,
			},
			expectedResult: true,
			expectedErr:    false,
		},
		{
			name: "failed state",
			currentStatus: &types.Status{
				State: types.StateFailed,
			},
			expectedResult: true,
			expectedErr:    false,
		},
		{
			name: "succeeded state",
			currentStatus: &types.Status{
				State: types.StateSucceeded,
			},
			expectedResult: true,
			expectedErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewInfraManager()
			if tt.currentStatus != nil {
				manager.SetStatus(*tt.currentStatus)
			}
			result, err := manager.installDidRun()

			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}
		})
	}
}
