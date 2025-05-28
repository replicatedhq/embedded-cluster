package runtimeconfig

import (
	"os"
	"path/filepath"
	"testing"

	ecv1beta1 "github.com/replicatedhq/embedded-cluster/kinds/apis/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanup_EmptyTmpDir(t *testing.T) {
	// Save original runtime config to restore after test
	originalConfig := runtimeConfig
	defer func() {
		runtimeConfig = originalConfig
	}()

	// Create a test config with a temporary directory
	testTmpParent := t.TempDir()
	testConfig := &ecv1beta1.RuntimeConfigSpec{
		DataDir: testTmpParent,
	}
	runtimeConfig = testConfig

	// Get the tmp directory path that will be used
	tmpDir := filepath.Join(testTmpParent, "tmp")

	// Create the tmp directory
	err := os.MkdirAll(tmpDir, 0755)
	require.NoError(t, err, "Failed to create test tmp directory")

	// Create test files and subdirectories
	testFiles := []string{"file1.txt", "file2.log"}
	testDirs := []string{"subdir1", "subdir2"}

	for _, file := range testFiles {
		path := filepath.Join(tmpDir, file)
		err := os.WriteFile(path, []byte("test content"), 0644)
		require.NoError(t, err, "Failed to create test file")
	}

	for _, dir := range testDirs {
		path := filepath.Join(tmpDir, dir)
		err := os.Mkdir(path, 0755)
		require.NoError(t, err, "Failed to create test directory")

		// Add a file in the subdirectory
		subFile := filepath.Join(path, "subfile.txt")
		err = os.WriteFile(subFile, []byte("subdir content"), 0644)
		require.NoError(t, err, "Failed to create file in subdirectory")
	}

	// Verify test files and directories were created
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err, "Failed to read test directory")
	require.Len(t, entries, len(testFiles)+len(testDirs), "Incorrect number of test items created")

	// Call the function under test
	Cleanup()

	// Verify the directory still exists
	_, err = os.Stat(tmpDir)
	assert.NoError(t, err, "The tmp directory should still exist")

	// Verify the directory is empty
	entries, err = os.ReadDir(tmpDir)
	assert.NoError(t, err, "Failed to read tmp directory after cleanup")
	assert.Empty(t, entries, "The tmp directory should be empty after cleanup")
}
