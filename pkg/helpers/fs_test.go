package helpers

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoveFile(t *testing.T) {
	srcContent := []byte("test")
	srcFile, err := CreateTemp("", "source-*")
	assert.NoError(t, err)
	defer Remove(srcFile.Name())
	defer srcFile.Close()

	_, err = srcFile.Write(srcContent)
	assert.NoError(t, err)

	dstFile, err := CreateTemp("", "destination-*")
	assert.NoError(t, err)
	defer Remove(dstFile.Name())
	defer dstFile.Close()

	err = MoveFile(srcFile.Name(), dstFile.Name())
	assert.NoError(t, err)

	_, err = Stat(dstFile.Name())
	assert.NoError(t, err)

	content, err := ReadFile(dstFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, srcContent, content)
}

func TestMoveFile_PreserveMode(t *testing.T) {
	srcContent := []byte("test")
	srcFile, err := CreateTemp("", "source-*")
	assert.NoError(t, err)
	defer Remove(srcFile.Name())
	defer srcFile.Close()

	_, err = srcFile.Write(srcContent)
	assert.NoError(t, err)

	err = os.Chmod(srcFile.Name(), 0755)
	assert.NoError(t, err)

	defer Remove("/tmp/testbin")
	err = MoveFile(srcFile.Name(), "/tmp/testbin")
	assert.NoError(t, err)

	info, err := Stat("/tmp/testbin")
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode())
}

func TestMoveFile_Directory(t *testing.T) {
	srcDir, err := MkdirTemp("", "sourcedir-*")
	assert.NoError(t, err)
	defer RemoveAll(srcDir)
	err = MoveFile(srcDir, "destination")
	assert.Error(t, err)
}

func TestMoveFile_Symlink(t *testing.T) {
	srcFile, err := CreateTemp("", "source-*")
	assert.NoError(t, err)
	_, err = srcFile.Write([]byte("test"))
	assert.NoError(t, err)
	defer Remove(srcFile.Name())
	defer srcFile.Close()

	symlinkPath := fmt.Sprintf("%s-symlink", srcFile.Name())
	err = os.Symlink(srcFile.Name(), symlinkPath)
	assert.NoError(t, err)
	defer Remove(symlinkPath)

	err = MoveFile(symlinkPath, "/tmp/destination")
	assert.NoError(t, err)
	defer RemoveAll("/tmp/destination")

	target, err := os.Readlink(symlinkPath)
	assert.Error(t, err)
	assert.Empty(t, target)
}

func TestRemoveAll(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) (string, bool)
		isDir bool
	}{
		{
			name: "remove file",
			setup: func(t *testing.T) (string, bool) {
				f, err := CreateTemp("", "test-file")
				if err != nil {
					t.Fatal(err)
				}
				return f.Name(), true
			},
		},
		{
			name: "remove directory",
			setup: func(t *testing.T) (string, bool) {
				dir, err := MkdirTemp("", "test-dir")
				if err != nil {
					t.Fatal(err)
				}
				if err := WriteFile(filepath.Join(dir, "file1"), []byte("test"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := WriteFile(filepath.Join(dir, "subdir", "file2"), []byte("test"), 0644); err != nil {
					t.Fatal(err)
				}
				return dir, true
			},
			isDir: true,
		},
		{
			name: "remove symlink",
			setup: func(t *testing.T) (string, bool) {
				f, err := CreateTemp("", "test-file")
				if err != nil {
					t.Fatal(err)
				}
				slink := filepath.Join(os.TempDir(), "test-symlink")
				if err := os.Symlink(f.Name(), slink); err != nil {
					t.Fatal(err)
				}
				return slink, true
			},
		},
		{
			name: "remove non-existent path",
			setup: func(t *testing.T) (string, bool) {
				return filepath.Join(os.TempDir(), "non-existent-path"), false
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			path, shouldExist := tt.setup(t)
			_, err := os.Lstat(path)
			if shouldExist {
				req.NoError(err)
			} else {
				req.Error(err)
			}

			if tt.isDir {
				// validate dir has contents
				d, err := Open(path)
				req.NoError(err)
				defer d.Close()

				names, err := d.Readdirnames(-1)
				req.NoError(err)
				req.NotEmpty(names)
			}

			// remove the path
			err = RemoveAll(path)
			req.NoError(err)

			if !tt.isDir {
				// file should be gone
				_, err := Lstat(path)
				req.Error(err)
			} else {
				// dir should exist and be empty
				_, err := Lstat(path)
				req.NoError(err)

				d, err := Open(path)
				req.NoError(err)
				defer d.Close()

				names, err := d.Readdirnames(-1)
				req.NoError(err)
				req.Empty(names)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		src         string
		dst         string
		mode        os.FileMode
		setup       func() error
		wantErr     bool
		errContains string
	}{
		{
			name: "successful copy",
			src:  filepath.Join(tmpDir, "source.txt"),
			dst:  filepath.Join(tmpDir, "subdir", "dest.txt"),
			mode: 0644,
			setup: func() error {
				return WriteFile(filepath.Join(tmpDir, "source.txt"), []byte("test content"), 0644)
			},
			wantErr: false,
		},
		{
			name:        "empty source",
			src:         "",
			dst:         filepath.Join(tmpDir, "dest.txt"),
			mode:        0644,
			setup:       func() error { return nil },
			wantErr:     true,
			errContains: "source path cannot be empty",
		},
		{
			name: "source is directory",
			src:  filepath.Join(tmpDir, "sourcedir"),
			dst:  filepath.Join(tmpDir, "dest.txt"),
			mode: 0644,
			setup: func() error {
				return os.Mkdir(filepath.Join(tmpDir, "sourcedir"), 0755)
			},
			wantErr:     true,
			errContains: "cannot copy directory",
		},
		{
			name: "source does not exist",
			src:  filepath.Join(tmpDir, "nonexistent.txt"),
			dst:  filepath.Join(tmpDir, "dest.txt"),
			mode: 0644,
			setup: func() error {
				return nil
			},
			wantErr:     true,
			errContains: "stat source file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			// Setup test case
			require.NoError(tt.setup(), "Setup failed")

			// Run CopyFile
			err := CopyFile(tt.src, tt.dst, tt.mode)

			// Check error
			if tt.wantErr {
				assert.Error(err, "CopyFile() should return error")
				if tt.errContains != "" {
					assert.Contains(err.Error(), tt.errContains, "Error message should contain expected text")
				}
				return
			}

			require.NoError(err, "CopyFile() should not return error")

			// Check if destination file exists
			_, err = Stat(tt.dst)
			require.NoError(err, "Destination file should exist")

			// Check file mode
			info, err := Stat(tt.dst)
			require.NoError(err, "Should be able to stat destination file")
			assert.Equal(tt.mode, info.Mode(), "Destination file mode should match expected mode")

			// Check file contents
			srcContent, err := ReadFile(tt.src)
			require.NoError(err, "Should be able to read source file")
			dstContent, err := ReadFile(tt.dst)
			require.NoError(err, "Should be able to read destination file")
			assert.Equal(string(srcContent), string(dstContent), "Destination file content should match source content")
		})
	}
}
