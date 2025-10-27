package util

import (
	"os"
	"testing"

	"github.com/replicatedhq/embedded-cluster/pkg/helpers"
)

func WriteTempFile(t *testing.T, pattern string, data []byte, perm os.FileMode) string {
	f, err := os.CreateTemp(t.TempDir(), pattern)
	if err != nil {
		t.Fatalf("failed to create temp file: %s", err)
	}
	defer f.Close()
	err = helpers.WriteFile(f.Name(), data, perm)
	if err != nil {
		t.Fatalf("failed to write temp file %s: %s", f.Name(), err)
	}
	return f.Name()
}

// TempDirForHostMount is needed because of test failure "cleanup: unlinkat ...: permission denied"
func TempDirForHostMount(t *testing.T, pattern string) string {
	dir, err := helpers.MkdirTemp("", pattern)
	if err != nil {
		t.Fatalf("failed to create temp dir: %s", err)
	}
	t.Cleanup(func() { _ = helpers.RemoveAll(dir) })
	return dir
}
