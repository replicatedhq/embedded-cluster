package helpers

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type MultiError struct {
	Errors []error
}

func (e *MultiError) Add(err error) {
	e.Errors = append(e.Errors, err)
}

func (e *MultiError) ErrorOrNil() error {
	switch len(e.Errors) {
	case 0:
		return nil
	case 1:
		return e.Errors[0]
	default:
		return fmt.Errorf("errors: %q", e.Errors)
	}
}

// MoveFile moves a file from one location to another, overwriting the destination if it
// exists. File mode is preserved.
func MoveFile(src, dst string) error {
	srcinfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcinfo.IsDir() {
		return fmt.Errorf("cannot move directory %s", src)
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
	var me MultiError
	for _, name := range names {
		if err := os.RemoveAll(filepath.Join(path, name)); err != nil {
			me.Add(fmt.Errorf("remove %s: %w", name, err))
		}
	}
	return me.ErrorOrNil()
}

func (h *Helpers) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

// CopyFile copies a file from src to dst, creating parent directories as needed.
// The destination file will be created with the specified mode.
func CopyFile(src, dst string, mode os.FileMode) error {
	if src == "" {
		return fmt.Errorf("source path cannot be empty")
	}

	srcinfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat source file: %w", err)
	}

	if srcinfo.IsDir() {
		return fmt.Errorf("cannot copy directory %s", src)
	}

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("create parent directories: %w", err)
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read source file: %w", err)
	}

	if err := WriteFile(dst, data, mode); err != nil {
		return fmt.Errorf("write destination file: %w", err)
	}

	return nil
}
