package util

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func WriteTempFile(t *testing.T, name string, data []byte, perm os.FileMode) string {
	filename := filepath.Join(t.TempDir(), name)
	err := os.WriteFile(filename, data, perm)
	if err != nil {
		t.Fatalf("failed to write temp file %s: %s", filename, err)
	}
	return filename
}

func TmpNameForHostMount(t *testing.T, name string) string {
	return filepath.Join("/tmp", t.Name(), strconv.FormatInt(time.Now().UnixNano(), 10), name)
}
