// Package goods handles embedded static assets. Things like writing them
// down to disk, return them as a parsed list, etc.
package goods

import (
	"embed"
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
