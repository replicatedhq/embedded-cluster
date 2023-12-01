// Package goods handles embedded static assets. Things like writing them
// down to disk, return them as a parsed list, etc.
package goods

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

//go:embed bins/*
var binfs embed.FS

// K0sBinarySHA256 returns the SHA256 checksum of the embedded k0s binary.
func K0sBinarySHA256() (string, error) {
	fname := fmt.Sprintf("k0s-%s", defaults.K0sVersion)
	binpath := path.Join("bins", "k0sctl", fname)
	fp, err := binfs.Open(binpath)
	if err != nil {
		return "", fmt.Errorf("unable to open embedded k0s binary: %w", err)
	}
	defer fp.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, fp); err != nil {
		return "", fmt.Errorf("unable to copy embedded k0s binary: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// Materialize writes to disk embed assets.
func Materialize() error {
	entries, err := binfs.ReadDir("bins/k0sctl")
	if err != nil {
		return fmt.Errorf("unable to read bins dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		srcpath := fmt.Sprintf("bins/k0sctl/%s", entry.Name())
		srcfile, err := binfs.ReadFile(srcpath)
		if err != nil {
			return fmt.Errorf("unable to read asset: %w", err)
		}
		dstpath := fmt.Sprintf("%s/%s", defaults.K0sctlBinsSubDir(), entry.Name())
		if err := os.WriteFile(dstpath, srcfile, 0755); err != nil {
			return fmt.Errorf("unable to write file: %w", err)
		}
	}
	entries, err = binfs.ReadDir("bins/embedded-cluster")
	if err != nil {
		return fmt.Errorf("unable to read embedded-cluster bins dir: %w", err)
	}
	suffix := fmt.Sprintf("-%s-%s", runtime.GOOS, runtime.GOARCH)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), suffix) {
			// we always materialize 'kubectl-preflight' binary because
			// we run it remotely in the configured cluster nodes.
			if entry.Name() != "kubectl-preflight" {
				continue
			}
		}
		srcpath := fmt.Sprintf("bins/embedded-cluster/%s", entry.Name())
		srcfile, err := binfs.ReadFile(srcpath)
		if err != nil {
			return fmt.Errorf("unable to read asset: %w", err)
		}
		fname := strings.TrimSuffix(entry.Name(), suffix)
		dstpath := fmt.Sprintf("%s/%s", defaults.EmbeddedClusterBinsSubDir(), fname)
		if err := os.WriteFile(dstpath, srcfile, 0755); err != nil {
			return fmt.Errorf("unable to write file: %w", err)
		}
	}
	return nil
}
