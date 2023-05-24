/*
Package static provides the static filesystem.
*/
package static

import (
	"embed"
)

//go:embed *
var static embed.FS

// FS returns the static filesystem.
func FS() embed.FS {
	return static
}
