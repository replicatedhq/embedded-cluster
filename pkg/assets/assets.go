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

	"k8s.io/klog/v2"
)

// Stage stages the embedded file or directory to the given destination.
func Stage(fsys embed.FS, dest string, name string, mode os.FileMode) error {
	p := filepath.Join(dest, name)
	klog.Infof("Staging %q", p)

	info, err := fs.Stat(fsys, name)
	if err != nil {
		return fmt.Errorf("stat %s: %w", name, err)
	}
	if info.IsDir() {
		err := fs.WalkDir(fsys, name, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return fmt.Errorf("walk %s: %w", path, err)
			}
			if path == filepath.Clean(name) {
				return nil
			}
			return Stage(fsys, dest, path, mode)
		})
		if err != nil {
			return fmt.Errorf("walk %s: %w", name, err)
		}
		return nil
	}

	err = os.MkdirAll(filepath.Dir(p), 0755)
	if err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(p), err)
	}

	src, err := fsys.Open(name)
	if err != nil {
		return fmt.Errorf("open %s: %w", name, err)
	}
	defer func() {
		_ = src.Close()
	}()

	dst, err := os.Create(p)
	if err != nil {
		return fmt.Errorf("create %s: %w", p, err)
	}
	defer func() {
		_ = dst.Close()
	}()

	_, err = io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("write %s: %w", p, err)
	}

	err = os.Chmod(p, mode)
	if err != nil {
		return fmt.Errorf("chmod %s: %w", p, err)
	}

	return nil
}

// BinPath searches for a binary on disk:
// - in the BinDir folder,
// - in the PATH.
// The first to be found is the one returned.
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
