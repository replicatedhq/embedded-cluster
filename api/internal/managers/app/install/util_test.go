package install

import (
	"strings"
	"testing"

	appinstallstore "github.com/replicatedhq/embedded-cluster/api/internal/store/app/install"
	"github.com/replicatedhq/embedded-cluster/api/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogWriter_Write(t *testing.T) {
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
			expectedOutput: "[kots] Installing package X",
			expectedInLogs: true,
		},
		{
			name:           "Output with newline",
			input:          "Installing package Y\n",
			expectedOutput: "[kots] Installing package Y",
			expectedInLogs: true,
		},
		{
			name:           "Empty string",
			input:          "",
			expectedOutput: "",
			expectedInLogs: false,
		},
		{
			name:           "Whitespace only",
			input:          "   \n\t  ",
			expectedOutput: "",
			expectedInLogs: false,
		},
		{
			name:           "Multiple lines",
			input:          "Line 1\nLine 2\n",
			expectedOutput: "[kots] Line 1\nLine 2",
			expectedInLogs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset the store by creating a new one for each test
			newStore := appinstallstore.NewMemoryStore()
			concreteManager.appInstallStore = newStore
			
			// Create new writer for the test
			testWriter := concreteManager.newLogWriter()

			// Write to log writer
			n, err := testWriter.Write([]byte(tt.input))
			assert.NoError(t, err)
			assert.Equal(t, len(tt.input), n)

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

func TestLogWriter_WriteMultipleOperations(t *testing.T) {
	// Create concrete manager directly for testing utilities
	concreteManager := &appInstallManager{
		appInstallStore: appinstallstore.NewMemoryStore(),
		logger:          logger.NewDiscardLogger(),
	}

	// Create log writer
	writer := concreteManager.newLogWriter()

	// Write multiple entries
	entries := []string{
		"Starting installation",
		"Downloading packages",
		"Installing dependencies",
		"Configuration complete",
	}

	for _, entry := range entries {
		n, err := writer.Write([]byte(entry))
		assert.NoError(t, err)
		assert.Equal(t, len(entry), n)
	}

	// Verify all entries are in logs
	logs, err := concreteManager.appInstallStore.GetLogs()
	require.NoError(t, err)

	for _, entry := range entries {
		expected := "[kots] " + entry
		assert.Contains(t, logs, expected)
	}

	// Verify entries are in correct order
	lines := strings.Split(strings.TrimSpace(logs), "\n")
	assert.Len(t, lines, len(entries))
	for i, entry := range entries {
		expected := "[kots] " + entry
		assert.Equal(t, expected, lines[i])
	}
}

func TestLogWriter_LargeOutput(t *testing.T) {
	// Create concrete manager directly for testing utilities
	concreteManager := &appInstallManager{
		appInstallStore: appinstallstore.NewMemoryStore(),
		logger:          logger.NewDiscardLogger(),
	}

	// Create log writer
	writer := concreteManager.newLogWriter()

	// Create a large output string
	largeOutput := strings.Repeat("A", 1000)

	// Write large output
	n, err := writer.Write([]byte(largeOutput))
	assert.NoError(t, err)
	assert.Equal(t, 1000, n)

	// Verify it was logged with prefix
	logs, err := concreteManager.appInstallStore.GetLogs()
	require.NoError(t, err)
	expected := "[kots] " + largeOutput
	assert.Contains(t, logs, expected)
}

func TestLogWriter_BinaryData(t *testing.T) {
	// Create concrete manager directly for testing utilities
	concreteManager := &appInstallManager{
		appInstallStore: appinstallstore.NewMemoryStore(),
		logger:          logger.NewDiscardLogger(),
	}

	// Create log writer
	writer := concreteManager.newLogWriter()

	// Write binary data (should still work as io.Writer)
	binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}
	n, err := writer.Write(binaryData)
	assert.NoError(t, err)
	assert.Equal(t, len(binaryData), n)

	// Verify it was processed (though it may not be readable text)
	logs, err := concreteManager.appInstallStore.GetLogs()
	require.NoError(t, err)
	// Should contain the [kots] prefix at minimum since binary data gets converted to string
	assert.Contains(t, logs, "[kots]")
}