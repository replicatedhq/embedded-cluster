package helpers

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMoveFile(t *testing.T) {
	srcContent := []byte("test")
	srcFile, err := os.CreateTemp("", "source-*")
	assert.NoError(t, err)
	defer os.Remove(srcFile.Name())
	defer srcFile.Close()

	_, err = srcFile.Write(srcContent)
	assert.NoError(t, err)

	dstFile, err := os.CreateTemp("", "destination-*")
	assert.NoError(t, err)
	defer os.Remove(dstFile.Name())
	defer dstFile.Close()

	err = MoveFile(srcFile.Name(), dstFile.Name())
	assert.NoError(t, err)

	_, err = os.Stat(dstFile.Name())
	assert.NoError(t, err)

	content, err := os.ReadFile(dstFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, srcContent, content)
}

func TestMoveFile_PreserveMode(t *testing.T) {
	srcContent := []byte("test")
	srcFile, err := os.CreateTemp("", "source-*")
	assert.NoError(t, err)
	defer os.Remove(srcFile.Name())
	defer srcFile.Close()

	_, err = srcFile.Write(srcContent)
	assert.NoError(t, err)

	err = os.Chmod(srcFile.Name(), 0755)
	assert.NoError(t, err)

	defer os.Remove("/tmp/testbin")
	err = MoveFile(srcFile.Name(), "/tmp/testbin")
	assert.NoError(t, err)

	info, err := os.Stat("/tmp/testbin")
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode())
}

func TestMoveFile_Directory(t *testing.T) {
	srcDir, err := os.MkdirTemp("", "sourcedir-*")
	assert.NoError(t, err)
	defer os.RemoveAll(srcDir)
	err = MoveFile(srcDir, "destination")
	assert.Error(t, err)
}

func TestMoveFile_Symlink(t *testing.T) {
	srcFile, err := os.CreateTemp("", "source-*")
	assert.NoError(t, err)
	_, err = srcFile.Write([]byte("test"))
	assert.NoError(t, err)
	defer os.Remove(srcFile.Name())
	defer srcFile.Close()

	symlinkPath := fmt.Sprintf("%s-symlink", srcFile.Name())
	err = os.Symlink(srcFile.Name(), symlinkPath)
	assert.NoError(t, err)
	defer os.Remove(symlinkPath)

	err = MoveFile(symlinkPath, "/tmp/destination")
	assert.NoError(t, err)
	defer os.RemoveAll("/tmp/destination")

	target, err := os.Readlink(symlinkPath)
	assert.Error(t, err)
	assert.Empty(t, target)
}
