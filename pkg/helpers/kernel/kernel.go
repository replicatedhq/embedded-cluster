// Package kernel provides host kernel capability detection for embedded-cluster.
package kernel

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

// IPTablesBackend represents the detected netfilter backend on the host.
type IPTablesBackend string

const (
	BackendLegacy  IPTablesBackend = "legacy"
	BackendNFT     IPTablesBackend = "nft"
	BackendUnknown IPTablesBackend = "unknown"
)

var (
	_stat       = os.Stat
	_readFile   = os.ReadFile
	_execCmdCtx = exec.CommandContext
)

// DetectIPTablesBackend determines if legacy iptables is available.
// Returns BackendLegacy if ip_tables module is available (loaded, loadable, or builtin).
// Returns BackendNFT if ip_tables is absent but nf_tables and nft_compat are available.
// Returns BackendUnknown if neither backend is detectable.
func DetectIPTablesBackend(ctx context.Context) (IPTablesBackend, error) {
	// 1. Check if ip_tables is loaded or builtin via /proc/net/ip_tables_names
	if _, err := _stat("/proc/net/ip_tables_names"); err == nil {
		return BackendLegacy, nil
	}

	// 2. Check if ip_tables module is loadable or already loaded
	if moduleExists(ctx, "ip_tables") || moduleLoaded("ip_tables") {
		return BackendLegacy, nil
	}
	logrus.Debugf("Kernel module ip_tables not detected (may not be available)")

	// 3. ip_tables is absent; check for nf_tables + nft_compat
	if moduleExists(ctx, "nf_tables") || moduleLoaded("nf_tables") {
		if moduleExists(ctx, "nft_compat") || moduleLoaded("nft_compat") {
			return BackendNFT, nil
		}
		logrus.Debugf("Kernel module nft_compat not detected (may not be available)")
	} else {
		logrus.Debugf("Kernel module nf_tables not detected (may not be available)")
	}

	return BackendUnknown, fmt.Errorf("neither ip_tables (legacy iptables) nor nf_tables+nft_compat (nftables) is available")
}

// moduleExists uses modprobe --dry-run to check if a kernel module is available.
func moduleExists(ctx context.Context, module string) bool {
	cmd := _execCmdCtx(ctx, "modprobe", "-n", module)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

// moduleLoaded checks /proc/modules for an already-loaded module.
func moduleLoaded(module string) bool {
	data, err := _readFile("/proc/modules")
	if err != nil {
		return false
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), module+" ") {
			return true
		}
	}
	if err := scanner.Err(); err != nil {
		logrus.WithError(err).Debugf("Failed to scan /proc/modules for %s", module)
		return false
	}
	return false
}
