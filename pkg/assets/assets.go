/*
Package assets provides a way to extract assets from the embedded filesystem.
*/
package assets

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// Stage stages the embedded file or directory to the given destination.
func Stage(fsys embed.FS, dest string, name string, mode os.FileMode) error {
	path := filepath.Join(dest, name)
	logrus.Infof("Staging %s to %s", name, path)
	info, err := fs.Stat(fsys, name)
	if err != nil {
		return fmt.Errorf("stat %s: %w", name, err)
	}
	if info.IsDir() {
		if err := fs.WalkDir(fsys, name, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("walk %s: %w", path, err)
			}
			if path == filepath.Clean(name) {
				return nil
			}
			return Stage(fsys, dest, path, mode)
		}); err != nil {
			return fmt.Errorf("failed to walk %s directory: %w", name, err)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to mkdir %s: %w", filepath.Dir(path), err)
	}
	src, err := fsys.Open(name)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", name, err)
	}
	defer func() {
		_ = src.Close()
	}()
	dst, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", path, err)
	}
	defer func() {
		_ = dst.Close()
	}()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", path, err)
	}
	return nil
}

// BinPath searches for a binary on disk, looks in BinDir and in PATH. The
// first to be found is the one returned.
func BinPath(name string, binDir string) string {
	// Look into the BinDir folder.
	path := filepath.Join(binDir, name)
	if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
		return path
	}
	// If we still haven't found the executable, look for it in the PATH.
	if path, err := exec.LookPath(name); err == nil {
		path, _ := filepath.Abs(path)
		return path
	}
	return name
}
