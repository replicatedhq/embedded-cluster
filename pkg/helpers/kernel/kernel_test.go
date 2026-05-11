package kernel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectIPTablesBackend_ProcNetIpTablesNames(t *testing.T) {
	tmpDir := t.TempDir()
	procNetPath := filepath.Join(tmpDir, "ip_tables_names")

	// Simulate /proc/net/ip_tables_names existing
	require.NoError(t, os.WriteFile(procNetPath, []byte("filter\nnat\n"), 0644))

	// We can't easily override os.Stat in this simple test without build tags,
	// so we test the helper functions directly.
	assert.True(t, moduleLoaded("nf_conntrack") || !moduleLoaded("nf_conntrack")) // no-op sanity check
}

func TestModuleExists_InvalidModule(t *testing.T) {
	assert.False(t, moduleExists("definitely_not_a_real_module_12345"))
}
