package installation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/replicatedhq/embedded-cluster/api/types"
)

func TestStatusSetAndGet(t *testing.T) {
	manager := NewInstallationManager()

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

	// Verify the values match
	assert.Equal(t, statusToWrite.State, readStatus.State)
	assert.Equal(t, statusToWrite.Description, readStatus.Description)

	// Compare time with string format to avoid precision issues
	expectedTime := statusToWrite.LastUpdated.Format(time.RFC3339)
	actualTime := readStatus.LastUpdated.Format(time.RFC3339)
	assert.Equal(t, expectedTime, actualTime)
}

func TestSetRunningStatus(t *testing.T) {
	manager := NewInstallationManager()
	description := "Installation is running"

	err := manager.setRunningStatus(description)
	assert.NoError(t, err)

	status, err := manager.GetStatus()
	assert.NoError(t, err)

	assert.Equal(t, types.StateRunning, status.State)
	assert.Equal(t, description, status.Description)
	assert.NotZero(t, status.LastUpdated)
}

func TestSetFailedStatus(t *testing.T) {
	manager := NewInstallationManager()
	description := "Installation failed"

	err := manager.setFailedStatus(description)
	assert.NoError(t, err)

	status, err := manager.GetStatus()
	assert.NoError(t, err)

	assert.Equal(t, types.StateFailed, status.State)
	assert.Equal(t, description, status.Description)
	assert.NotZero(t, status.LastUpdated)
}

func TestSetCompletedStatus(t *testing.T) {
	tests := []struct {
		name        string
		state       types.State
		description string
	}{
		{
			name:        "completed with success state",
			state:       types.StateSucceeded,
			description: "completed successfully",
		},
		{
			name:        "completed with failed state",
			state:       types.StateFailed,
			description: "completed with errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewInstallationManager()

			err := manager.setCompletedStatus(tt.state, tt.description)
			assert.NoError(t, err)

			status, err := manager.GetStatus()
			assert.NoError(t, err)

			assert.Equal(t, tt.state, status.State)
			assert.Equal(t, tt.description, status.Description)
			assert.NotZero(t, status.LastUpdated)
		})
	}
}
