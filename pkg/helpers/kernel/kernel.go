// Package kernel provides host kernel capability detection for embedded-cluster.
package kernel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// IPTablesBackend represents the detected netfilter backend on the host.
type IPTablesBackend string

const (
	BackendLegacy  IPTablesBackend = "legacy"
	BackendNFT     IPTablesBackend = "nft"
	BackendUnknown IPTablesBackend = "unknown"
)

// moduleExists uses modprobe --dry-run to check if a kernel module is available.
func moduleExists(module string) bool {
	cmd := exec.Command("modprobe", "-n", module)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// moduleLoaded checks /proc/modules for an already-loaded module.
func moduleLoaded(module string) bool {
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), module+" ")
}

// DetectIPTablesBackend determines if legacy iptables is available.
// Returns BackendLegacy if ip_tables module is available (loaded, loadable, or builtin).
// Returns BackendNFT if ip_tables is absent but nf_tables and nft_compat are available.
// Returns BackendUnknown if neither backend is detectable.
func DetectIPTablesBackend(ctx context.Context) (IPTablesBackend, error) {
	// 1. Check if ip_tables is loaded or builtin via /proc/net/ip_tables_names
	if _, err := os.Stat("/proc/net/ip_tables_names"); err == nil {
		return BackendLegacy, nil
	}

	// 2. Check if ip_tables module is loadable or already loaded
	if moduleExists("ip_tables") || moduleLoaded("ip_tables") {
		return BackendLegacy, nil
	}

	// 3. ip_tables is absent; check for nf_tables + nft_compat
	if moduleExists("nf_tables") || moduleLoaded("nf_tables") {
		if moduleExists("nft_compat") || moduleLoaded("nft_compat") {
			return BackendNFT, nil
		}
	}

	return BackendUnknown, fmt.Errorf("neither ip_tables (legacy iptables) nor nf_tables+nft_compat (nftables) is available")
}
