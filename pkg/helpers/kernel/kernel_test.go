package kernel

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectIPTablesBackend(t *testing.T) {
	tests := []struct {
		name        string
		stat        func(string) (os.FileInfo, error)
		execCmdCtx  func(context.Context, string, ...string) *exec.Cmd
		readFile    func(string) ([]byte, error)
		expected    IPTablesBackend
		expectError bool
	}{
		{
			name: "ip_tables_names exists",
			stat: func(path string) (os.FileInfo, error) {
				if path == "/proc/net/ip_tables_names" {
					return nil, nil
				}
				return nil, os.ErrNotExist
			},
			execCmdCtx: func(context.Context, string, ...string) *exec.Cmd {
				return exec.Command("false")
			},
			readFile:    func(name string) ([]byte, error) { return nil, os.ErrNotExist },
			expected:    BackendLegacy,
			expectError: false,
		},
		{
			name: "ip_tables modprobe succeeds",
			stat: func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			execCmdCtx: func(ctx context.Context, name string, args ...string) *exec.Cmd {
				if name == "modprobe" && len(args) == 2 && args[1] == "ip_tables" {
					return exec.Command("true")
				}
				return exec.Command("false")
			},
			readFile:    func(name string) ([]byte, error) { return nil, os.ErrNotExist },
			expected:    BackendLegacy,
			expectError: false,
		},
		{
			name: "nf_tables and nft_compat available",
			stat: func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			execCmdCtx: func(ctx context.Context, name string, args ...string) *exec.Cmd {
				if name == "modprobe" && len(args) == 2 {
					switch args[1] {
					case "ip_tables":
						return exec.Command("false")
					case "nf_tables":
						return exec.Command("true")
					case "nft_compat":
						return exec.Command("true")
					}
				}
				return exec.Command("false")
			},
			readFile:    func(name string) ([]byte, error) { return nil, os.ErrNotExist },
			expected:    BackendNFT,
			expectError: false,
		},
		{
			name: "nothing available",
			stat: func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			execCmdCtx: func(ctx context.Context, name string, args ...string) *exec.Cmd {
				return exec.Command("false")
			},
			readFile:    func(name string) ([]byte, error) { return nil, os.ErrNotExist },
			expected:    BackendUnknown,
			expectError: true,
		},
		{
			name: "nf_tables without nft_compat",
			stat: func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			execCmdCtx: func(ctx context.Context, name string, args ...string) *exec.Cmd {
				if name == "modprobe" && len(args) == 2 {
					switch args[1] {
					case "ip_tables":
						return exec.Command("false")
					case "nf_tables":
						return exec.Command("true")
					case "nft_compat":
						return exec.Command("false")
					}
				}
				return exec.Command("false")
			},
			readFile:    func(name string) ([]byte, error) { return nil, os.ErrNotExist },
			expected:    BackendUnknown,
			expectError: true,
		},
		{
			name: "moduleLoaded fallback ip_tables",
			stat: func(path string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			},
			execCmdCtx: func(ctx context.Context, name string, args ...string) *exec.Cmd {
				return exec.Command("false")
			},
			readFile: func(name string) ([]byte, error) {
				return []byte("ip_tables 28672 4 iptable_filter,iptable_nat,iptable_mangle,iptable_raw, Live 0x0000000000000000\n"), nil
			},
			expected:    BackendLegacy,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origStat := _stat
			origExec := _execCmdCtx
			origReadFile := _readFile
			_stat = tt.stat
			_execCmdCtx = tt.execCmdCtx
			_readFile = tt.readFile
			t.Cleanup(func() {
				_stat = origStat
				_execCmdCtx = origExec
				_readFile = origReadFile
			})

			backend, err := DetectIPTablesBackend(context.Background())
			assert.Equal(t, tt.expected, backend)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestModuleLoaded(t *testing.T) {
	tests := []struct {
		name     string
		readFile func(string) ([]byte, error)
		module   string
		expected bool
	}{
		{
			name: "module present in data",
			readFile: func(name string) ([]byte, error) {
				return []byte("nf_tables 245760 2 nft_compat,xt_set, Live 0x0000000000000000\n" +
					"nft_compat 16384 0 - Live 0x0000000000000000\n" +
					"ip_tables 28672 4 iptable_filter,iptable_nat,iptable_mangle,iptable_raw, Live 0x0000000000000000\n"), nil
			},
			module:   "ip_tables",
			expected: true,
		},
		{
			name: "module absent in data",
			readFile: func(name string) ([]byte, error) {
				return []byte("nf_tables 245760 2 nft_compat,xt_set, Live 0x0000000000000000\n"), nil
			},
			module:   "ip_tables",
			expected: false,
		},
		{
			name: "read file returns error",
			readFile: func(name string) ([]byte, error) {
				return nil, os.ErrNotExist
			},
			module:   "ip_tables",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origReadFile := _readFile
			_readFile = tt.readFile
			t.Cleanup(func() {
				_readFile = origReadFile
			})

			assert.Equal(t, tt.expected, moduleLoaded(tt.module))
		})
	}
}
