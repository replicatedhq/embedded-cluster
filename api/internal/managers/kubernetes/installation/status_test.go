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
