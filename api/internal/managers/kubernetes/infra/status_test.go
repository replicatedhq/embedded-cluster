package infra

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInfraWithLogs(t *testing.T) {
	manager, err := NewInfraManager()
	require.NoError(t, err)

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
