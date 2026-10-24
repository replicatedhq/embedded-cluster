package cmx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalToolPathFromEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tool")
	if err := os.WriteFile(path, []byte("tool"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEST_TOOL_PATH", path)

	got, err := localToolPath("TEST_TOOL_PATH", "missing-test-tool")
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("localToolPath() = %q, want %q", got, path)
	}
}

func TestLocalToolPathRejectsMissingTool(t *testing.T) {
	t.Setenv("TEST_TOOL_PATH", "")
	t.Setenv("PATH", t.TempDir())

	_, err := localToolPath("TEST_TOOL_PATH", "missing-test-tool")
	if err == nil || !strings.Contains(err.Error(), "TEST_TOOL_PATH is not set") {
		t.Fatalf("localToolPath() error = %v, want missing environment error", err)
	}
}
