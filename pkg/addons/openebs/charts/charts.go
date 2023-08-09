// Package charts embeds all static tgz files in this directory.
package charts

import "embed"

//go:embed *.tgz
var FS embed.FS
