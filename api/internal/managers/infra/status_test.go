package infra

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func TestStatusSetAndGet(t *testing.T) {
	manager := NewInfraManager()

	// Test writing a status
	statusToWrite := types.Status{
		State:       types.StateRunning,
		Description: "Installation in progress",
		LastUpdated: time.Now().UTC().Truncate(time.Second), // Truncate to avoid time precision issues
	}

	err := manager.SetStatus(statusToWrite)
	assert.NoError(t, err)

	// Test reading it back
	readStatus, err := manager.GetStatus()
	assert.NoError(t, err)
	assert.NotNil(t, readStatus)

	// Verify the values match
	assert.Equal(t, statusToWrite.State, readStatus.State)
	assert.Equal(t, statusToWrite.Description, readStatus.Description)

	// Compare time with string format to avoid precision issues
	expectedTime := statusToWrite.LastUpdated.Format(time.RFC3339)
	actualTime := readStatus.LastUpdated.Format(time.RFC3339)
	assert.Equal(t, expectedTime, actualTime)
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
