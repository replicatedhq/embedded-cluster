package helpers

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// MoveFile moves a file from one location to another, overwriting the destination if it
// exists. File mode is preserved.
func MoveFile(src, dst string) error {
	srcinfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %s", src, err)
	}

	if srcinfo.IsDir() {
		return fmt.Errorf("move directory %s", src)
	}

	srcfp, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source file: %s", err)
	}
	defer srcfp.Close()

	opts := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	dstfp, err := os.OpenFile(dst, opts, srcinfo.Mode())
	if err != nil {
		return fmt.Errorf("open destination file: %s", err)
	}
	defer dstfp.Close()

	if _, err := io.Copy(dstfp, srcfp); err != nil {
		return fmt.Errorf("copy file: %s", err)
	}

	if err := dstfp.Sync(); err != nil {
		return fmt.Errorf("sync file: %s", err)
	}

	if err := os.Remove(src); err != nil {
		return fmt.Errorf("remove source file: %s", err)
	}

	return nil
}

// RemoveAll removes path if it's a file. If path is a directory, it only removes its contents.
// This is to handle the case where path is a bind mounted directory.
func RemoveAll(path string) error {
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat file: %w", err)
	}
	if os.IsNotExist(err) {
		return nil
	}
	if !info.IsDir() {
		return os.Remove(path)
	}
	d, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open directory: %w", err)
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}
	for _, name := range names {
		if err := os.RemoveAll(filepath.Join(path, name)); err != nil {
			return fmt.Errorf("remove %s: %w", name, err)
		}
	}
	return nil
}
