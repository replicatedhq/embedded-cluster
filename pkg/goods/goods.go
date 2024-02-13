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

	"github.com/replicatedhq/embedded-cluster/pkg/defaults"
)

//go:embed bins/*
var binfs embed.FS

// K0sBinarySHA256 returns the SHA256 checksum of the embedded k0s binary.
func K0sBinarySHA256() (string, error) {
	fp, err := binfs.Open("bins/k0s")
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

// Materialize writes to disk all embedded assets.
func Materialize() error {
	entries, err := binfs.ReadDir("bins")
	if err != nil {
		return fmt.Errorf("unable to read embedded-cluster bins dir: %w", err)
	}
	for _, entry := range entries {
		srcpath := fmt.Sprintf("bins/%s", entry.Name())
		srcfile, err := binfs.ReadFile(srcpath)
		if err != nil {
			return fmt.Errorf("unable to read asset: %w", err)
		}
		dstpath := defaults.PathToEmbeddedClusterBinary(entry.Name())
		if err := os.WriteFile(dstpath, srcfile, 0755); err != nil {
			return fmt.Errorf("unable to write file: %w", err)
		}
	}
	return nil
}
