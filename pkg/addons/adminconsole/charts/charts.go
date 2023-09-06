// Package charts embeds all static tgz files in this directory.
package charts

import "embed"

// FS is the embedded filesystem.
//
//go:embed *.tgz
var FS embed.FS
