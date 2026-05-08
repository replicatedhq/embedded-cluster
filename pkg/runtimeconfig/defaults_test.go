package runtimeconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEmbeddedClusterLogsPath tests the log path.
func TestEmbeddedClusterLogsPath(t *testing.T) {
	got := EmbeddedClusterLogsPath()
	assert.Equal(t, "/var/log/embedded-cluster", got)
}
