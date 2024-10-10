// Package goods handles embedded static assets. Things like writing them
// down to disk, return them as a parsed list, etc.
package goods

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

var (
	//go:embed bins/*
	binfs embed.FS
	//go:embed support/*
	supportfs embed.FS
	//go:embed systemd/*
	systemdfs embed.FS
	//go:embed internal/bins/*
	internalBinfs embed.FS
)

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

func GetSupportBundleSpec(name string) ([]byte, error) {
	entries, err := supportfs.ReadDir("support")
	if err != nil {
		return nil, fmt.Errorf("unable to read embedded-cluster support dir: %w", err)
	}
	for _, entry := range entries {
		srcpath := fmt.Sprintf("support/%s", entry.Name())
		if !strings.Contains(entry.Name(), name) {
			continue
		}
		srcfile, err := supportfs.ReadFile(srcpath)
		if err != nil {
			return nil, fmt.Errorf("unable to read asset: %w", err)
		}
		return srcfile, nil
	}
	return nil, fmt.Errorf("unable to find support file %s", name)
}
