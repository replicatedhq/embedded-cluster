package helpers

import (
	"fmt"
	"io"
	"os"
)

// MoveFile moves a file from one location to another, overwriting the destination if it
// exists. File mode is preserved.
func MoveFile(src, dst string) error {
	srcinfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("unable to stat %s: %s", src, err)
	}

	if srcinfo.IsDir() {
		return fmt.Errorf("unable to move directory %s", src)
	}

	srcfp, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("unable to open source file: %s", err)
	}
	defer srcfp.Close()

	opts := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	dstfp, err := os.OpenFile(dst, opts, srcinfo.Mode())
	if err != nil {
		return fmt.Errorf("unable to open destination file: %s", err)
	}
	defer dstfp.Close()

	if _, err := io.Copy(dstfp, srcfp); err != nil {
		return fmt.Errorf("unable to copy file: %s", err)
	}

	if err := dstfp.Sync(); err != nil {
		return fmt.Errorf("unable to sync file: %s", err)
	}

	if err := os.Remove(src); err != nil {
		return fmt.Errorf("unable to remove source file: %s", err)
	}

	return nil
}
