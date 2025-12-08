package install

import (
	"strings"
	"testing"

	appinstallstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogFn_Write(t *testing.T) {
	// Create store
	store := appinstallstore.NewMemoryStore()

	// Create concrete manager directly for testing utilities
	concreteManager := &appInstallManager{
		appInstallStore: store,
		logger:          logger.NewDiscardLogger(),
	}

	tests := []struct {
		name           string
		input          string
		expectedOutput string
		expectedInLogs bool
	}{
		{
			name:           "Single line output",
			input:          "Installing package X",
			expectedOutput: "[app] Installing package X",
			expectedInLogs: true,
		},
		{
			name:           "Output with newline",
			input:          "Installing package Y\n",
			expectedOutput: "[app] Installing package Y",
			expectedInLogs: true,
		},
		{
			name:           "Empty string",
			input:          "",
			expectedOutput: "[app]",
			expectedInLogs: true,
		},
		{
			name:           "Whitespace only",
			input:          "   \n\t  ",
			expectedOutput: "[app]",
			expectedInLogs: true,
		},
		{
			name:           "Multiple lines",
			input:          "Line 1\nLine 2\n",
			expectedOutput: "[app] Line 1\nLine 2",
			expectedInLogs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset the store by creating a new one for each test
			newStore := appinstallstore.NewMemoryStore()
			concreteManager.appInstallStore = newStore

			// Create new writer for the test
			logFn := concreteManager.logFn("app")

			// Write to log writer
			logFn(tt.input)

			// Check if logs were added
			logs, err := concreteManager.appInstallStore.GetLogs()
			require.NoError(t, err)

			if tt.expectedInLogs {
				assert.Contains(t, logs, tt.expectedOutput)
			} else {
				assert.Empty(t, logs)
			}
		})
	}
}

func TestLogFn_MultipleOperations(t *testing.T) {
	// Create concrete manager directly for testing utilities
	concreteManager := &appInstallManager{
		appInstallStore: appinstallstore.NewMemoryStore(),
		logger:          logger.NewDiscardLogger(),
	}

	// Create log function
	logFn := concreteManager.logFn("app")

	// Write multiple entries
	entries := []string{
		"Starting installation",
		"Downloading packages",
		"Installing dependencies",
		"Configuration complete",
	}

	for _, entry := range entries {
		logFn(entry)
	}

	// Verify all entries are in logs
	logs, err := concreteManager.appInstallStore.GetLogs()
	require.NoError(t, err)

	for _, entry := range entries {
		expected := "[app] " + entry
		assert.Contains(t, logs, expected)
	}

	// Verify entries are in correct order
	lines := strings.Split(strings.TrimSpace(logs), "\n")
	assert.Len(t, lines, len(entries))
	for i, entry := range entries {
		expected := "[app] " + entry
		assert.Equal(t, expected, lines[i])
	}
}

func TestLogFn_LargeOutput(t *testing.T) {
	// Create concrete manager directly for testing utilities
	concreteManager := &appInstallManager{
		appInstallStore: appinstallstore.NewMemoryStore(),
		logger:          logger.NewDiscardLogger(),
	}

	// Create log writer
	logFn := concreteManager.logFn("app")

	// Create a large output string
	largeOutput := strings.Repeat("A", 1000)

	// Write large output
	logFn(largeOutput)

	// Verify it was logged with prefix
	logs, err := concreteManager.appInstallStore.GetLogs()
	require.NoError(t, err)
	expected := "[app] " + largeOutput
	assert.Contains(t, logs, expected)
}

func TestLogFn_BinaryData(t *testing.T) {
	// Create concrete manager directly for testing utilities
	concreteManager := &appInstallManager{
		appInstallStore: appinstallstore.NewMemoryStore(),
		logger:          logger.NewDiscardLogger(),
	}

	// Create log writer
	logFn := concreteManager.logFn("app")

	// Write binary data (should still work as io.Writer)
	binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}
	logFn(string(binaryData))

	// Verify it was processed (though it may not be readable text)
	logs, err := concreteManager.appInstallStore.GetLogs()
	require.NoError(t, err)
	// Should contain the [app] prefix at minimum since binary data gets converted to string
	assert.Contains(t, logs, "[app]")
}
